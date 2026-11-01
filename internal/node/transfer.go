package node

import (
	"github.com/temurcanbulut/pasla/internal/i18n"

	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxFilesPerTransfer       = 200_000
	maxTransferBytes    int64 = 16 << 40 // 16 TiB per file and batch
	offerWait                 = 5 * time.Minute
	keepTransfers             = 300
	keepMessages              = 500
	maxMessageBytes           = 1 << 20
)

type FileMeta struct {
	Name string `json:"name"` // relative path with "/" separators
	Size int64  `json:"size"`
}

// Transfer is one batch of files in either direction.
type Transfer struct {
	ID       string
	Dir      string // "in" | "out"
	PeerFP   string
	PeerName string
	Files    []FileMeta
	Total    int64
	Created  time.Time

	done      atomic.Int64
	ctx       context.Context
	cancel    context.CancelFunc
	decision  chan bool
	dest      []string // incoming: final paths
	src       []string // outgoing via paths: local files
	reveal    string
	mu        sync.Mutex
	state     string // pending | active | done | rejected | cancelled | failed
	err       error
	finished  time.Time
	fileState []uint8 // 0 waiting, 1 running, 2 done
	filesDone int
	lastBytes int64
	lastAt    time.Time
	speed     float64
}

type TransferView struct {
	ID        string    `json:"id"`
	Dir       string    `json:"dir"`
	PeerFP    string    `json:"peerFp"`
	PeerName  string    `json:"peerName"`
	Count     int       `json:"count"`
	Names     []string  `json:"names"`
	Total     int64     `json:"total"`
	Done      int64     `json:"done"`
	FilesDone int       `json:"filesDone"`
	Risky     int       `json:"risky"`
	State     string    `json:"state"`
	Error     string    `json:"error,omitempty"`
	Speed     float64   `json:"speed"`
	Created   time.Time `json:"created"`
	Finished  time.Time `json:"finished,omitempty"`
	CanReveal bool      `json:"canReveal"`
}

func (t *Transfer) view(lang string) TransferView {
	t.mu.Lock()
	defer t.mu.Unlock()
	names := []string{}
	seen := map[string]bool{}
	risky := 0
	for _, f := range t.Files {
		if isRisky(f.Name) {
			risky++
		}
	}
	for _, f := range t.Files {
		top := strings.SplitN(f.Name, "/", 2)[0]
		if !seen[top] {
			seen[top] = true
			names = append(names, top)
			if len(names) == 3 {
				break
			}
		}
	}
	return TransferView{
		ID: t.ID, Dir: t.Dir, PeerFP: t.PeerFP, PeerName: t.PeerName, Count: len(t.Files), Names: names,
		Total: t.Total, Done: t.done.Load(), FilesDone: t.filesDone, Risky: risky, State: t.state, Error: i18n.Text(lang, t.err),
		Speed: t.speed, Created: t.Created, Finished: t.finished,
		CanReveal: t.Dir == "in" && t.reveal != "" && (t.state == "done" || t.filesDone > 0),
	}
}

func (t *Transfer) State() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

func terminal(state string) bool {
	return state == "done" || state == "rejected" || state == "cancelled" || state == "failed"
}

func checkedTransferTotal(files []FileMeta) (int64, error) {
	if len(files) == 0 || len(files) > maxFilesPerTransfer {
		return 0, i18n.E("invalid number of files")
	}
	var total int64
	for _, f := range files {
		if f.Size < 0 || f.Size > maxTransferBytes || f.Size > maxTransferBytes-total {
			return 0, i18n.E("transfer exceeds the 16 TiB size limit")
		}
		total += f.Size
	}
	return total, nil
}

