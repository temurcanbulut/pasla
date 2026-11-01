package node

import (
	"github.com/temurcanbulut/pasla/internal/i18n"

	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var errWrongDevice = i18n.E("the other device's identity does not match the expected one")

// client returns an HTTP client that presents this device's certificate and
// accepts the server only if its certificate fingerprint equals expect.
// With expect == "" any certificate is accepted; that mode is used solely
// to read /v1/info and to run pairing, which authenticates by itself.
func (n *Node) client(expect string) *http.Client {
	n.clientsMu.Lock()
	defer n.clientsMu.Unlock()
	if c, ok := n.clients[expect]; ok {
		return c
	}
	tr := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{n.cert},
			MinVersion:         tls.VersionTLS13,
			InsecureSkipVerify: true, // replaced by fingerprint pinning below
			VerifyConnection: func(cs tls.ConnectionState) error {
				if len(cs.PeerCertificates) == 0 {
					return errWrongDevice
				}
				if expect != "" && fingerprint(cs.PeerCertificates[0].Raw) != expect {
					return errWrongDevice
				}
				return nil
			},
		},
		TLSHandshakeTimeout: 8 * time.Second,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		WriteBufferSize:     256 << 10,
		ReadBufferSize:      256 << 10,
	}
	c := &http.Client{Transport: tr}
	n.clients[expect] = c
	return c
}

func (n *Node) dropClient(fp string) {
	n.clientsMu.Lock()
	if c, ok := n.clients[fp]; ok {
		c.CloseIdleConnections()
		delete(n.clients, fp)
	}
	n.clientsMu.Unlock()
}

type apiError struct {
	Status int
	Msg    string
	msg    *i18n.Msg // set when the other device sent a translatable message
}

func (e *apiError) Error() string { return e.Msg }

// Unwrap lets i18n.Text translate an error that came from another device.
func (e *apiError) Unwrap() error {
	if e.msg == nil {
		return nil
	}
	return e.msg
}

// callPeer sends a JSON request to a device. When fp is non-empty the
// connection is pinned to that fingerprint; addr may override discovery.
func (n *Node) callPeer(ctx context.Context, fp, addr, method, path string, in, out any) error {
	if addr == "" {
		var ok bool
		if addr, ok = n.addrOf(fp); !ok {
			return i18n.E("the device is not visible on the network right now")
		}
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://"+addr+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client(fp).Do(req)
	if err != nil {
		return friendlyNetErr(err)
	}
	defer resp.Body.Close()
	return decodeResp(resp, out)
}

func decodeResp(resp *http.Response, out any) error {
	if resp.StatusCode >= 300 {
		var e errBody
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&e)
		if e.Error == "" {
			e.Error = resp.Status
		}
		ae := &apiError{Status: resp.StatusCode, Msg: e.Error}
		if e.Key != "" {
			ae.msg = i18n.FromWire(e.Key, e.Args)
		}
		return ae
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}

func friendlyNetErr(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return i18n.E("cancelled")
	case errors.Is(err, errWrongDevice):
		return errWrongDevice
	case strings.Contains(err.Error(), "connection refused"):
		return i18n.E("the device refused the connection (Pasla is closed or a firewall is blocking it)")
	case strings.Contains(err.Error(), "timeout"), strings.Contains(err.Error(), "deadline"):
		return i18n.E("the device did not answer (timed out)")
	case strings.Contains(err.Error(), "no such host"):
		return i18n.E("this address could not be found on the network")
	case strings.Contains(err.Error(), "network is unreachable"), strings.Contains(err.Error(), "no route to host"):
		return i18n.E("this address cannot be reached from this computer")
	}
	// Drop the "Get https://…" wrapper; the people reading this need the cause.
	var ue *url.Error
	if errors.As(err, &ue) {
		return i18n.E("could not connect: %v", ue.Err)
	}
	return err
}

type infoResp struct {
	App  string `json:"app"`
	V    int    `json:"v"`
	FP   string `json:"fp"`
	Name string `json:"name"`
	OS   string `json:"os"`
	Port int    `json:"port"`
}

// normalizeAddr accepts "192.168.1.20" or "192.168.1.20:47821".
func normalizeAddr(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", i18n.E("address is empty")
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		host, port = s, strconv.Itoa(DefaultPeerPort)
	}
	if host == "" || (looksNumeric(host) && net.ParseIP(host) == nil) {
		return "", i18n.E("address is not valid")
	}
	if p, err := strconv.Atoi(port); err != nil || p <= 0 || p > 65535 {
		return "", i18n.E("port is not valid")
	}
	return net.JoinHostPort(host, port), nil
}

// looksNumeric spots typos like 300.1.1.1 or 192.168.1 before a DNS lookup.
func looksNumeric(h string) bool {
	return strings.Trim(h, "0123456789.") == ""
}

// probe asks a device for its identity directly (no broadcast needed).
func (n *Node) probe(ctx context.Context, addr string) (*infoResp, error) {
	return n.probeAs(ctx, addr, true)
}

func (n *Node) probeAs(ctx context.Context, addr string, manual bool) (*infoResp, error) {
	return n.probeExpected(ctx, addr, manual, "")
}

// probeExpected binds both the TLS certificate and /v1/info to a discovery claim.
func (n *Node) probeExpected(ctx context.Context, addr string, manual bool, expected string) (*infoResp, error) {
	addr, err := normalizeAddr(addr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+addr+"/v1/info", nil)
	client := n.client(expected)
	if expected != "" {
		defer n.dropClient(expected)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, friendlyNetErr(err)
	}
	defer resp.Body.Close()
	var info infoResp
	if err := decodeResp(resp, &info); err != nil {
		return nil, err
	}
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 ||
		fingerprint(resp.TLS.PeerCertificates[0].Raw) != info.FP || (expected != "" && info.FP != expected) || info.App != "pasla" || info.V != protocolVersion {
		return nil, i18n.E("%s does not look like a Pasla device", addr)
	}
	if info.FP == n.fp {
		return nil, i18n.E("this address is this device itself")
	}
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	n.seePeer(Peer{FP: info.FP, Name: clip(info.Name, 64), OS: clip(info.OS, 16), Host: host, Port: port, Manual: manual})
	return &info, nil
}
