// Package node implements an Pasla device: discovery, pairing, transfers,
// the peer-to-peer TLS API and the local web interface.
package node

import (
	"github.com/temurcanbulut/pasla/internal/i18n"

	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultPeerPort      = 47821
	DefaultDiscoveryPort = 47820
	DefaultUIAddr        = "127.0.0.1:47822"
	protocolVersion      = 1
)

// Config is fixed at start-up (command-line flags).
type Config struct {
	Dir           string // configuration directory
	PeerPort      int
	DiscoveryPort int
	UIAddr        string
	PhonePort     int
	NoDiscovery   bool
	Name          string // overrides the stored name when non-empty
	ReceiveDir    string // overrides the stored folder when non-empty
	Version       string
	Logger        *log.Logger
	// Notify shows a short message outside the browser (the Windows tray
	// balloon). Optional; messages are logged either way.
	Notify func(title, text string)
}

// Peer is a device currently visible on the network.
type Peer struct {
	FP       string    `json:"fp"`
	Name     string    `json:"name"`
	OS       string    `json:"os"`
	Host     string    `json:"host"`
	Port     int       `json:"port"`
	Manual   bool      `json:"manual"`
	LastSeen time.Time `json:"-"`
}

func (p *Peer) addr() string { return net.JoinHostPort(p.Host, strconv.Itoa(p.Port)) }

type Node struct {
	cfg   Config
	log   *log.Logger
	cert  tls.Certificate
	fp    string
	store *Store
	bus   *Bus

	mu        sync.Mutex
	peers     map[string]*Peer
	pairs     map[string]*pairSession
	transfers map[string]*Transfer
	order     []string // transfer ids, oldest first
	messages  []Message

	launchCodes map[string]time.Time
	pairGuard   *pairGuard

	clientsMu sync.Mutex
	clients   map[string]*http.Client

	uiToken  string
	quit     chan struct{}
	quitOnce sync.Once
	win      windowWatch
	phone    *phoneHub

	discoveryMu     sync.Mutex
	discoveryRecent map[string]time.Time
	discoveryActive int
	udp             *net.UDPConn
}

func New(cfg Config) (*Node, error) {
	if cfg.Logger == nil {
		cfg.Logger = log.New(os.Stderr, "pasla ", log.LstdFlags)
	}
	if cfg.PeerPort == 0 {
		cfg.PeerPort = DefaultPeerPort
	}
	if cfg.DiscoveryPort == 0 {
		cfg.DiscoveryPort = DefaultDiscoveryPort
	}
	if cfg.PhonePort == 0 {
		cfg.PhonePort = DefaultPhonePort
	}
	if cfg.UIAddr == "" {
		cfg.UIAddr = DefaultUIAddr
	}
	if cfg.Dir == "" {
		dir, err := DefaultDir()
		if err != nil {
			return nil, err
		}
		cfg.Dir = dir
	}
	if err := os.MkdirAll(cfg.Dir, 0o700); err != nil {
		return nil, err
	}
	cert, fp, err := loadOrCreateIdentity(cfg.Dir)
	if err != nil {
		return nil, err
	}
	store, err := openStore(cfg.Dir, Settings{Name: defaultName(), ReceiveDir: defaultReceiveDir()})
	if err != nil {
		return nil, fmt.Errorf("could not read settings: %w", err)
	}
	if cfg.Name != "" || cfg.ReceiveDir != "" {
		_ = store.UpdateSettings(func(s *Settings) {
			if cfg.Name != "" {
				s.Name = cfg.Name
			}
			if cfg.ReceiveDir != "" {
				s.ReceiveDir = cfg.ReceiveDir
			}
		})
	}
	n := &Node{
		cfg: cfg, log: cfg.Logger, cert: cert, fp: fp, store: store, bus: newBus(),
		peers: map[string]*Peer{}, pairs: map[string]*pairSession{},
		transfers: map[string]*Transfer{}, clients: map[string]*http.Client{},
		launchCodes: map[string]time.Time{}, pairGuard: newPairGuard(), phone: newPhoneHub(),
		quit: make(chan struct{}),
	}
	if err := n.writeToken(); err != nil {
		return nil, fmt.Errorf("could not write the session key: %w", err)
	}
	return n, nil
}