func (n *Node) newTransfer(id, dir, peerFP string, files []FileMeta) (*Transfer, error) {
	total, err := checkedTransferTotal(files)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &Transfer{
		ID: id, Dir: dir, PeerFP: peerFP, PeerName: n.peerName(peerFP), Files: files,
		Created: time.Now(), ctx: ctx, cancel: cancel, decision: make(chan bool, 1),
		state: "pending", fileState: make([]uint8, len(files)), lastAt: time.Now(),
	}
	t.Total = total
	n.mu.Lock()
	n.transfers[id] = t
	n.order = append(n.order, id)
	n.trimTransfersLocked()
	n.mu.Unlock()
	n.bus.Publish("transfer", t.view(n.lang()))
	return t, nil
}

func (n *Node) trimTransfersLocked() {
	for len(n.order) > keepTransfers {
		dropped := false
		for i, id := range n.order {
			if terminal(n.transfers[id].State()) {
				delete(n.transfers, id)
				n.order = append(n.order[:i], n.order[i+1:]...)
				dropped = true
				break
			}
		}
		if !dropped {
			return
		}
	}
}

func (n *Node) transfer(id string) *Transfer {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.transfers[id]
}

func (n *Node) transferViews(lang string) []TransferView {
	n.mu.Lock()
	list := make([]*Transfer, 0, len(n.order))
	for _, id := range n.order {
		list = append(list, n.transfers[id])
	}
	n.mu.Unlock()
	out := make([]TransferView, 0, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		out = append(out, list[i].view(lang))
	}
	return out
}

// setState moves a transfer forward; terminal states are final.
func (n *Node) setState(t *Transfer, state string, errMsg error) bool {
	t.mu.Lock()
	if terminal(t.state) {
		t.mu.Unlock()
		return false
	}
	t.state = state
	if errMsg != nil {
		t.err = errMsg
	}
	if terminal(state) {
		t.finished = time.Now()
		t.speed = 0
	}
	t.mu.Unlock()
	if terminal(state) {
		t.cancel()
	}
	n.bus.Publish("transfer", t.view(n.lang()))
	if state == "done" {
		n.bus.Publish("attention", map[string]any{"kind": "done", "dir": t.Dir, "name": t.PeerName, "count": len(t.Files)})
	}
	return true
}

func (n *Node) publishProgress(now time.Time) {
	n.mu.Lock()
	active := []*Transfer{}
	for _, t := range n.transfers {
		if t.State() == "active" {
			active = append(active, t)
		}
	}
	n.mu.Unlock()
	if len(active) == 0 {
		return
	}
	views := make([]TransferView, 0, len(active))
	for _, t := range active {
		t.mu.Lock()
		cur := t.done.Load()
		dt := now.Sub(t.lastAt).Seconds()
		if dt > 0 {
			inst := float64(cur-t.lastBytes) / dt
			if t.speed == 0 {
				t.speed = inst
			} else {
				t.speed = 0.6*t.speed + 0.4*inst
			}
		}
		t.lastBytes, t.lastAt = cur, now
		t.mu.Unlock()
		views = append(views, t.view(n.lang()))
	}
	n.bus.Publish("progress", views)
}

type progressWriter struct{ t *Transfer }

func (p progressWriter) Write(b []byte) (int, error) {
	if err := p.t.ctx.Err(); err != nil {
		return 0, err
	}
	p.t.done.Add(int64(len(b)))
	return len(b), nil
}

// countingWriter is a progressWriter that also remembers how much of the
// current file it has counted, so a failed file can be subtracted again.
type countingWriter struct {
	t *Transfer
	n *atomic.Int64
}

func (c countingWriter) Write(b []byte) (int, error) {
	if _, err := (progressWriter{c.t}).Write(b); err != nil {
		return 0, err
	}
	c.n.Add(int64(len(b)))
	return len(b), nil
}

func (n *Node) markFileDone(t *Transfer, i int) (all bool) {
	t.mu.Lock()
	if t.fileState[i] != 2 {
		t.fileState[i] = 2
		t.filesDone++
	}
	all = t.filesDone == len(t.Files)
	t.mu.Unlock()
	return all
}

var idRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

// ---- receiving (peer API) ---------------------------------------------------

