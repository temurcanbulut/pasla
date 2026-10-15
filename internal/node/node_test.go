package node

import (
	"bytes"

	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/temurcanbulut/pasla/internal/i18n"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

type testNode struct {
	*Node
	ui string
}

func startNode(t *testing.T, name string) testNode {
	t.Helper()
	dir := t.TempDir()
	n, err := New(Config{
		Dir: filepath.Join(dir, "cfg"), PeerPort: freePort(t), PhonePort: freePort(t), NoDiscovery: true, Name: name,
		ReceiveDir: filepath.Join(dir, "in"), Version: "test", Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = n.Run(ctx, ln); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	// wait until the peer port answers
	for i := 0; i < 50; i++ {
		if c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", n.cfg.PeerPort)); err == nil {
			c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return testNode{n, ln.Addr().String()}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("zaman aşımı: %s", what)
}

func pair(t *testing.T, a, b testNode) {
	t.Helper()
	ctx := context.Background()
	if _, err := a.probe(ctx, fmt.Sprintf("127.0.0.1:%d", b.cfg.PeerPort)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.probe(ctx, fmt.Sprintf("127.0.0.1:%d", a.cfg.PeerPort)); err != nil {
		t.Fatal(err)
	}
	out, err := a.StartPair(ctx, b.fp)
	if err != nil {
		t.Fatal(err)
	}
	var in PairView
	waitFor(t, "eşleştirme isteği", func() bool {
		for _, p := range b.pairViews() {
			if p.Role == "in" && p.PeerFP == a.fp {
				in = p
				return true
			}
		}
		return false
	})
	if in.Code != out.Code || len(in.Code) != 6 {
		t.Fatalf("kodlar farklı: %q / %q", out.Code, in.Code)
	}
	if err := a.DecidePair(out.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := b.DecidePair(in.ID, true); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "iki yönlü güven", func() bool {
		_, ab := a.store.Trusted(b.fp)
		_, ba := b.store.Trusted(a.fp)
		return ab && ba
	})
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func randData(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func TestPairSendFolderAndMessage(t *testing.T) {
	a, b := startNode(t, "Salon PC"), startNode(t, "Çalışma Odası")

	// Unpaired devices are refused.
	if err := a.SendMessage(b.fp, "selam"); err == nil {
		t.Fatal("eşleşmeden mesaj gitmemeli")
	}
	pair(t, a, b)

	src := t.TempDir()
	big := randData(5<<20 + 123)
	writeFile(t, filepath.Join(src, "Tatil", "deniz.jpg"), big)
	writeFile(t, filepath.Join(src, "Tatil", "notlar", "liste.txt"), []byte("güneş kremi\n"))
	writeFile(t, filepath.Join(src, "Tatil", "boş.txt"), nil)
	writeFile(t, filepath.Join(src, "tek.bin"), randData(1000))

	tr, err := a.SendPaths(b.fp, []string{filepath.Join(src, "Tatil"), filepath.Join(src, "tek.bin")})
	if err != nil {
		t.Fatal(err)
	}
	var incoming *Transfer
	waitFor(t, "gelen teklif", func() bool { incoming = b.transfer(tr.ID); return incoming != nil })
	if incoming.State() != "pending" {
		t.Fatalf("onay beklenmeliydi, durum %s", incoming.State())
	}
	if err := b.DecideOffer(tr.ID, true); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "aktarım bitişi", func() bool { return tr.State() == "done" && incoming.State() == "done" })

	recv := b.store.Settings().ReceiveDir
	got, _ := os.ReadFile(filepath.Join(recv, "Tatil", "deniz.jpg"))
	if !bytes.Equal(got, big) {
		t.Fatal("büyük dosya bozuk geldi")
	}
	if s, _ := os.ReadFile(filepath.Join(recv, "Tatil", "notlar", "liste.txt")); string(s) != "güneş kremi\n" {
		t.Fatalf("metin dosyası: %q", s)
	}
	if st, err := os.Stat(filepath.Join(recv, "Tatil", "boş.txt")); err != nil || st.Size() != 0 {
		t.Fatal("boş dosya oluşmadı")
	}

	// Same folder again: must not overwrite, lands in "Tatil (2)".
	b.store.UpdateTrusted(a.fp, func(p *TrustedPeer) { p.AutoAccept = true })
	tr2, err := a.SendPaths(b.fp, []string{filepath.Join(src, "Tatil")})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "ikinci aktarım", func() bool { return tr2.State() == "done" })
	if _, err := os.Stat(filepath.Join(recv, "Tatil (2)", "deniz.jpg")); err != nil {
		t.Fatal("çakışan klasör yeniden adlandırılmadı")
	}

	if err := a.SendMessage(b.fp, "Merhaba 👋"); err != nil {
		t.Fatal(err)
	}
	msgs := b.messageList()
	if len(msgs) != 1 || msgs[0].Text != "Merhaba 👋" || msgs[0].Dir != "in" {
		t.Fatalf("mesaj: %+v", msgs)
	}
}

func TestRejectAndUnpair(t *testing.T) {
	a, b := startNode(t, "A"), startNode(t, "B")
	pair(t, a, b)
	src := filepath.Join(t.TempDir(), "x.txt")
	writeFile(t, src, []byte("x"))
	tr, err := a.SendPaths(b.fp, []string{src})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "teklif", func() bool { return b.transfer(tr.ID) != nil })
	_ = b.DecideOffer(tr.ID, false)
	waitFor(t, "ret", func() bool { return tr.State() == "rejected" })

	if err := b.Unpair(a.fp); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "karşılıklı unutma", func() bool {
		_, ok := a.store.Trusted(b.fp)
		return !ok
	})
	if _, err := a.Offer(b.fp, []FileMeta{{Name: "x", Size: 1}}); err == nil {
		t.Fatal("eşleşme kalkınca gönderim engellenmeli")
	}
}

// A device holding a different certificate must not be able to impersonate
// a paired one: the pinned client refuses the TLS session.
func TestFingerprintPinning(t *testing.T) {
	a, b, c := startNode(t, "A"), startNode(t, "B"), startNode(t, "Sahte")
	pair(t, a, b)
	a.mu.Lock()
	a.peers[b.fp].Port = c.cfg.PeerPort // B "moves" to C's address
	a.mu.Unlock()
	err := a.SendMessage(b.fp, "gizli")
	if err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("yanlış cihaza bağlanılmamalıydı, hata: %v", err)
	}
	if len(c.messageList()) != 0 {
		t.Fatal("sahte cihaz mesaj aldı")
	}
}

// The browser path: offer over the local API, then stream bytes with PUT.
func TestUIUploadFlow(t *testing.T) {
	a, b := startNode(t, "A"), startNode(t, "B")
	pair(t, a, b)
	_ = b.store.UpdateSettings(func(s *Settings) { s.AutoAccept = true })

	data := randData(300_000)
	offer, _ := json.Marshal(map[string]any{"files": []FileMeta{{Name: "klasör/foto.png", Size: int64(len(data))}}})
	req, _ := http.NewRequest(http.MethodPost, "http://"+a.ui+"/api/peers/"+b.fp+"/offer", bytes.NewReader(offer))
	req.Header.Set(tokenHeader, a.uiToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var tv TransferView
	_ = json.NewDecoder(resp.Body).Decode(&tv)
	resp.Body.Close()
	if tv.State != "active" {
		t.Fatalf("teklif durumu %q (%s)", tv.State, tv.Error)
	}
	req, _ = http.NewRequest(http.MethodPut, "http://"+a.ui+"/api/transfers/"+tv.ID+"/files/0", bytes.NewReader(data))
	req.Header.Set(tokenHeader, a.uiToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("yükleme: %v %v", err, resp.Status)
	}
	resp.Body.Close()
	got, _ := os.ReadFile(filepath.Join(b.store.Settings().ReceiveDir, "klasör", "foto.png"))
	if !bytes.Equal(got, data) {
		t.Fatal("dosya eşleşmiyor")
	}

	// Cross-site requests and foreign Host headers are refused.
	req, _ = http.NewRequest(http.MethodDelete, "http://"+a.ui+"/api/transfers", nil)
	req.Header.Set(tokenHeader, a.uiToken)
	req.Header.Set("Origin", "http://kotu-site.example")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("yabancı Origin kabul edildi: %s", resp.Status)
	}
	req, _ = http.NewRequest(http.MethodGet, "http://"+a.ui+"/api/state", nil)
	req.Header.Set(tokenHeader, a.uiToken)
	req.Host = "evil.example"
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("yabancı Host kabul edildi: %s", resp.Status)
	}
}

func TestSanitize(t *testing.T) {
	bad := []string{"../x", "a/../../x", "/etc/passwd", "C:\\Windows\\x", "", "./"}
	for _, p := range bad {
		if _, err := sanitizeRelPath(p); err == nil {
			t.Errorf("%q kabul edilmemeliydi", p)
		}
	}
	cases := map[string]string{
		"a/b.txt":           "a/b.txt",
		"dosya:adı?.txt":    "dosya_adı_.txt",
		"CON.txt":           "_CON.txt",
		"klasör\\alt\\x.md": "klasör/alt/x.md",
		"nokta. ":           "nokta",
	}
	for in, want := range cases {
		got, err := sanitizeRelPath(in)
		if err != nil || strings.Join(got, "/") != want {
			t.Errorf("%q → %v (%v), beklenen %q", in, got, err, want)
		}
	}
}

func TestPairCodeBinding(t *testing.T) {
	na, nb := randData(32), randData(32)
	c1 := pairCode(strings.Repeat("a", 64), strings.Repeat("b", 64), na, nb)
	c2 := pairCode(strings.Repeat("a", 64), strings.Repeat("c", 64), na, nb)
	if c1 == c2 && c1 != "" {
		t.Log("çok düşük olasılıklı çakışma") // 1e-6
	}
	if len(c1) != 6 {
		t.Fatalf("kod uzunluğu %d", len(c1))
	}
}

// Another local user or program can reach 127.0.0.1 too; without the
// session secret it gets nothing. The browser signs in with a one-time code.
func TestUISession(t *testing.T) {
	a := startNode(t, "A")
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	resp, err := http.Get("http://" + a.ui + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anahtarsız istek: %s", resp.Status)
	}
	tok, err := ReadToken(a.cfg.Dir)
	if err != nil || tok != a.uiToken {
		t.Fatalf("anahtar dosyası: %v", err)
	}
	// Windows has no Unix permission bits; the file sits in the user's
	// profile, which other users cannot read.
	if st, _ := os.Stat(filepath.Join(a.cfg.Dir, tokenFile)); runtime.GOOS != "windows" && st != nil && st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("anahtar dosyası başkalarınca okunabilir: %v", st.Mode())
	}

	launch := strings.Replace(a.LaunchURL(), a.cfg.UIAddr, a.ui, 1)
	resp, err = noRedirect.Get(launch)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	if resp.StatusCode != http.StatusSeeOther || cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("giriş: %s, çerez %+v", resp.Status, cookie)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://"+a.ui+"/api/state", nil)
	req.AddCookie(cookie)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("çerezle istek: %s", resp.Status)
	}
	// The code works once.
	resp, _ = noRedirect.Get(launch)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("kod ikinci kez kabul edildi: %s", resp.Status)
	}
}