func (n *Node) Fingerprint() string { return n.fp }

// Quit asks Run to shut down (used by the interface and the tray icon).
func (n *Node) Quit() { n.quitOnce.Do(func() { close(n.quit) }) }

// lang is the language the user picked for this device's interface.
func (n *Node) lang() string  { return i18n.Norm(n.store.Settings().Lang) }
func (n *Node) Store() *Store { return n.store }

func defaultName() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "Pasla device"
	}
	return h
}

func defaultReceiveDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "Pasla"
	}
	if st, err := os.Stat(filepath.Join(home, "Downloads")); err == nil && st.IsDir() {
		return filepath.Join(home, "Downloads", "Pasla")
	}
	return filepath.Join(home, "Pasla")
}

func osName() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "macos"
	default:
		return runtime.GOOS
	}
}

// Run starts every service and blocks until ctx is cancelled or a listener
// fails. uiLn is the already-open listener for the web interface.
func (n *Node) Run(ctx context.Context, uiLn net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	peerLn, err := net.Listen("tcp", fmt.Sprintf(":%d", n.cfg.PeerPort))
	if err != nil {
		return i18n.E("could not open transfer port %v: %v", n.cfg.PeerPort, err)
	}
	peerSrv := &http.Server{
		Handler:           n.peerHandler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{n.cert},
			ClientAuth:   tls.RequireAnyClientCert,
			MinVersion:   tls.VersionTLS13,
		},
		ErrorLog: log.New(discardWriter{}, "", 0),
	}
	uiSrv := &http.Server{Handler: n.uiHandler(), ReadHeaderTimeout: 15 * time.Second}

	errc := make(chan error, 3)
	go func() { errc <- peerSrv.ServeTLS(peerLn, "", "") }()
	go func() { errc <- uiSrv.Serve(uiLn) }()

	if !n.cfg.NoDiscovery {
		if err := n.startDiscovery(ctx); err != nil {
			n.log.Print(i18n.T(n.lang(), "automatic discovery is off: %v", err))
		}
	}
	go n.manualProbeLoop(ctx)
	go n.housekeeping(ctx)

	var runErr error
	select {
	case <-ctx.Done():
	case <-n.quit:
	case runErr = <-errc:
	}
	n.sayGoodbye()
	n.StopPhone()
	shutCtx, c := context.WithTimeout(context.Background(), 2*time.Second)
	defer c()
	_ = peerSrv.Shutdown(shutCtx)
	_ = uiSrv.Shutdown(shutCtx)
	if errors.Is(runErr, http.ErrServerClosed) {
		runErr = nil
	}
	return runErr
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// housekeeping expires silent peers and stale pairing sessions, and pushes
// progress snapshots of running transfers to the UI.
func (n *Node) housekeeping(ctx context.Context) {
	tick := time.NewTicker(400 * time.Millisecond)
	defer tick.Stop()
	var lastPrune time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			n.publishProgress(now)
			n.checkWindow(now)
			if now.Sub(lastPrune) > 2*time.Second {
				lastPrune = now
				n.prune(now)
				n.prunePhones(now)
			}
		}
	}
}

func (n *Node) prune(now time.Time) {
	n.mu.Lock()
	changed := false
	for fp, p := range n.peers {
		if now.Sub(p.LastSeen) > 12*time.Second {
			delete(n.peers, fp)
			changed = true
		}
	}
	pairsChanged := false
	for id, s := range n.pairs {
		if now.After(s.expires) {
			delete(n.pairs, id)
			pairsChanged = true
			if s.Role == "in" && s.State != "rejected" {
				n.pairGuard.penalize(s.PeerFP, s.PeerIP, now)
			}
		}
	}
	n.mu.Unlock()
	if changed {
		n.bus.Publish("peers", nil)
	}
	if pairsChanged {
		n.bus.Publish("pairs", nil)
	}
}