type offerReq struct {
	ID    string     `json:"id"`
	Files []FileMeta `json:"files"`
}
type offerResp struct {
	Accepted   bool     `json:"accepted"`
	Reason     string   `json:"reason,omitempty"` // English, for people reading the wire
	ReasonKey  string   `json:"reasonKey,omitempty"`
	ReasonArgs []string `json:"reasonArgs,omitempty"`
}

func rejectWith(m *i18n.Msg) offerResp {
	return offerResp{Reason: m.Error(), ReasonKey: m.Format, ReasonArgs: m.StringArgs()}
}

// reason rebuilds the receiver's explanation so it shows in our language.
func (o offerResp) reason() error {
	switch {
	case o.ReasonKey != "":
		return i18n.FromWire(o.ReasonKey, o.ReasonArgs)
	case o.Reason != "":
		return errors.New(o.Reason)
	}
	return nil
}

func (n *Node) hOffer(w http.ResponseWriter, r *http.Request) {
	fp := clientFP(r)
	tp, _ := n.store.Trusted(fp)
	var req offerReq
	if !readBody(w, r, &req, 64<<20) {
		return
	}
	n.mu.Lock()
	pending := 0
	for _, t := range n.transfers {
		if t.Dir == "in" && t.PeerFP == fp && t.State() == "pending" {
			pending++
		}
	}
	n.mu.Unlock()
	if pending >= 3 {
		jsonErr(w, r, http.StatusTooManyRequests, i18n.E("there are already offers from this device waiting for an answer"))
		return
	}
	if !idRe.MatchString(req.ID) || n.transfer(req.ID) != nil {
		jsonErr(w, r, http.StatusBadRequest, i18n.E("invalid transfer id"))
		return
	}
	if len(req.Files) == 0 || len(req.Files) > maxFilesPerTransfer {
		jsonErr(w, r, http.StatusBadRequest, i18n.E("invalid number of files"))
		return
	}
	rel := make([][]string, len(req.Files))
	for i, f := range req.Files {
		parts, err := sanitizeRelPath(f.Name)
		if err != nil || f.Size < 0 {
			jsonErr(w, r, http.StatusBadRequest, i18n.E("invalid file: %s", f.Name))
			return
		}
		rel[i] = parts
		req.Files[i].Name = strings.Join(parts, "/")
	}
	t, err := n.newTransfer(req.ID, "in", fp, req.Files)
	if err != nil {
		jsonErr(w, r, http.StatusBadRequest, err)
		return
	}

	if free, ok := n.freeSpace(); ok && uint64(t.Total)+diskReserve > free {
		msg := i18n.E("not enough free space on the receiving device (%s needed, %s free)", humanBytes(t.Total), humanBytes(int64(free)))
		n.setState(t, "rejected", msg)
		writeJSON200(w, rejectWith(msg))
		return
	}

	accept := n.store.Settings().AutoAccept || tp.AutoAccept
	if !accept {
		n.bus.Publish("attention", map[string]any{"kind": "offer", "name": t.PeerName, "count": len(t.Files)})
		timer := time.NewTimer(offerWait)
		defer timer.Stop()
		select {
		case accept = <-t.decision:
		case <-r.Context().Done():
			n.setState(t, "cancelled", i18n.E("the sender gave up"))
			return
		case <-t.ctx.Done():
			accept = false
		case <-timer.C:
			n.setState(t, "rejected", i18n.E("no answer"))
			writeJSON200(w, rejectWith(i18n.E("nobody answered on the other device")))
			return
		}
	}
	if !accept {
		n.setState(t, "rejected", nil)
		writeJSON200(w, rejectWith(i18n.E("the other side declined")))
		return
	}
	root := n.store.Settings().ReceiveDir
	if err := os.MkdirAll(root, 0o755); err != nil {
		n.setState(t, "failed", i18n.E("could not create the receive folder: %v", err))
		jsonErr(w, r, http.StatusInsufficientStorage, i18n.E("could not create the receive folder"))
		return
	}
	n.reserveDestinations(t, root, rel)
	n.setState(t, "active", nil)
	writeJSON200(w, offerResp{Accepted: true})
}

