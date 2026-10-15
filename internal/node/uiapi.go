package node

import (
	"github.com/temurcanbulut/pasla/internal/i18n"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/temurcanbulut/pasla/internal/ui"
)

// uiHandler serves the local web interface and its JSON API. It is meant to
// listen on the loopback interface only. Two checks keep other web pages in
// the same browser from driving it: the Host header must be a loopback name
// (defeats DNS rebinding) and state-changing requests must not come from a
// foreign Origin (defeats cross-site requests).
func (n *Node) uiHandler() http.Handler {
	root := http.NewServeMux()
	static := http.FileServerFS(ui.Static)
	root.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		static.ServeHTTP(w, r)
	}))
	root.HandleFunc("GET /open", n.hOpen)
	// Unauthenticated liveness check: lets a second copy find the first.
	root.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON200(w, map[string]string{"app": "pasla"})
	})

	mux := http.NewServeMux()
	root.Handle("/api/", n.requireSession(mux))
	mux.HandleFunc("POST /api/quit", func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, nil, nil)
		go func() { time.Sleep(300 * time.Millisecond); n.Quit() }()
	})
	mux.HandleFunc("POST /api/launch-code", func(w http.ResponseWriter, r *http.Request) {
		writeJSON200(w, map[string]string{"url": n.LaunchURL()})
	})

	mux.HandleFunc("GET /api/state", n.uState)
	mux.HandleFunc("GET /api/events", n.uEvents)
	mux.HandleFunc("PUT /api/settings", n.uSettings)
	mux.HandleFunc("POST /api/reveal-receive-dir", func(w http.ResponseWriter, r *http.Request) {
		dir := n.store.Settings().ReceiveDir
		_ = mkdirAll(dir)
		respond(w, r, nil, reveal(dir))
	})

	mux.HandleFunc("POST /api/manual-peers", n.uAddManual)
	mux.HandleFunc("DELETE /api/manual-peers", n.uRemoveManual)

	mux.HandleFunc("POST /api/peers/{fp}/pair", func(w http.ResponseWriter, r *http.Request) {
		v, err := n.StartPair(r.Context(), r.PathValue("fp"))
		respond(w, r, v, err)
	})
	mux.HandleFunc("POST /api/pairs/{id}/{answer}", func(w http.ResponseWriter, r *http.Request) {
		a := r.PathValue("answer")
		if a != "accept" && a != "reject" {
			jsonErr(w, r, http.StatusNotFound, i18n.E("unknown action"))
			return
		}
		respond(w, r, nil, n.DecidePair(r.PathValue("id"), a == "accept"))
	})
	mux.HandleFunc("DELETE /api/peers/{fp}/trust", func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, nil, n.Unpair(r.PathValue("fp")))
	})
	mux.HandleFunc("PUT /api/peers/{fp}/auto-accept", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			On bool `json:"on"`
		}
		if !readBody(w, r, &req, 1024) {
			return
		}
		err := n.store.UpdateTrusted(r.PathValue("fp"), func(t *TrustedPeer) { t.AutoAccept = req.On })
		n.bus.Publish("peers", nil)
		respond(w, r, nil, err)
	})
	mux.HandleFunc("POST /api/peers/{fp}/message", func(w http.ResponseWriter, r *http.Request) {
		var req messageReq
		if !readBody(w, r, &req, maxMessageBytes+4096) {
			return
		}
		respond(w, r, nil, n.SendMessage(r.PathValue("fp"), req.Text))
	})
	mux.HandleFunc("POST /api/peers/{fp}/offer", n.uOffer)

	mux.HandleFunc("GET /api/transfers/{id}", func(w http.ResponseWriter, r *http.Request) {
		t := n.transfer(r.PathValue("id"))
		if t == nil {
			jsonErr(w, r, http.StatusNotFound, i18n.E("transfer not found"))
			return
		}
		writeJSON200(w, t.view(n.lang()))
	})
	mux.HandleFunc("PUT /api/transfers/{id}/files/{index}", n.uUpload)
	mux.HandleFunc("POST /api/transfers/{id}/{action}", n.uTransferAction)
	mux.HandleFunc("DELETE /api/transfers", func(w http.ResponseWriter, r *http.Request) {
		n.ClearFinished()
		respond(w, r, nil, nil)
	})
	mux.HandleFunc("POST /api/send-paths", n.uSendPaths)

	mux.HandleFunc("POST /api/phone/start", func(w http.ResponseWriter, r *http.Request) {
		v, err := n.StartPhone()
		respond(w, r, v, err)
	})
	mux.HandleFunc("POST /api/phone/stop", func(w http.ResponseWriter, r *http.Request) {
		n.StopPhone()
		respond(w, r, nil, nil)
	})
	mux.HandleFunc("POST /api/phones/{id}/{answer}", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("answer") {
		case "accept", "reject":
			respond(w, r, nil, n.DecidePhone(r.PathValue("id"), r.PathValue("answer") == "accept"))
		case "message":
			var req messageReq
			if !readBody(w, r, &req, maxMessageBytes+4096) {
				return
			}
			respond(w, r, nil, n.SendPhoneMessage(r.PathValue("id"), req.Text))
		default:
			jsonErr(w, r, http.StatusNotFound, i18n.E("unknown action"))
		}
	})
	mux.HandleFunc("DELETE /api/phones/{id}", func(w http.ResponseWriter, r *http.Request) {
		n.DisconnectPhone(r.PathValue("id"))
		respond(w, r, nil, nil)
	})
	mux.HandleFunc("PUT /api/phones/{id}/outbox", func(w http.ResponseWriter, r *http.Request) {
		name, err := url.QueryUnescape(r.Header.Get("X-File-Name"))
		if err != nil || name == "" || r.ContentLength < 0 {
			jsonErr(w, r, http.StatusBadRequest, i18n.E("file name or size is missing"))
			return
		}
		it, err := n.AddToOutbox(r.PathValue("id"), name, r.ContentLength, r.Body)
		respond(w, r, it, err)
	})
	mux.HandleFunc("DELETE /api/phones/{id}/outbox/{item}", func(w http.ResponseWriter, r *http.Request) {
		n.RemoveFromOutbox(r.PathValue("id"), r.PathValue("item"))
		respond(w, r, nil, nil)
	})

	return n.guard(root)
}

