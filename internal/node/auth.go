package node

import (
	"github.com/temurcanbulut/pasla/internal/i18n"

	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The local interface listens on 127.0.0.1, but on a shared computer every
// local user and program can reach that address. So the API also requires a
// session secret:
//
//   - At start-up the node creates a random token and stores it in the
//     configuration folder, readable only by the user who runs the app. The
//     command line client reads it from there.
//   - The browser never sees the token in a URL. The app opens
//     /open?c=<launch code>, a one-time code that expires in two minutes and
//     is exchanged for an HttpOnly, SameSite=Strict cookie.
//
// A second copy of the program started by the same user reads the token,
// asks the running copy for a fresh launch code and opens the browser with it.

const (
	sessionCookie  = "pasla_session"
	tokenHeader    = "X-Pasla-Token"
	tokenFile      = "ui.token"
	launchCodeLife = 2 * time.Minute
)

// DefaultDir is the configuration folder used when none is given.
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pasla"), nil
}

// ReadToken returns the interface secret of the copy running from dir.
func ReadToken(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, tokenFile))
	if err != nil {
		return "", i18n.E("could not read the session key; is Pasla running as this user?")
	}
	return strings.TrimSpace(string(b)), nil
}

func (n *Node) writeToken() error {
	n.uiToken = hex.EncodeToString(randomBytes(32))
	path := filepath.Join(n.cfg.Dir, tokenFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(n.uiToken), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LaunchURL returns a one-time address that signs the browser in.
func (n *Node) LaunchURL() string {
	code := hex.EncodeToString(randomBytes(16))
	n.mu.Lock()
	now := time.Now()
	for c, exp := range n.launchCodes {
		if now.After(exp) {
			delete(n.launchCodes, c)
		}
	}
	n.launchCodes[code] = now.Add(launchCodeLife)
	n.mu.Unlock()
	return "http://" + n.cfg.UIAddr + "/open?c=" + code
}

func (n *Node) useLaunchCode(code string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	exp, ok := n.launchCodes[code]
	delete(n.launchCodes, code)
	return ok && time.Now().Before(exp)
}

func (n *Node) authorized(r *http.Request) bool {
	got := r.Header.Get(tokenHeader)
	if got == "" {
		if c, err := r.Cookie(sessionCookie); err == nil {
			got = c.Value
		}
	}
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(n.uiToken)) == 1
}

func (n *Node) hOpen(w http.ResponseWriter, r *http.Request) {
	if !n.useLaunchCode(r.URL.Query().Get("c")) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(expiredPage))
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: n.uiToken, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (n *Node) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !n.authorized(r) {
			jsonErr(w, r, http.StatusUnauthorized, i18n.E("no session: open Pasla again"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

const expiredPage = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Pasla</title><body style="font:16px system-ui,sans-serif;max-width:34rem;margin:15vh auto;padding:0 1rem;line-height:1.5">
<h1 style="font-size:1.4rem">This link is no longer valid</h1>
<p>Links that open the interface work once and expire after two minutes.
Start Pasla again (or click its shortcut again) and the interface opens by itself.</p>
<hr style="border:0;border-top:1px solid #ccc;margin:2rem 0">
<h1 style="font-size:1.4rem" lang="tr">Bu bağlantı artık geçerli değil</h1>
<p lang="tr">Arayüzü açan bağlantılar tek kullanımlıktır ve iki dakika içinde geçersiz olur.
Pasla'yı yeniden başlatın (ya da kısayoluna yeniden tıklayın); arayüz kendiliğinden açılır.</p></body>`