// revealTarget picks what "show in folder" should open.
func revealTarget(root string, dest []string) string {
	if len(dest) == 0 {
		return root
	}
	first, _ := filepath.Rel(root, dest[0])
	top := strings.SplitN(filepath.ToSlash(first), "/", 2)[0]
	for _, d := range dest[1:] {
		rel, _ := filepath.Rel(root, d)
		if strings.SplitN(filepath.ToSlash(rel), "/", 2)[0] != top {
			return root
		}
	}
	return filepath.Join(root, top)
}

// DecideOffer is this user's answer to an incoming offer.
func (n *Node) DecideOffer(id string, ok bool) error {
	t := n.transfer(id)
	if t == nil || t.Dir != "in" || t.State() != "pending" {
		return i18n.E("no waiting transfer found")
	}
	select {
	case t.decision <- ok:
	default:
	}
	return nil
}

// diskReserve is left free on the receiving disk so a transfer never fills it.
const diskReserve = 256 << 20

// freeSpace reports the free bytes where incoming files land (or on the
// nearest existing parent, if the folder has not been created yet).
func (n *Node) freeSpace() (uint64, bool) {
	dir := n.store.Settings().ReceiveDir
	for {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return 0, false
		}
		dir = parent
	}
	free, err := freeBytes(dir)
	return free, err == nil
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for v := b / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (n *Node) hFile(w http.ResponseWriter, r *http.Request) {
	fp := clientFP(r)
	t := n.transfer(r.PathValue("id"))
	if t == nil || t.Dir != "in" || t.PeerFP != fp {
		jsonErr(w, r, http.StatusNotFound, i18n.E("transfer not found"))
		return
	}
	// Between devices every file carries its SHA-256 in an HTTP trailer.
	n.receiveInto(w, r, t, func() string { return r.Trailer.Get("X-Pasla-Sha256") })
}