func (n *Node) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(r.Host); err == nil {
			host = h
		}
		host = strings.Trim(host, "[]")
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			http.Error(w, "The Pasla interface can only be opened on this computer.", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
				http.Error(w, "foreign origin", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		jsonErr(w, r, http.StatusBadRequest, err)
		return
	}
	if v == nil {
		v = map[string]bool{"ok": true}
	}
	writeJSON200(w, v)
}

type meView struct {
	FP          string   `json:"fp"`
	Name        string   `json:"name"`
	OS          string   `json:"os"`
	Version     string   `json:"version"`
	Port        int      `json:"port"`
	ReceiveDir  string   `json:"receiveDir"`
	AutoAccept  bool     `json:"autoAccept"`
	Addrs       []string `json:"addrs"`
	ManualPeers []string `json:"manualPeers"`
	Discovery   bool     `json:"discovery"`
	Lang        string   `json:"lang"`
	KeepRunning bool     `json:"keepRunning"`
}

func (n *Node) me() meView {
	s := n.store.Settings()
	return meView{
		FP: n.fp, Name: s.Name, OS: osName(), Version: n.cfg.Version, Port: n.cfg.PeerPort,
		ReceiveDir: s.ReceiveDir, AutoAccept: s.AutoAccept, Addrs: localIPv4s(),
		ManualPeers: append([]string{}, s.ManualPeers...), Discovery: n.udp != nil, Lang: i18n.Norm(s.Lang),
		KeepRunning: s.KeepRunning,
	}
}

func (n *Node) uState(w http.ResponseWriter, r *http.Request) {
	writeJSON200(w, map[string]any{
		"me": n.me(), "peers": n.peerViews(), "pairs": n.pairViews(),
		"transfers": n.transferViews(n.lang()), "messages": n.messageList(),
		"phone": n.phoneView(), "phones": n.phoneSessViews(),
	})
}

