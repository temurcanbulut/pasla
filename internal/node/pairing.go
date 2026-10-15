package node

import (
	"github.com/temurcanbulut/pasla/internal/i18n"

	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Pairing uses numeric comparison with a commitment, the same idea as
// Bluetooth "Secure Simple Pairing":
//
//   A (initiator)                         B (responder)
//   ── mutual TLS: each side sees the other's certificate fingerprint ──
//   start                       ──▶  picks Nb, replies commit = SHA256(Nb)
//   nonce: Na                   ──▶  replies Nb (now bound by the commitment)
//   both compute code = SHA256("pasla-pair-v1" | fpA | fpB | Na | Nb) mod 10⁶
//
// Both screens show the 6-digit code; the user confirms on both devices that
// the codes match. A machine-in-the-middle would hold two TLS sessions with
// different fingerprints and would have to make two independent codes
// collide while being forced to commit before seeing the other nonce, so it
// succeeds with probability 1 in a million per attempt — and every attempt
// pops up a visible request on the target device.

const pairTTL = 3 * time.Minute

type pairSession struct {
	ID       string
	Role     string // "in": the other device asked us; "out": we asked
	PeerFP   string
	PeerName string
	PeerOS   string
	PeerAddr string
	PeerIP   string
	nb, na   []byte
	Code     string
	State    string // "code": waiting for this user; "waiting": waiting for the other device

	decided chan struct{}
	localOK bool
	expires time.Time
}

type PairView struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	PeerFP   string `json:"peerFp"`
	PeerName string `json:"peerName"`
	PeerOS   string `json:"peerOs"`
	Code     string `json:"code"`
	State    string `json:"state"`
}

func (s *pairSession) view() PairView {
	return PairView{ID: s.ID, Role: s.Role, PeerFP: s.PeerFP, PeerName: s.PeerName, PeerOS: s.PeerOS, Code: s.Code, State: s.State}
}

func pairCode(fpInitiator, fpResponder string, na, nb []byte) string {
	h := sha256.New()
	h.Write([]byte("pasla-pair-v1|" + fpInitiator + "|" + fpResponder + "|"))
	h.Write(na)
	h.Write(nb)
	sum := h.Sum(nil)
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(sum[:4])%1_000_000)
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func randomID() string { return hex.EncodeToString(randomBytes(16)) }

func (n *Node) pairViews() []PairView {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := []PairView{}
	for _, s := range n.pairs {
		if s.Code != "" && s.State != "rejected" {
			out = append(out, s.view())
		}
	}
	return out
}

// ---- responder side (peer API) ---------------------------------------------

type pairStartReq struct {
	Name string `json:"name"`
	OS   string `json:"os"`
	Port int    `json:"port"`
}
type pairStartResp struct {
	SID    string `json:"sid"`
	Commit string `json:"commit"`
}
type pairNonceReq struct {
	SID string `json:"sid"`
	Na  string `json:"na"`
}
type pairNonceResp struct {
	Nb   string `json:"nb"`
	Name string `json:"name"`
	OS   string `json:"os"`
}
type pairSIDReq struct {
	SID string `json:"sid"`
}
type pairFinishResp struct {
	OK bool `json:"ok"`
}

