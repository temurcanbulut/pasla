package node

import (
	"github.com/temurcanbulut/pasla/internal/i18n"

	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/temurcanbulut/pasla/internal/qr"
	"github.com/temurcanbulut/pasla/internal/ui"
)

// Phone connection: a phone needs no app. The computer shows a QR code that
// holds an HTTPS address with a one-off key in its fragment (the part after
// "#", which browsers never send over the network or keep in server logs).
//
//  1. The phone opens the page and sends the key once; it gets back an
//     HttpOnly session cookie and a 4-digit check number.
//  2. The computer shows "<phone> wants to connect, check number 1234" and
//     the user allows it only if the phone shows the same number. Until then
//     the phone can do nothing.
//  3. The phone can then send files and text to the computer, and download
//     what the computer puts in its outbox.
//
// The page is served over TLS with the device certificate, so traffic is
// encrypted; the phone's browser will warn once because the certificate is
// self-signed. The port is open only while the user has the QR window open
// or a phone is connected, and the key expires after ten minutes.

const (
	DefaultPhonePort = 47823
	phoneKeyLife     = 10 * time.Minute
	phonePendingLife = 3 * time.Minute
	phoneIdleLife    = 30 * time.Minute
	phoneCookie      = "pasla_phone"
	phoneMaxSessions = 4
	phoneMaxBadKeys  = 8
)

type phoneHub struct {
	mu       sync.Mutex
	srv      *http.Server
	ln       net.Listener
	key      string
	keyExp   time.Time
	badKeys  int
	sessions map[string]*phoneSess
	outDir   string
}

type phoneSess struct {
	ID       string
	Name     string
	OS       string
	Check    string
	cookie   string
	Approved bool
	Created  time.Time
	LastSeen time.Time
	outbox   []*outItem
}

type outItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Downloads int       `json:"downloads"`
	Added     time.Time `json:"added"`
	path      string
}

type PhoneLink struct {
	URL string `json:"url"`
	QR  string `json:"qr"`
}

type PhoneView struct {
	Active  bool        `json:"active"`
	Links   []PhoneLink `json:"links"`
	Expires time.Time   `json:"expires,omitempty"`
	KeyLive bool        `json:"keyLive"`
}

type PhoneSessView struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	OS       string    `json:"os"`
	Check    string    `json:"check"`
	Approved bool      `json:"approved"`
	Outbox   []outItem `json:"outbox"`
}

func randomDigits(n int) string {
	b := randomBytes(8)
	v := uint64(0)
	for _, x := range b {
		v = v<<8 | uint64(x)
	}
	mod := uint64(1)
	for i := 0; i < n; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", n, v%mod)
}

func newPhoneHub() *phoneHub { return &phoneHub{sessions: map[string]*phoneSess{}} }

func phoneFP(id string) string { return "phone:" + id }

func isPhoneFP(fp string) bool { return strings.HasPrefix(fp, "phone:") }

// ---- desktop side -----------------------------------------------------------

// StartPhone opens the phone port (if needed) and issues a fresh key.
func (n *Node) StartPhone() (PhoneView, error) {
	h := n.phone
	h.mu.Lock()
	if h.srv == nil {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", n.cfg.PhonePort))
		if err != nil {
			h.mu.Unlock()
			return PhoneView{}, i18n.E("could not open phone port %v: %v", n.cfg.PhonePort, err)
		}
		dir, err := os.MkdirTemp("", "pasla-phone-*")
		if err != nil {
			ln.Close()
			h.mu.Unlock()
			return PhoneView{}, err
		}
		h.outDir = dir
		h.ln = ln
		h.srv = &http.Server{
			Handler:           n.phoneHandler(),
			ReadHeaderTimeout: 15 * time.Second,
			IdleTimeout:       2 * time.Minute,
			MaxHeaderBytes:    64 << 10,
			TLSConfig:         &tls.Config{Certificates: []tls.Certificate{n.cert}, MinVersion: tls.VersionTLS12},
			ErrorLog:          log.New(discardWriter{}, "", 0),
		}
		go func(srv *http.Server, ln net.Listener) { _ = srv.ServeTLS(ln, "", "") }(h.srv, ln)
	}
	h.key = base64.RawURLEncoding.EncodeToString(randomBytes(16))
	h.keyExp = time.Now().Add(phoneKeyLife)
	h.badKeys = 0
	h.mu.Unlock()
	n.bus.Publish("phones", nil)
	return n.phoneView(), nil
}

// StopPhone closes the port and forgets every phone and outbox file.
func (n *Node) StopPhone() {
	h := n.phone
	h.mu.Lock()
	srv, dir := h.srv, h.outDir
	h.srv, h.ln, h.outDir, h.key = nil, nil, "", ""
	h.sessions = map[string]*phoneSess{}
	h.mu.Unlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = srv.Shutdown(ctx)
		cancel()
	}
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
	n.bus.Publish("phones", nil)
}