func (n *Node) uEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	ch, unsub := n.bus.Subscribe()
	defer unsub()
	defer n.windowOpened()()
	fmt.Fprint(w, "retry: 1500\n\n")
	fl.Flush()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		fl.Flush()
	}
}

func (n *Node) uSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        *string `json:"name"`
		ReceiveDir  *string `json:"receiveDir"`
		AutoAccept  *bool   `json:"autoAccept"`
		Lang        *string `json:"lang"`
		KeepRunning *bool   `json:"keepRunning"`
	}
	if !readBody(w, r, &req, 8<<10) {
		return
	}
	if req.Name != nil {
		name := clip(*req.Name, 64)
		if name == "" {
			jsonErr(w, r, http.StatusBadRequest, i18n.E("device name cannot be empty"))
			return
		}
		req.Name = &name
	}
	if req.ReceiveDir != nil {
		dir := strings.TrimSpace(*req.ReceiveDir)
		if !filepath.IsAbs(dir) {
			jsonErr(w, r, http.StatusBadRequest, i18n.E("the folder must be a full path (e.g. %s)", examplePath()))
			return
		}
		if err := mkdirAll(dir); err != nil {
			jsonErr(w, r, http.StatusBadRequest, i18n.E("could not create the folder: %v", err))
			return
		}
		dir = filepath.Clean(dir)
		req.ReceiveDir = &dir
	}
	err := n.store.UpdateSettings(func(s *Settings) {
		if req.Name != nil {
			s.Name = *req.Name
		}
		if req.ReceiveDir != nil {
			s.ReceiveDir = *req.ReceiveDir
		}
		if req.AutoAccept != nil {
			s.AutoAccept = *req.AutoAccept
		}
		if req.Lang != nil {
			s.Lang = i18n.Norm(*req.Lang)
		}
		if req.KeepRunning != nil {
			s.KeepRunning = *req.KeepRunning
		}
	})
	if req.Lang != nil {
		i18n.SetDefault(*req.Lang)
	}
	if err == nil && req.Name != nil {
		n.broadcast(n.hello(false, false))
	}
	n.bus.Publish("me", nil)
	respond(w, r, n.me(), err)
}

func (n *Node) uAddManual(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Addr string `json:"addr"`
	}
	if !readBody(w, r, &req, 1024) {
		return
	}
	addr, err := normalizeAddr(req.Addr)
	if err != nil {
		jsonErr(w, r, http.StatusBadRequest, err)
		return
	}
	info, err := n.probe(r.Context(), addr)
	if err != nil {
		jsonErr(w, r, http.StatusBadGateway, i18n.E("%s: %v", addr, err))
		return
	}
	_ = n.store.UpdateSettings(func(s *Settings) {
		for _, a := range s.ManualPeers {
			if a == addr {
				return
			}
		}
		s.ManualPeers = append(s.ManualPeers, addr)
	})
	n.bus.Publish("me", nil)
	writeJSON200(w, info)
}

func (n *Node) uRemoveManual(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Addr string `json:"addr"`
	}
	if !readBody(w, r, &req, 1024) {
		return
	}
	err := n.store.UpdateSettings(func(s *Settings) {
		out := s.ManualPeers[:0]
		for _, a := range s.ManualPeers {
			if a != req.Addr {
				out = append(out, a)
			}
		}
		s.ManualPeers = out
	})
	n.bus.Publish("me", nil)
	respond(w, r, nil, err)
}

func (n *Node) uOffer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Files []FileMeta `json:"files"`
	}
	if !readBody(w, r, &req, 64<<20) {
		return
	}
	t, err := n.Offer(r.PathValue("fp"), req.Files)
	if err != nil {
		jsonErr(w, r, http.StatusBadRequest, err)
		return
	}
	n.WaitDecision(r.Context(), t)
	if r.Context().Err() != nil {
		_ = n.Cancel(t.ID)
		return
	}
	writeJSON200(w, t.view(n.lang()))
}