// A device whose pairing request was turned down must wait before asking
// again, so nobody can flood the screen with requests.
func TestPairingBackoff(t *testing.T) {
	a, b := startNode(t, "A"), startNode(t, "B")
	ctx := context.Background()
	if _, err := a.probe(ctx, fmt.Sprintf("127.0.0.1:%d", b.cfg.PeerPort)); err != nil {
		t.Fatal(err)
	}
	out, err := a.StartPair(ctx, b.fp)
	if err != nil {
		t.Fatal(err)
	}
	var in PairView
	waitFor(t, "istek", func() bool {
		for _, p := range b.pairViews() {
			in = p
			return true
		}
		return false
	})
	_ = a.DecidePair(out.ID, true)
	_ = b.DecidePair(in.ID, false)
	waitFor(t, "ret", func() bool { return len(a.pairViews()) == 0 })
	_, err = a.StartPair(ctx, b.fp)
	if err == nil || !strings.Contains(err.Error(), "not accepting") {
		t.Fatalf("hemen yeni istek kabul edilmemeliydi: %v", err)
	}
	// The refusal travels as a message key, so it reads in the user's language.
	if got := i18n.Text(i18n.TR, err); !strings.Contains(got, "şimdilik eşleştirme isteği kabul etmiyor") {
		t.Fatalf("karşı cihazdan gelen hata çevrilmedi: %q", got)
	}
}