func (n *Node) phoneView() PhoneView {
	h := n.phone
	h.mu.Lock()
	defer h.mu.Unlock()
	v := PhoneView{Active: h.srv != nil, Links: []PhoneLink{}}
	if h.srv == nil {
		return v
	}
	v.KeyLive = h.key != "" && time.Now().Before(h.keyExp)
	if !v.KeyLive {
		return v
	}
	v.Expires = h.keyExp
	for _, ip := range localIPv4s() {
		u := fmt.Sprintf("https://%s/#k=%s", net.JoinHostPort(ip, strconv.Itoa(n.cfg.PhonePort)), h.key)
		link := PhoneLink{URL: u}
		if c, err := qr.Encode([]byte(u)); err == nil {
			link.QR = c.SVG()
		}
		v.Links = append(v.Links, link)
	}
	return v
}

func (n *Node) phoneSessViews() []PhoneSessView {
	h := n.phone
	h.mu.Lock()
	defer h.mu.Unlock()
	out := []PhoneSessView{}
	for _, s := range h.sessions {
		v := PhoneSessView{ID: s.ID, Name: s.Name, OS: s.OS, Check: s.Check, Approved: s.Approved, Outbox: []outItem{}}
		for _, it := range s.outbox {
			v.Outbox = append(v.Outbox, *it)
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (n *Node) phoneSession(id string) *phoneSess {
	n.phone.mu.Lock()
	defer n.phone.mu.Unlock()
	return n.phone.sessions[id]
}

func (n *Node) DecidePhone(id string, ok bool) error {
	h := n.phone
	h.mu.Lock()
	s, found := h.sessions[id]
	if found {
		if ok {
			s.Approved = true
			s.LastSeen = time.Now()
		} else {
			delete(h.sessions, id)
		}
	}
	h.mu.Unlock()
	if !found {
		return i18n.E("phone not found or its connection has expired")
	}
	n.bus.Publish("phones", nil)
	return nil
}

func (n *Node) DisconnectPhone(id string) {
	h := n.phone
	h.mu.Lock()
	s := h.sessions[id]
	delete(h.sessions, id)
	h.mu.Unlock()
	if s != nil {
		for _, it := range s.outbox {
			_ = os.Remove(it.path)
		}
	}
	n.bus.Publish("phones", nil)
}

// AddToOutbox stores a file the phone can then download.
func (n *Node) AddToOutbox(id, name string, size int64, body io.Reader) (outItem, error) {
	parts, err := sanitizeRelPath(name)
	if err != nil {
		return outItem{}, err
	}
	h := n.phone
	h.mu.Lock()
	s, dir := h.sessions[id], h.outDir
	h.mu.Unlock()
	if s == nil || !s.Approved || dir == "" {
		return outItem{}, i18n.E("phone is not connected")
	}
	it := &outItem{ID: randomID(), Name: parts[len(parts)-1], Added: time.Now()}
	it.path = filepath.Join(dir, it.ID)
	f, err := os.OpenFile(it.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return outItem{}, err
	}
	written, err := io.Copy(f, io.LimitReader(body, size+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && written != size {
		err = i18n.E("file arrived incomplete")
	}
	if err != nil {
		_ = os.Remove(it.path)
		return outItem{}, err
	}
	it.Size = written
	h.mu.Lock()
	if cur := h.sessions[id]; cur == s {
		s.outbox = append(s.outbox, it)
	} else {
		_ = os.Remove(it.path)
		h.mu.Unlock()
		return outItem{}, i18n.E("phone connection closed")
	}
	h.mu.Unlock()
	n.bus.Publish("phones", nil)
	return *it, nil
}

func (n *Node) RemoveFromOutbox(id, item string) {
	h := n.phone
	h.mu.Lock()
	if s := h.sessions[id]; s != nil {
		keep := s.outbox[:0]
		for _, it := range s.outbox {
			if it.ID == item {
				_ = os.Remove(it.path)
				continue
			}
			keep = append(keep, it)
		}
		s.outbox = keep
	}
	h.mu.Unlock()
	n.bus.Publish("phones", nil)
}

func (n *Node) SendPhoneMessage(id, text string) error {
	s := n.phoneSession(id)
	if s == nil || !s.Approved {
		return i18n.E("phone is not connected")
	}
	if strings.TrimSpace(text) == "" || len(text) > maxMessageBytes {
		return i18n.E("text is empty or longer than 1 MB")
	}
	n.addMessage(Message{ID: randomID(), PeerFP: phoneFP(id), Dir: "out", Text: text, At: time.Now()})
	return nil
}

// prunePhones drops idle phones and closes the port when nothing needs it.
func (n *Node) prunePhones(now time.Time) {
	h := n.phone
	h.mu.Lock()
	changed := false
	var stale []*phoneSess
	for id, s := range h.sessions {
		if (!s.Approved && now.Sub(s.Created) > phonePendingLife) || now.Sub(s.LastSeen) > phoneIdleLife {
			delete(h.sessions, id)
			stale = append(stale, s)
			changed = true
		}
	}
	idle := h.srv != nil && len(h.sessions) == 0 && now.After(h.keyExp)
	h.mu.Unlock()
	for _, s := range stale {
		for _, it := range s.outbox {
			_ = os.Remove(it.path)
		}
	}
	if idle {
		n.StopPhone()
		return
	}
	if changed {
		n.bus.Publish("phones", nil)
	}
}

// ---- phone side (HTTPS) -----------------------------------------------------

func (n *Node) phoneHandler() http.Handler {
	mux := http.NewServeMux()
	static := http.FileServerFS(ui.Static)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		b, err := fs.ReadFile(ui.Static, "m/index.html")
		if err != nil {
			http.Error(w, "page not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(b)
	})
	for _, p := range []string{"GET /m/", "GET /i18n.js", "GET /icon.svg", "GET /favicon.svg", "GET /mark.svg", "GET /mark-dark.svg", "GET /icon.png", "GET /icon-512.png"} {
		mux.Handle(p, static)
	}
	mux.HandleFunc("POST /p/login", n.pLogin)
	mux.HandleFunc("GET /p/api/state", n.phoneAuth(false, n.pState))
	mux.HandleFunc("POST /p/api/offer", n.phoneAuth(true, n.pOffer))
	mux.HandleFunc("PUT /p/api/upload/{id}/{index}", n.phoneAuth(true, n.pUpload))
	mux.HandleFunc("GET /p/api/outbox/{item}", n.phoneAuth(true, n.pDownload))
	mux.HandleFunc("POST /p/api/message", n.phoneAuth(true, n.pMessage))
	mux.HandleFunc("POST /p/api/logout", n.phoneAuth(false, func(w http.ResponseWriter, r *http.Request, s *phoneSess) {
		n.DisconnectPhone(s.ID)
		http.SetCookie(w, &http.Cookie{Name: phoneCookie, Value: "", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true})
		writeJSON200(w, map[string]bool{"ok": true})
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && o != "https://"+r.Host {
				http.Error(w, "foreign origin", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		mux.ServeHTTP(w, r)
	})
}

func (n *Node) pLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		OS   string `json:"os"`
	}
	if !readBody(w, r, &req, 4096) {
		return
	}
	h := n.phone
	h.mu.Lock()
	live := h.key != "" && time.Now().Before(h.keyExp)
	if !live || subtle.ConstantTimeCompare([]byte(req.Key), []byte(h.key)) != 1 {
		if live {
			h.badKeys++
			if h.badKeys >= phoneMaxBadKeys {
				h.key = "" // too many guesses: the user has to show a new QR code
			}
		}
		h.mu.Unlock()
		jsonErr(w, r, http.StatusUnauthorized, i18n.E("The QR code has expired or is not valid. Create a new code on the computer and scan it again."))
		return
	}
	if len(h.sessions) >= phoneMaxSessions {
		h.mu.Unlock()
		jsonErr(w, r, http.StatusTooManyRequests, i18n.E("too many phones are connected to this computer"))
		return
	}
	osName := "android"
	switch strings.ToLower(req.OS) {
	case "ios", "android", "other":
		osName = strings.ToLower(req.OS)
	}
	name := clip(req.Name, 40)
	if name == "" {
		name = "Phone"
	}
	s := &phoneSess{
		ID: randomID(), Name: name, OS: osName,
		Check:  randomDigits(4),
		cookie: hex.EncodeToString(randomBytes(32)), Created: time.Now(), LastSeen: time.Now(),
	}
	h.sessions[s.ID] = s
	h.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: phoneCookie, Value: s.cookie, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
	n.bus.Publish("phones", nil)
	n.bus.Publish("attention", map[string]any{"kind": "phone", "name": s.Name})
	writeJSON200(w, map[string]string{"check": s.Check})
}

type phoneHandlerFunc func(http.ResponseWriter, *http.Request, *phoneSess)

func (n *Node) phoneAuth(needApproved bool, next phoneHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(phoneCookie)
		var s *phoneSess
		if err == nil {
			h := n.phone
			h.mu.Lock()
			for _, cand := range h.sessions {
				if subtle.ConstantTimeCompare([]byte(cand.cookie), []byte(c.Value)) == 1 {
					s = cand
				}
			}
			if s != nil {
				s.LastSeen = time.Now()
			}
			h.mu.Unlock()
		}
		if s == nil {
			jsonErr(w, r, http.StatusUnauthorized, i18n.E("Disconnected. Scan the QR code on the computer again."))
			return
		}
		if needApproved && !s.Approved {
			jsonErr(w, r, http.StatusForbidden, i18n.E("waiting for approval on the computer"))
			return
		}
		next(w, r, s)
	}
}

func (n *Node) pState(w http.ResponseWriter, r *http.Request, s *phoneSess) {
	n.phone.mu.Lock()
	out := []outItem{}
	for _, it := range s.outbox {
		out = append(out, *it)
	}
	approved, check := s.Approved, s.Check
	n.phone.mu.Unlock()
	fp := phoneFP(s.ID)
	msgs := []Message{}
	for _, m := range n.messageList() {
		if m.PeerFP == fp {
			// Stored from the computer's side; the phone sees the reverse.
			if m.Dir == "in" {
				m.Dir = "out"
			} else {
				m.Dir = "in"
			}
			msgs = append(msgs, m)
		}
	}
	ups := []TransferView{}
	for _, t := range n.transferViews(reqLang(r)) {
		if t.PeerFP == fp {
			ups = append(ups, t)
		}
	}
	writeJSON200(w, map[string]any{
		"approved": approved, "check": check, "computer": n.store.Settings().Name,
		"outbox": out, "messages": msgs, "uploads": ups,
	})
}

func (n *Node) pOffer(w http.ResponseWriter, r *http.Request, s *phoneSess) {
	var req struct {
		Files []FileMeta `json:"files"`
	}
	if !readBody(w, r, &req, 4<<20) {
		return
	}
	if len(req.Files) == 0 || len(req.Files) > 5000 {
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
	t := n.newTransfer(randomID(), "in", phoneFP(s.ID), req.Files)
	if free, ok := n.freeSpace(); ok && uint64(t.Total)+diskReserve > free {
		msg := i18n.E("not enough free space on the computer (%s needed)", humanBytes(t.Total))
		n.setState(t, "rejected", msg)
		jsonErr(w, r, http.StatusInsufficientStorage, msg)
		return
	}
	root := n.store.Settings().ReceiveDir
	if err := os.MkdirAll(root, 0o755); err != nil {
		n.setState(t, "failed", err)
		jsonErr(w, r, http.StatusInternalServerError, i18n.E("could not create the receive folder"))
		return
	}
	dest := planDestinations(root, rel)
	t.mu.Lock()
	t.dest = dest
	t.reveal = revealTarget(root, dest)
	t.mu.Unlock()
	n.setState(t, "active", nil)
	writeJSON200(w, map[string]string{"id": t.ID})
}

func (n *Node) pUpload(w http.ResponseWriter, r *http.Request, s *phoneSess) {
	t := n.transfer(r.PathValue("id"))
	if t == nil || t.Dir != "in" || t.PeerFP != phoneFP(s.ID) {
		jsonErr(w, r, http.StatusNotFound, i18n.E("transfer not found"))
		return
	}
	n.receiveInto(w, r, t, nil)
}

func (n *Node) pDownload(w http.ResponseWriter, r *http.Request, s *phoneSess) {
	n.phone.mu.Lock()
	var it *outItem
	for _, x := range s.outbox {
		if x.ID == r.PathValue("item") {
			it = x
		}
	}
	if it != nil {
		it.Downloads++
	}
	n.phone.mu.Unlock()
	if it == nil {
		http.Error(w, "file no longer available", http.StatusNotFound)
		return
	}
	f, err := os.Open(it.path)
	if err != nil {
		http.Error(w, "file no longer available", http.StatusNotFound)
		return
	}
	defer f.Close()
	ctype := mime.TypeByExtension(strings.ToLower(filepath.Ext(it.Name)))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(it.Name))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", it.Added, f)
	n.bus.Publish("phones", nil)
}

func (n *Node) pMessage(w http.ResponseWriter, r *http.Request, s *phoneSess) {
	var req messageReq
	if !readBody(w, r, &req, maxMessageBytes+4096) {
		return
	}
	if strings.TrimSpace(req.Text) == "" || len(req.Text) > maxMessageBytes {
		jsonErr(w, r, http.StatusBadRequest, i18n.E("text is empty or too long"))
		return
	}
	n.addMessage(Message{ID: randomID(), PeerFP: phoneFP(s.ID), Dir: "in", Text: req.Text, At: time.Now()})
	n.bus.Publish("attention", map[string]any{"kind": "message", "name": s.Name})
	writeJSON200(w, map[string]bool{"ok": true})
}