func (n *Node) uUpload(w http.ResponseWriter, r *http.Request) {
	t := n.transfer(r.PathValue("id"))
	if t == nil || t.Dir != "out" {
		jsonErr(w, r, http.StatusNotFound, i18n.E("transfer not found"))
		return
	}
	i, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || i < 0 || i >= len(t.Files) {
		jsonErr(w, r, http.StatusBadRequest, i18n.E("invalid file number"))
		return
	}
	if r.ContentLength >= 0 && r.ContentLength != t.Files[i].Size {
		jsonErr(w, r, http.StatusBadRequest, i18n.E("file size does not match the offer"))
		return
	}
	if err := n.SendFile(t, i, r.Body); err != nil {
		jsonErr(w, r, http.StatusConflict, err)
		return
	}
	writeJSON200(w, map[string]bool{"ok": true})
}

func (n *Node) uTransferAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	switch r.PathValue("action") {
	case "accept":
		respond(w, r, nil, n.DecideOffer(id, true))
	case "reject":
		respond(w, r, nil, n.DecideOffer(id, false))
	case "cancel":
		respond(w, r, nil, n.Cancel(id))
	case "reveal":
		t := n.transfer(id)
		if t == nil {
			jsonErr(w, r, http.StatusNotFound, i18n.E("transfer not found"))
			return
		}
		t.mu.Lock()
		target := t.reveal
		t.mu.Unlock()
		if target == "" {
			target = n.store.Settings().ReceiveDir
		}
		respond(w, r, nil, reveal(target))
	default:
		jsonErr(w, r, http.StatusNotFound, i18n.E("unknown action"))
	}
}

// resolvePeer finds a paired device by fingerprint prefix or by name.
func (n *Node) resolvePeer(q string) (string, error) {
	q = strings.TrimSpace(q)
	var matches []TrustedPeer
	for _, t := range n.store.TrustedList() {
		if strings.EqualFold(t.Name, q) || (len(q) >= 6 && strings.HasPrefix(t.FP, strings.ToLower(q))) {
			matches = append(matches, t)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0].FP, nil
	case 0:
		return "", i18n.E("no paired device is called “%s”", q)
	default:
		return "", i18n.E("“%s” matches more than one device; use the first characters of its fingerprint", q)
	}
}

func (n *Node) uSendPaths(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To    string   `json:"to"`
		Paths []string `json:"paths"`
	}
	if !readBody(w, r, &req, 4<<20) {
		return
	}
	fp, err := n.resolvePeer(req.To)
	if err != nil {
		jsonErr(w, r, http.StatusBadRequest, err)
		return
	}
	t, err := n.SendPaths(fp, req.Paths)
	if err != nil {
		jsonErr(w, r, http.StatusBadRequest, err)
		return
	}
	writeJSON200(w, t.view(n.lang()))
}

// ListenUI opens the interface listener. If another Pasla already owns the
// port it reports ErrAlreadyRunning so the caller can just open a browser.
func ListenUI(addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		return ln, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/ping", nil)
	if resp, herr := http.DefaultClient.Do(req); herr == nil {
		var v struct{ App string }
		_ = json.NewDecoder(resp.Body).Decode(&v)
		resp.Body.Close()
		if v.App == "pasla" {
			return nil, ErrAlreadyRunning
		}
	}
	return nil, err
}

// RunningLaunchURL asks the copy already listening on addr for a one-time
// sign-in address. Only the user who owns dir can read its token.
func RunningLaunchURL(addr, dir string) (string, error) {
	tok, err := ReadToken(dir)
	if err != nil {
		return "", err
	}
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/api/launch-code", nil)
	req.Header.Set(tokenHeader, tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var v struct{ URL string }
	if err := decodeResp(resp, &v); err != nil {
		return "", err
	}
	return v.URL, nil
}

func examplePath() string {
	if runtime.GOOS == "windows" {
		return `C:\Users\me\Downloads`
	}
	return "/home/me/Downloads"
}

// TokenHeader is the header the command line client authenticates with.
const TokenHeader = tokenHeader

var ErrAlreadyRunning = errors.New("pasla is already running")