func (n *Node) hPairStart(w http.ResponseWriter, r *http.Request) {
	fp := clientFP(r)
	var req pairStartReq
	if !readBody(w, r, &req, 4<<10) {
		return
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ok, wait := n.pairGuard.allow(fp, host, time.Now()); !ok {
		secs := int(wait.Seconds()) + 1
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		jsonErr(w, r, http.StatusTooManyRequests, i18n.E("this device is not accepting pairing requests for now; try again in %v s", secs))
		return
	}
	port := req.Port
	if port <= 0 || port > 65535 {
		port = DefaultPeerPort
	}
	n.mu.Lock()
	pending := 0
	for id, s := range n.pairs {
		if s.Role == "in" {
			if s.PeerFP == fp {
				delete(n.pairs, id)
				continue
			}
			pending++
		}
	}
	if pending >= 4 {
		n.mu.Unlock()
		jsonErr(w, r, http.StatusTooManyRequests, i18n.E("too many pairing requests are waiting"))
		return
	}
	s := &pairSession{
		ID: randomID(), Role: "in", PeerFP: fp, PeerName: clip(req.Name, 64), PeerOS: clip(req.OS, 16),
		PeerAddr: net.JoinHostPort(host, strconv.Itoa(port)), PeerIP: host,
		nb: randomBytes(32), decided: make(chan struct{}), expires: time.Now().Add(pairTTL),
	}
	n.pairs[s.ID] = s
	n.mu.Unlock()
	sum := sha256.Sum256(s.nb)
	writeJSON200(w, pairStartResp{SID: s.ID, Commit: hex.EncodeToString(sum[:])})
}

func (n *Node) inSession(r *http.Request, sid string) *pairSession {
	n.mu.Lock()
	defer n.mu.Unlock()
	s, ok := n.pairs[sid]
	if !ok || s.Role != "in" || s.PeerFP != clientFP(r) {
		return nil
	}
	return s
}

func (n *Node) hPairNonce(w http.ResponseWriter, r *http.Request) {
	var req pairNonceReq
	if !readBody(w, r, &req, 4<<10) {
		return
	}
	na, err := hex.DecodeString(req.Na)
	if err != nil || len(na) != 32 {
		jsonErr(w, r, http.StatusBadRequest, i18n.E("invalid nonce"))
		return
	}
	s := n.inSession(r, req.SID)
	if s == nil {
		jsonErr(w, r, http.StatusNotFound, i18n.E("pairing session not found"))
		return
	}
	n.mu.Lock()
	if s.na != nil {
		n.mu.Unlock()
		jsonErr(w, r, http.StatusConflict, i18n.E("nonce already received"))
		return
	}
	s.na = na
	s.Code = pairCode(s.PeerFP, n.fp, s.na, s.nb)
	s.State = "code"
	n.mu.Unlock()
	n.bus.Publish("pairs", nil)
	n.bus.Publish("attention", map[string]string{"kind": "pair", "name": s.PeerName})
	writeJSON200(w, pairNonceResp{Nb: hex.EncodeToString(s.nb), Name: n.store.Settings().Name, OS: osName()})
}

func (n *Node) hPairFinish(w http.ResponseWriter, r *http.Request) {
	var req pairSIDReq
	if !readBody(w, r, &req, 4<<10) {
		return
	}
	s := n.inSession(r, req.SID)
	if s == nil || s.na == nil {
		jsonErr(w, r, http.StatusNotFound, i18n.E("pairing session not found"))
		return
	}
	timer := time.NewTimer(time.Until(s.expires))
	defer timer.Stop()
	select {
	case <-s.decided:
	case <-r.Context().Done():
		n.dropPair(s.ID)
		n.pairGuard.penalize(s.PeerFP, s.PeerIP, time.Now())
		return
	case <-timer.C:
		n.dropPair(s.ID)
		n.pairGuard.penalize(s.PeerFP, s.PeerIP, time.Now())
		writeJSON200(w, pairFinishResp{OK: false})
		return
	}
	n.dropPair(s.ID)
	if !s.localOK {
		writeJSON200(w, pairFinishResp{OK: false})
		return
	}
	n.pairGuard.forgive(s.PeerFP, s.PeerIP)
	if err := n.store.Trust(TrustedPeer{FP: s.PeerFP, Name: s.PeerName, OS: s.PeerOS, PairedAt: time.Now(), LastAddr: s.PeerAddr}); err != nil {
		jsonErr(w, r, http.StatusInternalServerError, err)
		return
	}
	n.dropClient(s.PeerFP)
	n.bus.Publish("peers", nil)
	n.bus.Publish("paired", map[string]any{"name": s.PeerName, "fp": s.PeerFP, "ok": true})
	writeJSON200(w, pairFinishResp{OK: true})
}

func (n *Node) hPairCancel(w http.ResponseWriter, r *http.Request) {
	var req pairSIDReq
	if !readBody(w, r, &req, 4<<10) {
		return
	}
	if s := n.inSession(r, req.SID); s != nil {
		n.dropPair(s.ID)
		n.pairGuard.penalize(s.PeerFP, s.PeerIP, time.Now())
		n.bus.Publish("paired", map[string]any{"name": s.PeerName, "fp": s.PeerFP, "ok": false, "error": i18n.T(n.lang(), "the other device cancelled pairing")})
	}
	writeJSON200(w, map[string]bool{"ok": true})
}

func (n *Node) dropPair(id string) {
	n.mu.Lock()
	_, ok := n.pairs[id]
	delete(n.pairs, id)
	n.mu.Unlock()
	if ok {
		n.bus.Publish("pairs", nil)
	}
}

// ---- initiator side ---------------------------------------------------------

// StartPair contacts a visible device and returns the code to compare.
func (n *Node) StartPair(ctx context.Context, fp string) (PairView, error) {
	if fp == n.fp {
		return PairView{}, i18n.E("a device cannot pair with itself")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	addr, ok := n.addrOf(fp)
	if !ok {
		return PairView{}, i18n.E("the device is not visible on the network right now")
	}
	var st pairStartResp
	if err := n.callPeer(ctx, fp, addr, http.MethodPost, "/v1/pair/start",
		pairStartReq{Name: n.store.Settings().Name, OS: osName(), Port: n.cfg.PeerPort}, &st); err != nil {
		return PairView{}, err
	}
	na := randomBytes(32)
	var nr pairNonceResp
	if err := n.callPeer(ctx, fp, addr, http.MethodPost, "/v1/pair/nonce",
		pairNonceReq{SID: st.SID, Na: hex.EncodeToString(na)}, &nr); err != nil {
		return PairView{}, err
	}
	nb, err := hex.DecodeString(nr.Nb)
	commit, err2 := hex.DecodeString(st.Commit)
	sum := sha256.Sum256(nb)
	if err != nil || err2 != nil || len(nb) != 32 || !bytes.Equal(sum[:], commit) {
		return PairView{}, i18n.E("the other device broke the pairing protocol")
	}
	s := &pairSession{
		ID: st.SID, Role: "out", PeerFP: fp, PeerName: clip(nr.Name, 64), PeerOS: clip(nr.OS, 16), PeerAddr: addr,
		na: na, nb: nb, Code: pairCode(n.fp, fp, na, nb), State: "code",
		decided: make(chan struct{}), expires: time.Now().Add(pairTTL),
	}
	n.mu.Lock()
	n.pairs[s.ID] = s
	n.mu.Unlock()
	n.bus.Publish("pairs", nil)
	return s.view(), nil
}

// DecidePair records this user's answer to "do the codes match?".
func (n *Node) DecidePair(id string, ok bool) error {
	n.mu.Lock()
	s, found := n.pairs[id]
	if !found || s.State != "code" {
		n.mu.Unlock()
		return i18n.E("pairing session not found or expired")
	}
	s.localOK = ok
	if ok {
		s.State = "waiting"
	}
	close(s.decided)
	n.mu.Unlock()

	if s.Role == "in" {
		if !ok {
			// Keep the session until the other side's finish call collects
			// the answer; just hide it and make the requester wait.
			n.mu.Lock()
			s.State = "rejected"
			n.mu.Unlock()
			n.pairGuard.penalize(s.PeerFP, s.PeerIP, time.Now())
		}
		n.bus.Publish("pairs", nil)
		return nil
	}

	if !ok {
		n.dropPair(id)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = n.callPeer(ctx, s.PeerFP, s.PeerAddr, http.MethodPost, "/v1/pair/cancel", pairSIDReq{SID: id}, nil)
		}()
		return nil
	}
	n.bus.Publish("pairs", nil)
	go func() {
		ctx, cancel := context.WithDeadline(context.Background(), s.expires)
		defer cancel()
		var res pairFinishResp
		err := n.callPeer(ctx, s.PeerFP, s.PeerAddr, http.MethodPost, "/v1/pair/finish", pairSIDReq{SID: id}, &res)
		n.dropPair(id)
		ev := map[string]any{"name": s.PeerName, "fp": s.PeerFP, "ok": false}
		switch {
		case err != nil:
			ev["error"] = err.Error()
		case !res.OK:
			ev["error"] = i18n.T(n.lang(), "pairing was not confirmed on the other device")
		default:
			if err := n.store.Trust(TrustedPeer{FP: s.PeerFP, Name: s.PeerName, OS: s.PeerOS, PairedAt: time.Now(), LastAddr: s.PeerAddr}); err != nil {
				ev["error"] = err.Error()
			} else {
				ev["ok"] = true
			}
		}
		n.bus.Publish("peers", nil)
		n.bus.Publish("paired", ev)
	}()
	return nil
}

// Unpair forgets a device locally and tells it (best effort) to forget us.
func (n *Node) Unpair(fp string) error {
	if _, ok := n.store.Trusted(fp); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = n.callPeer(ctx, fp, "", http.MethodPost, "/v1/unpair", struct{}{}, nil)
		cancel()
	}
	if err := n.store.Untrust(fp); err != nil {
		return err
	}
	n.dropClient(fp)
	n.bus.Publish("peers", nil)
	return nil
}

func (n *Node) hUnpair(w http.ResponseWriter, r *http.Request) {
	fp := clientFP(r)
	if _, ok := n.store.Trusted(fp); ok {
		_ = n.store.Untrust(fp)
		n.dropClient(fp)
		n.bus.Publish("peers", nil)
	}
	writeJSON200(w, map[string]bool{"ok": true})
}