func TestPairGuardBurst(t *testing.T) {
	g := newPairGuard()
	now := time.Now()
	for i := 0; i < pairBurst; i++ {
		if ok, _ := g.allow(fmt.Sprint("fp", i), fmt.Sprint("10.0.0.", i), now); !ok {
			t.Fatalf("%d. istek reddedildi", i)
		}
	}
	if ok, _ := g.allow("yeni", "10.0.0.99", now); ok {
		t.Fatal("dakikalık sınır aşıldı")
	}
	if ok, _ := g.allow("yeni", "10.0.0.99", now.Add(pairBurstEvery+time.Second)); !ok {
		t.Fatal("bir dakika sonra izin verilmeliydi")
	}
	g.penalize("x", "1.2.3.4", now)
	g.penalize("x", "1.2.3.4", now)
	if ok, wait := g.allow("baska", "1.2.3.4", now.Add(45*time.Second)); ok || wait <= 0 {
		t.Fatal("aynı IP'den yeni kimlikle gelen istek de beklemeli")
	}
}

func TestSafetyNumberSymmetric(t *testing.T) {
	a, b := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if safetyNumber(a, b) != safetyNumber(b, a) || len(safetyNumber(a, b)) != 14 {
		t.Fatal(safetyNumber(a, b))
	}
	if !isRisky("Kurulum.EXE") || isRisky("tatil.jpg") {
		t.Fatal("riskli uzantı tespiti")
	}
}