// receiveInto stores file {index} of an incoming transfer from the request
// body. wantHash, when set, returns the checksum the sender announced; the
// phone page has no way to send trailers and relies on TLS integrity alone.
func (n *Node) receiveInto(w http.ResponseWriter, r *http.Request, t *Transfer, wantHash func() string) {
	if t.State() != "active" {
		jsonErr(w, r, http.StatusGone, i18n.E("transfer was cancelled"))
		return
	}
	i, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || i < 0 || i >= len(t.Files) {
		jsonErr(w, r, http.StatusBadRequest, i18n.E("invalid file number"))
		return
	}
	t.mu.Lock()
	if t.fileState[i] != 0 {
		t.mu.Unlock()
		jsonErr(w, r, http.StatusConflict, i18n.E("this file was already received"))
		return
	}
	t.fileState[i] = 1
	dest := t.dest[i]
	t.mu.Unlock()

	if err := n.receiveFile(t, i, dest, r.Body, wantHash); err != nil {
		t.mu.Lock()
		if t.fileState[i] == 1 {
			t.fileState[i] = 0
		}
		t.mu.Unlock()
		if t.ctx.Err() != nil {
			jsonErr(w, r, http.StatusGone, i18n.E("transfer was cancelled"))
			return
		}
		n.setState(t, "failed", err)
		jsonErr(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	if n.markFileDone(t, i) {
		n.setState(t, "done", nil)
	}
	writeJSON200(w, map[string]bool{"ok": true})
}

func (n *Node) receiveFile(t *Transfer, i int, dest string, body io.Reader, wantHash func() string) error {
	size := t.Files[i].Size
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return i18n.E("could not create the folder: %v", err)
	}
	// CreateTemp uses exclusive creation: an incoming name must never truncate
	// an existing user file or another transfer's partial download.
	f, err := os.CreateTemp(filepath.Dir(dest), ".pasla-*.part")
	if err != nil {
		return i18n.E("could not write the file: %v", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	h := sha256.New()
	buf := make([]byte, 1<<20)
	var counted atomic.Int64
	written, err := io.CopyBuffer(io.MultiWriter(f, h, countingWriter{t, &counted}), io.LimitReader(body, size+1), buf)
	cerr := f.Close()
	fail := func(e error) error {
		t.done.Add(-counted.Load())
		return e
	}
	if err != nil {
		// The sender's cancel notice usually arrives a moment after the
		// stream breaks; give it a chance so the UI says "cancelled".
		select {
		case <-t.ctx.Done():
		case <-time.After(800 * time.Millisecond):
		}
		return fail(i18n.E("the transfer was interrupted: %v", err))
	}
	if cerr != nil {
		return fail(i18n.E("disk error: %v", cerr))
	}
	if written != size {
		return fail(i18n.E("%s: expected %v bytes, received %v", t.Files[i].Name, size, written))
	}
	if wantHash != nil {
		if got, want := hex.EncodeToString(h.Sum(nil)), wantHash(); want == "" || got != want {
			return fail(i18n.E("%s: integrity check failed", t.Files[i].Name))
		}
	}
	if err := n.commitReceivedFile(tmp, dest); err != nil {
		return fail(i18n.E("could not save the file: %v", err))
	}
	markFromNetwork(dest)
	return nil
}

// Serialise the final existence check and rename across this node's receivers.
// A destination selected while another transfer was active must not overwrite
// that transfer's completed file. Local processes must not mutate ReceiveDir
// concurrently; this mutex does not lock other processes' filesystem access.
func (n *Node) commitReceivedFile(tmp, dest string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, err := os.Lstat(dest); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(tmp, dest)
}

func (n *Node) hRemoteCancel(w http.ResponseWriter, r *http.Request) {
	t := n.transfer(r.PathValue("id"))
	if t != nil && t.PeerFP == clientFP(r) {
		n.setState(t, "cancelled", i18n.E("the other device cancelled"))
	}
	writeJSON200(w, map[string]bool{"ok": true})
}

// ---- sending ----------------------------------------------------------------

// Offer announces a batch to a paired device and waits for its answer.
func (n *Node) Offer(fp string, files []FileMeta) (*Transfer, error) {
	if _, ok := n.store.Trusted(fp); !ok {
		return nil, i18n.E("pair with this device first")
	}
	if len(files) == 0 {
		return nil, i18n.E("nothing to send")
	}
	for i := range files {
		files[i].Name = strings.TrimLeft(strings.ReplaceAll(files[i].Name, "\\", "/"), "/")
		if files[i].Size < 0 {
			return nil, i18n.E("invalid file size")
		}
	}
	t, err := n.newTransfer(randomID(), "out", fp, files)
	if err != nil {
		return nil, err
	}
	go n.runOffer(t)
	return t, nil
}

func (n *Node) runOffer(t *Transfer) {
	ctx, cancel := context.WithTimeout(t.ctx, offerWait+30*time.Second)
	defer cancel()
	var res offerResp
	if err := n.callPeer(ctx, t.PeerFP, "", http.MethodPost, "/v1/offer", offerReq{ID: t.ID, Files: t.Files}, &res); err != nil {
		if t.ctx.Err() == nil {
			n.setState(t, "failed", err)
		}
		return
	}
	if !res.Accepted {
		n.setState(t, "rejected", res.reason())
		return
	}
	n.setState(t, "active", nil)
}

// WaitDecision blocks until an outgoing offer is answered.
func (n *Node) WaitDecision(ctx context.Context, t *Transfer) string {
	ch, unsub := n.bus.Subscribe()
	defer unsub()
	for {
		if s := t.State(); s != "pending" {
			return s
		}
		select {
		case <-ctx.Done():
			return t.State()
		case <-ch:
		case <-time.After(time.Second):
		}
	}
}

// SendFile streams file i of an accepted outgoing transfer.
func (n *Node) SendFile(t *Transfer, i int, body io.Reader) error {
	if i < 0 || i >= len(t.Files) {
		return i18n.E("invalid file number")
	}
	if s := t.State(); s != "active" {
		return i18n.E("transfer is not active (%s)", s)
	}
	t.mu.Lock()
	if t.fileState[i] != 0 {
		t.mu.Unlock()
		return i18n.E("this file is already being sent")
	}
	t.fileState[i] = 1
	t.mu.Unlock()

	err := n.sendFile(t, i, body)
	if err != nil {
		t.mu.Lock()
		t.fileState[i] = 0
		t.mu.Unlock()
		var ae *apiError
		switch {
		case t.ctx.Err() != nil:
			// cancelled locally or by the peer: state already set
		case errors.As(err, &ae) && ae.Status == http.StatusGone:
			n.setState(t, "cancelled", i18n.E("the other device cancelled"))
		default:
			n.setState(t, "failed", err)
		}
		return err
	}
	if n.markFileDone(t, i) {
		n.setState(t, "done", nil)
	}
	return nil
}

func (n *Node) sendFile(t *Transfer, i int, body io.Reader) error {
	addr, ok := n.addrOf(t.PeerFP)
	if !ok {
		return i18n.E("the device is not visible on the network right now")
	}
	ur := &uploadReader{r: io.LimitReader(body, t.Files[i].Size), h: sha256.New(), t: t, size: t.Files[i].Size, name: t.Files[i].Name}
	req, err := http.NewRequestWithContext(t.ctx, http.MethodPut,
		fmt.Sprintf("https://%s/v1/transfer/%s/%d", addr, t.ID, i), ur)
	if err != nil {
		return err
	}
	ur.req = req
	req.Header.Set("Content-Type", "application/octet-stream")
	// The checksum travels as an HTTP trailer, filled in by uploadReader at
	// end of file, so the file is read only once and never buffered.
	req.Trailer = http.Header{"X-Pasla-Sha256": nil}

	resp, err := n.client(t.PeerFP).Do(req)
	if err != nil {
		t.done.Add(-ur.sent.Load())
		if ur.err != nil {
			return ur.err
		}
		return friendlyNetErr(err)
	}
	defer resp.Body.Close()
	if err := decodeResp(resp, nil); err != nil {
		t.done.Add(-ur.sent.Load())
		return err
	}
	return nil
}

// uploadReader hashes and counts a file while the HTTP transport reads it.
type uploadReader struct {
	r    io.Reader
	h    hash.Hash
	t    *Transfer
	req  *http.Request
	size int64
	name string
	n    int64
	sent atomic.Int64
	err  error
}

func (u *uploadReader) Read(p []byte) (int, error) {
	if err := u.t.ctx.Err(); err != nil {
		return 0, err
	}
	k, err := u.r.Read(p)
	if k > 0 {
		u.h.Write(p[:k])
		u.n += int64(k)
		u.sent.Add(int64(k))
		u.t.done.Add(int64(k))
	}
	if err == io.EOF {
		if u.n != u.size {
			u.err = i18n.E("%s changed or got shorter while it was being read", u.name)
			return k, u.err
		}
		u.req.Trailer.Set("X-Pasla-Sha256", hex.EncodeToString(u.h.Sum(nil)))
	}
	return k, err
}

// Cancel stops a transfer here and tells the other device.
func (n *Node) Cancel(id string) error {
	t := n.transfer(id)
	if t == nil {
		return i18n.E("transfer not found")
	}
	if t.Dir == "in" && t.State() == "pending" {
		return n.DecideOffer(id, false)
	}
	if !n.setState(t, "cancelled", i18n.E("you cancelled")) {
		return nil
	}
	if isPhoneFP(t.PeerFP) {
		return nil
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		_ = n.callPeer(ctx, t.PeerFP, "", http.MethodPost, "/v1/transfer/"+t.ID+"/cancel", struct{}{}, nil)
	}()
	return nil
}

func (n *Node) ClearFinished() {
	n.mu.Lock()
	keep := n.order[:0]
	for _, id := range n.order {
		if terminal(n.transfers[id].State()) {
			delete(n.transfers, id)
		} else {
			keep = append(keep, id)
		}
	}
	n.order = keep
	n.mu.Unlock()
	n.bus.Publish("transfers", nil)
}

// SendPaths sends files and folders from this computer's disk. It is what
// the command line uses; the web interface streams from the browser instead.
func (n *Node) SendPaths(fp string, paths []string) (*Transfer, error) {
	var files []FileMeta
	var src []string
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		base := filepath.Base(abs)
		if !st.IsDir() {
			files = append(files, FileMeta{Name: base, Size: st.Size()})
			src = append(src, abs)
			continue
		}
		err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.Type().IsRegular() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(abs, path)
			files = append(files, FileMeta{Name: base + "/" + filepath.ToSlash(rel), Size: info.Size()})
			src = append(src, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if len(files) == 0 {
		return nil, i18n.E("no files found to send")
	}
	t, err := n.Offer(fp, files)
	if err != nil {
		return nil, err
	}
	t.src = src
	go func() {
		if n.WaitDecision(t.ctx, t) != "active" {
			return
		}
		jobs := make(chan int)
		var wg sync.WaitGroup
		for w := 0; w < 3; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobs {
					f, err := os.Open(t.src[i])
					if err != nil {
						n.setState(t, "failed", err)
						continue
					}
					_ = n.SendFile(t, i, f)
					f.Close()
				}
			}()
		}
		order := make([]int, len(files))
		for i := range order {
			order[i] = i
		}
		// Small files first: the list fills quickly and big files keep the pipe busy at the end.
		sort.SliceStable(order, func(a, b int) bool { return files[order[a]].Size < files[order[b]].Size })
		for _, i := range order {
			if t.State() != "active" {
				break
			}
			jobs <- i
		}
		close(jobs)
		wg.Wait()
	}()
	return t, nil
}

// ---- text messages ----------------------------------------------------------

type Message struct {
	ID     string    `json:"id"`
	PeerFP string    `json:"peerFp"`
	Dir    string    `json:"dir"`
	Text   string    `json:"text"`
	At     time.Time `json:"at"`
}

func (n *Node) addMessage(m Message) {
	n.mu.Lock()
	n.messages = append(n.messages, m)
	if len(n.messages) > keepMessages {
		n.messages = n.messages[len(n.messages)-keepMessages:]
	}
	n.mu.Unlock()
	n.bus.Publish("message", m)
}

func (n *Node) messageList() []Message {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]Message{}, n.messages...)
}

