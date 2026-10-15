package node

import (
	"github.com/temurcanbulut/pasla/internal/i18n"

	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// peerHandler serves the device-to-device API on the TLS port. Every
// connection carries a client certificate; only /v1/info and pairing are
// open to devices that are not paired yet.
func (n *Node) peerHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/info", n.hInfo)
	mux.HandleFunc("POST /v1/pair/start", n.hPairStart)
	mux.HandleFunc("POST /v1/pair/nonce", n.hPairNonce)
	mux.HandleFunc("POST /v1/pair/finish", n.hPairFinish)
	mux.HandleFunc("POST /v1/pair/cancel", n.hPairCancel)

	mux.HandleFunc("POST /v1/unpair", n.trusted(n.hUnpair))
	mux.HandleFunc("POST /v1/offer", n.trusted(n.hOffer))
	mux.HandleFunc("PUT /v1/transfer/{id}/{index}", n.trusted(n.hFile))
	mux.HandleFunc("POST /v1/transfer/{id}/cancel", n.trusted(n.hRemoteCancel))
	mux.HandleFunc("POST /v1/message", n.trusted(n.hMessage))
	return mux
}

func clientFP(r *http.Request) string {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return ""
	}
	return fingerprint(r.TLS.PeerCertificates[0].Raw)
}

func (n *Node) trusted(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fp := clientFP(r)
		if fp == "" {
			jsonErr(w, r, http.StatusUnauthorized, i18n.E("no client certificate"))
			return
		}
		if _, ok := n.store.Trusted(fp); !ok {
			jsonErr(w, r, http.StatusForbidden, i18n.E("this device is not paired"))
			return
		}
		h(w, r)
	}
}

func (n *Node) hInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON200(w, infoResp{
		App: "pasla", V: protocolVersion, FP: n.fp, Name: n.store.Settings().Name, OS: osName(), Port: n.cfg.PeerPort,
	})
}

func writeJSON200(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// langHeader carries the language of the person using the interface (desktop
// or phone). Requests from other devices omit it and get English, plus the
// message key and arguments so that device can translate it for its user.
const langHeader = "X-Pasla-Lang"

func reqLang(r *http.Request) string {
	if r == nil {
		return i18n.EN
	}
	return i18n.Norm(r.Header.Get(langHeader))
}

type errBody struct {
	Error string   `json:"error"`
	Key   string   `json:"key,omitempty"`
	Args  []string `json:"args,omitempty"`
}

func jsonErr(w http.ResponseWriter, r *http.Request, status int, err error) {
	body := errBody{Error: i18n.Text(reqLang(r), err)}
	var m *i18n.Msg
	if errors.As(err, &m) {
		body.Key, body.Args = m.Format, m.StringArgs()
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func readBody(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, limit))
	if err := dec.Decode(v); err != nil {
		jsonErr(w, r, http.StatusBadRequest, i18n.E("could not read the request"))
		return false
	}
	return true
}

// LangHeader is the header clients use to ask for messages in a language.
const LangHeader = langHeader
