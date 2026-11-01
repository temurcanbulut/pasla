package node

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAuditForgedDiscoveryCannotRewriteTrustedPeer(t *testing.T) {
	a, b := startNode(t, "receiver"), startNode(t, "known laptop")
	attacker := startNode(t, "forged")
	addr := net.JoinHostPort("127.0.0.1", fmtPort(b.cfg.PeerPort))
	if _, err := a.probe(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	if err := a.store.Trust(TrustedPeer{FP: b.fp, Name: "known laptop", LastAddr: addr}); err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	go a.readHellos(conn)
	sender, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	spoof := hello{App: "pasla", V: protocolVersion, FP: b.fp, Name: "forged", Port: attacker.cfg.PeerPort}
	raw, _ := json.Marshal(spoof)
	_, _ = sender.Write(raw)
	time.Sleep(150 * time.Millisecond)
	tp, _ := a.store.Trusted(b.fp)
	if tp.Name != "known laptop" || tp.LastAddr != addr {
		t.Fatalf("UDP rewrote trust: %+v", tp)
	}
	spoof.Bye = true
	raw, _ = json.Marshal(spoof)
	_, _ = sender.Write(raw)
	time.Sleep(100 * time.Millisecond)
	a.mu.Lock()
	_, visible := a.peers[b.fp]
	a.mu.Unlock()
	if !visible {
		t.Fatal("unsigned goodbye removed authenticated peer")
	}
}

func fmtPort(p int) string { return fmt.Sprint(p) }

func TestAuditHugeTransferRejectedBeforeRegistration(t *testing.T) {
	n := startNode(t, "receiver")
	_ = n.store.UpdateSettings(func(s *Settings) { s.AutoAccept = true })
	files := []FileMeta{{"a", 1 << 62}, {"b", 1 << 62}, {"c", 1 << 62}, {"d", 1 << 62}}
	raw, _ := json.Marshal(offerReq{ID: strings.Repeat("a", 32), Files: files})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/offer", strings.NewReader(string(raw)))
	cert, err := x509.ParseCertificate(n.cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}
	n.hOffer(w, r)
	if w.Code != 400 || len(n.transfers) != 0 {
		t.Fatalf("huge offer registered: status %d", w.Code)
	}
	for _, size := range []int64{-1, math.MaxInt64} {
		raw, _ = json.Marshal(map[string]any{"files": []FileMeta{{"a", size}}})
		w = httptest.NewRecorder()
		r = httptest.NewRequest("POST", "/offer", strings.NewReader(string(raw)))
		n.pOffer(w, r, &phoneSess{ID: "test"})
		if w.Code != 400 {
			t.Fatalf("phone size %d: %d", size, w.Code)
		}
	}
}

func TestAuditConcurrentBatchFoldersAreIsolated(t *testing.T) {
	n := startNode(t, "receiver")
	root := t.TempDir()
	var wg sync.WaitGroup
	paths := make(chan string, 16)
	for i := 0; i < 16; i++ {
		tr, err := n.newTransfer(fmt.Sprint(i), "in", n.fp, []FileMeta{{"Foo/a.txt", 1}})
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func(tr *Transfer) {
			defer wg.Done()
			n.reserveDestinations(tr, root, [][]string{{"Foo", "a.txt"}})
			paths <- tr.dest[0]
		}(tr)
	}
	wg.Wait()
	close(paths)
	seen := map[string]bool{}
	for path := range paths {
		if seen[path] {
			t.Fatalf("batches share destination: %s", path)
		}
		seen[path] = true
	}
}

func TestAuditPhoneQRConsumedExactlyOnce(t *testing.T) {
	n := startNode(t, "receiver")
	n.phone.mu.Lock()
	n.phone.key = "offline-qr-key"
	n.phone.keyExp = time.Now().Add(time.Minute)
	n.phone.mu.Unlock()
	login := func(ip string) int {
		r := httptest.NewRequest("POST", "/p/login", strings.NewReader(`{"key":"offline-qr-key","name":"phone"}`))
		r.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		n.pLogin(w, r)
		return w.Code
	}
	if login("192.0.2.1") != 200 || login("192.0.2.2") != 401 {
		t.Fatal("QR accepted more than once")
	}
	if len(n.phone.sessions) != 1 {
		t.Fatal("multiple sessions created with one QR")
	}
}

func TestAuditDuplicatePairStartPreservesPendingSession(t *testing.T) {
	n := startNode(t, "receiver")
	cert, _ := x509.ParseCertificate(n.cert.Certificate[0])
	start := func() int {
		r := httptest.NewRequest("POST", "/v1/pair/start", strings.NewReader(`{"name":"fixture"}`))
		r.RemoteAddr = "192.0.2.1:1111"
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}
		w := httptest.NewRecorder()
		n.hPairStart(w, r)
		return w.Code
	}
	if start() != 200 {
		t.Fatal("initial request failed")
	}
	var id string
	for k := range n.pairs {
		id = k
	}
	if start() != 409 || n.pairs[id] == nil {
		t.Fatal("duplicate replaced pending SAS session")
	}
}

func TestAuditWindowsDangerousNames(t *testing.T) {
	for _, name := range []string{"CONIN$", "CONOUT$.txt", "COM¹.log", "LPT³"} {
		if sanitizeComponent(name) == name {
			t.Errorf("reserved name accepted: %s", name)
		}
	}
	for _, ext := range []string{".chm", ".msp", ".mst", ".cab", ".application", ".appref-ms", ".scf", ".library-ms", ".search-ms", ".settingcontent-ms", ".xll", ".xlam", ".dotm", ".py", ".pyw", ".ps2"} {
		if !isRisky("file" + ext) {
			t.Errorf("risky extension missing: %s", ext)
		}
	}
}

func TestAuditPairFloodIsScopedToSource(t *testing.T) {
	g := newPairGuard()
	now := time.Now()
	for i := 0; i < pairBurst; i++ {
		g.allow(fmt.Sprint(i), "192.0.2.1", now)
	}
	if ok, _ := g.allow("other", "192.0.2.2", now); !ok {
		t.Fatal("one source exhausted another source's pairing budget")
	}
}