func FuzzSanitizeRelPath(f *testing.F) {
	for _, s := range []string{"a/b.txt", "../x", "C:\\x", "a/./b", "CON", "nokta. ", "x/\u202e/y"} {
		f.Add(s)
	}
	root := filepath.Join(os.TempDir(), "pasla-fuzz-root")
	f.Fuzz(func(t *testing.T, p string) {
		parts, err := sanitizeRelPath(p)
		if err != nil {
			return
		}
		full := filepath.Join(append([]string{root}, parts...)...)
		rel, err := filepath.Rel(root, full)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			t.Fatalf("%q kökten kaçtı: %q", p, full)
		}
		for _, c := range parts {
			if c == "" || c == "." || c == ".." || strings.ContainsAny(c, "/\\:*?\"<>|") {
				t.Fatalf("%q güvensiz parça üretti: %q", p, c)
			}
		}
	})
}

// The whole phone flow: key from the QR code, approval with the check number
// on the computer, then files both ways and text.
func TestPhoneFlow(t *testing.T) {
	a := startNode(t, "Masaüstü")
	v, err := a.StartPhone()
	if err != nil || !v.KeyLive || len(v.Links) == 0 || !strings.Contains(v.Links[0].QR, "<svg") {
		t.Fatalf("telefon modu: %+v %v", v, err)
	}
	key := v.Links[0].URL[strings.Index(v.Links[0].URL, "#k=")+3:]
	base := fmt.Sprintf("https://127.0.0.1:%d", a.cfg.PhonePort)
	jar, _ := cookiejar.New(nil)
	phone := &http.Client{Jar: jar, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	call := func(method, path string, body io.Reader, out any) int {
		t.Helper()
		req, _ := http.NewRequest(method, base+path, body)
		req.Header.Set("Content-Type", "application/json")
		resp, err := phone.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if out != nil {
			_ = json.NewDecoder(resp.Body).Decode(out)
		} else {
			_, _ = io.Copy(io.Discard, resp.Body)
		}
		return resp.StatusCode
	}

	if c := call("POST", "/p/login", strings.NewReader(`{"key":"yanlis","name":"x"}`), nil); c != 401 {
		t.Fatalf("yanlış anahtar: %d", c)
	}
	var login struct{ Check string }
	if c := call("POST", "/p/login", strings.NewReader(`{"key":"`+key+`","name":"Ayşe'nin telefonu","os":"android"}`), &login); c != 200 || len(login.Check) != 4 {
		t.Fatalf("giriş: %d %+v", c, login)
	}
	if c := call("POST", "/p/api/offer", strings.NewReader(`{"files":[{"name":"a.jpg","size":3}]}`), nil); c != http.StatusForbidden {
		t.Fatalf("onaysız telefon dosya gönderebildi: %d", c)
	}
	ph := a.phoneSessViews()
	if len(ph) != 1 || ph[0].Check != login.Check || ph[0].Approved {
		t.Fatalf("masaüstündeki istek: %+v", ph)
	}
	if err := a.DecidePhone(ph[0].ID, true); err != nil {
		t.Fatal(err)
	}

	photo := randData(200_000)
	var off struct{ ID string }
	if c := call("POST", "/p/api/offer", strings.NewReader(`{"files":[{"name":"../../x.jpg","size":1}]}`), nil); c != 400 {
		t.Fatalf("yol kaçışı kabul edildi: %d", c)
	}
	body, _ := json.Marshal(map[string]any{"files": []FileMeta{{Name: "IMG_0001.jpg", Size: int64(len(photo))}}})
	if c := call("POST", "/p/api/offer", bytes.NewReader(body), &off); c != 200 {
		t.Fatalf("teklif: %d", c)
	}
	if c := call("PUT", "/p/api/upload/"+off.ID+"/0", bytes.NewReader(photo), nil); c != 200 {
		t.Fatalf("yükleme: %d", c)
	}
	got, _ := os.ReadFile(filepath.Join(a.store.Settings().ReceiveDir, "IMG_0001.jpg"))
	if !bytes.Equal(got, photo) {
		t.Fatal("telefondan gelen dosya bozuk ya da yanlış yerde")
	}

	doc := []byte("rapor içeriği")
	it, err := a.AddToOutbox(ph[0].ID, "Rapor 2026.pdf", int64(len(doc)), bytes.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", base+"/p/api/outbox/"+it.ID, nil)
	resp, err := phone.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	dl, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.Equal(dl, doc) || !strings.Contains(resp.Header.Get("Content-Disposition"), "Rapor%202026.pdf") {
		t.Fatalf("indirme: %q %s", dl, resp.Header.Get("Content-Disposition"))
	}

	if c := call("POST", "/p/api/message", strings.NewReader(`{"text":"selam"}`), nil); c != 200 {
		t.Fatalf("mesaj: %d", c)
	}
	// A page on another site cannot drive the phone API with the cookie.
	req, _ = http.NewRequest("POST", base+"/p/api/message", strings.NewReader(`{"text":"x"}`))
	req.Header.Set("Origin", "https://kotu.example")
	resp, _ = phone.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("yabancı Origin: %s", resp.Status)
	}

	a.StopPhone()
	if _, err := phone.Get(base + "/p/api/state"); err == nil {
		t.Fatal("kapatıldıktan sonra port hâlâ açık")
	}
}

// Guessing the key is pointless: after a few wrong tries it is revoked.
func TestPhoneKeyRevokedAfterGuesses(t *testing.T) {
	a := startNode(t, "A")
	if _, err := a.StartPhone(); err != nil {
		t.Fatal(err)
	}
	defer a.StopPhone()
	cl := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	base := fmt.Sprintf("https://127.0.0.1:%d/p/login", a.cfg.PhonePort)
	for i := 0; i < phoneMaxBadKeys; i++ {
		resp, err := cl.Post(base, "application/json", strings.NewReader(`{"key":"tahmin"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if a.phoneView().KeyLive {
		t.Fatal("anahtar iptal edilmeliydi")
	}
}

func TestNormalizeAddr(t *testing.T) {
	for _, bad := range []string{"", "300.1.1.1", "192.168.1", "1.2.3.4:0", "1.2.3.4:99999"} {
		if _, err := normalizeAddr(bad); err == nil {
			t.Errorf("%q kabul edilmemeliydi", bad)
		}
	}
	for in, want := range map[string]string{"192.168.1.20": "192.168.1.20:47821", "10.0.0.5:5000": "10.0.0.5:5000", "ofis-pc": "ofis-pc:47821"} {
		if got, err := normalizeAddr(in); err != nil || got != want {
			t.Errorf("%q → %q (%v), beklenen %q", in, got, err, want)
		}
	}
}