type messageReq struct {
	Text string `json:"text"`
}

func (n *Node) hMessage(w http.ResponseWriter, r *http.Request) {
	var req messageReq
	if !readBody(w, r, &req, maxMessageBytes+4096) {
		return
	}
	if req.Text == "" || len(req.Text) > maxMessageBytes {
		jsonErr(w, r, http.StatusBadRequest, i18n.E("text is empty or too long"))
		return
	}
	fp := clientFP(r)
	n.addMessage(Message{ID: randomID(), PeerFP: fp, Dir: "in", Text: req.Text, At: time.Now()})
	n.bus.Publish("attention", map[string]any{"kind": "message", "name": n.peerName(fp)})
	writeJSON200(w, map[string]bool{"ok": true})
}

func (n *Node) SendMessage(fp, text string) error {
	if _, ok := n.store.Trusted(fp); !ok {
		return i18n.E("pair with this device first")
	}
	if strings.TrimSpace(text) == "" || len(text) > maxMessageBytes {
		return i18n.E("text is empty or longer than 1 MB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := n.callPeer(ctx, fp, "", http.MethodPost, "/v1/message", messageReq{Text: text}, nil); err != nil {
		return err
	}
	n.addMessage(Message{ID: randomID(), PeerFP: fp, Dir: "out", Text: text, At: time.Now()})
	return nil
}