// seePeer records only certificate-verified /v1/info results, never raw UDP claims.
func (n *Node) seePeer(p Peer) bool {
	p.LastSeen = time.Now()
	n.mu.Lock()
	old, ok := n.peers[p.FP]
	changed := !ok || old.Name != p.Name || old.Host != p.Host || old.Port != p.Port
	if ok && old.Manual {
		p.Manual = true
	}
	cp := p
	n.peers[p.FP] = &cp
	n.mu.Unlock()
	if changed {
		n.bus.Publish("peers", nil)
	}
	if prior, trusted := n.store.Trusted(p.FP); trusted {
		if prior.Name != "" && p.Name != "" && prior.Name != p.Name {
			n.bus.Publish("peer-renamed", map[string]string{"fp": p.FP, "old": prior.Name, "name": p.Name})
		}
		_ = n.store.UpdateTrusted(p.FP, func(t *TrustedPeer) {
			t.LastAddr = cp.addr()
			if p.Name != "" {
				t.Name = p.Name
			}
			if p.OS != "" {
				t.OS = p.OS
			}
		})
	}
	return !ok
}

func (n *Node) forgetPeer(fp string) {
	n.mu.Lock()
	_, ok := n.peers[fp]
	delete(n.peers, fp)
	n.mu.Unlock()
	if ok {
		n.bus.Publish("peers", nil)
	}
}

// addrOf resolves a fingerprint to host:port using live discovery first and
// the last known address of a paired device second.
func (n *Node) addrOf(fp string) (string, bool) {
	n.mu.Lock()
	p, ok := n.peers[fp]
	n.mu.Unlock()
	if ok {
		return p.addr(), true
	}
	if t, ok := n.store.Trusted(fp); ok && t.LastAddr != "" {
		return t.LastAddr, true
	}
	return "", false
}

func (n *Node) peerName(fp string) string {
	if isPhoneFP(fp) {
		if s := n.phoneSession(strings.TrimPrefix(fp, "phone:")); s != nil {
			return s.Name
		}
		return "Phone"
	}
	n.mu.Lock()
	p, ok := n.peers[fp]
	n.mu.Unlock()
	if ok && p.Name != "" {
		return p.Name
	}
	if t, ok := n.store.Trusted(fp); ok {
		return t.Name
	}
	return fp[:8]
}

// PeerView is what the UI sees for one device.
type PeerView struct {
	FP         string `json:"fp"`
	Name       string `json:"name"`
	OS         string `json:"os"`
	Addr       string `json:"addr"`
	Online     bool   `json:"online"`
	Trusted    bool   `json:"trusted"`
	AutoAccept bool   `json:"autoAccept"`
	Manual     bool   `json:"manual"`
	Safety     string `json:"safety,omitempty"`
}

func (n *Node) peerViews() []PeerView {
	byFP := map[string]*PeerView{}
	n.mu.Lock()
	for _, p := range n.peers {
		byFP[p.FP] = &PeerView{FP: p.FP, Name: p.Name, OS: p.OS, Addr: p.Host, Online: true, Manual: p.Manual}
	}
	n.mu.Unlock()
	for _, t := range n.store.TrustedList() {
		v, ok := byFP[t.FP]
		if !ok {
			host, _, _ := net.SplitHostPort(t.LastAddr)
			v = &PeerView{FP: t.FP, Name: t.Name, OS: t.OS, Addr: host}
			byFP[t.FP] = v
		}
		v.Trusted = true
		v.AutoAccept = t.AutoAccept
		v.Safety = safetyNumber(n.fp, t.FP)
	}
	out := make([]PeerView, 0, len(byFP))
	for _, v := range byFP {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Online != b.Online {
			return a.Online
		}
		if a.Trusted != b.Trusted {
			return a.Trusted
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.FP < b.FP
	})
	return out
}

func localIPv4s() []string {
	var out []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				if ip4 := ipn.IP.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
					out = append(out, ip4.String())
				}
			}
		}
	}
	return out
}
