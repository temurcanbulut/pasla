package node

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Discovery: every device broadcasts a small JSON hello on UDP every few
// seconds to each local IPv4 broadcast address. A device that hears a new
// hello answers with a unicast reply so newcomers appear immediately.
// Hellos carry no secrets; whatever they claim is verified later by TLS,
// where the certificate fingerprint must match.

type hello struct {
	App   string `json:"app"`
	V     int    `json:"v"`
	FP    string `json:"fp"`
	Name  string `json:"name"`
	OS    string `json:"os"`
	Port  int    `json:"port"`
	Reply bool   `json:"reply,omitempty"`
	Bye   bool   `json:"bye,omitempty"`
}

func (n *Node) hello(reply, bye bool) []byte {
	b, _ := json.Marshal(hello{
		App: "pasla", V: protocolVersion, FP: n.fp, Name: n.store.Settings().Name,
		OS: osName(), Port: n.cfg.PeerPort, Reply: reply, Bye: bye,
	})
	return b
}

func (n *Node) startDiscovery(ctx context.Context) error {
	lc := net.ListenConfig{Control: reuseAddr}
	pc, err := lc.ListenPacket(ctx, "udp4", fmt.Sprintf(":%d", n.cfg.DiscoveryPort))
	if err != nil {
		return err
	}
	conn := pc.(*net.UDPConn)
	n.udp = conn
	go func() {
		<-ctx.Done()
		time.Sleep(300 * time.Millisecond) // let the goodbye go out first
		conn.Close()
	}()
	go n.readHellos(conn)
	go func() {
		t := time.NewTicker(3 * time.Second)
		defer t.Stop()
		n.broadcast(n.hello(false, false))
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				n.broadcast(n.hello(false, false))
			}
		}
	}()
	return nil
}

func (n *Node) readHellos(conn *net.UDPConn) {
	buf := make([]byte, 2048)
	for {
		nr, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return
			}
			continue
		}
		var h hello
		if json.Unmarshal(buf[:nr], &h) != nil || h.App != "pasla" || h.FP == n.fp || len(h.FP) != 64 {
			continue
		}
		if h.Port <= 0 || h.Port > 65535 {
			continue
		}
		// UDP is an unauthenticated hint. Ignore goodbye and claimed display names.
		// A successful pinned TLS probe is the only way to refresh a peer.
		if h.Bye || h.V != protocolVersion {
			continue
		}
		if _, err := hex.DecodeString(h.FP); err != nil {
			continue
		}
		endpoint := net.JoinHostPort(src.IP.String(), strconv.Itoa(h.Port))
		if !n.beginDiscoveryProbe(endpoint, time.Now()) {
			continue
		}
		replyAddr := *src
		go func(h hello, endpoint string) {
			defer n.endDiscoveryProbe()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if _, err := n.probeExpected(ctx, endpoint, false, h.FP); err == nil && !h.Reply {
				_, _ = conn.WriteToUDP(n.hello(true, false), &replyAddr)
			}
		}(h, endpoint)
	}
}

func (n *Node) broadcast(msg []byte) {
	if n.udp == nil {
		return
	}
	for _, addr := range broadcastAddrs() {
		_, _ = n.udp.WriteToUDP(msg, &net.UDPAddr{IP: addr, Port: n.cfg.DiscoveryPort})
	}
}

func (n *Node) sayGoodbye() { n.broadcast(n.hello(false, true)) }

func broadcastAddrs() []net.IP {
	seen := map[string]bool{}
	out := []net.IP{}
	add := func(ip net.IP) {
		if !seen[ip.String()] {
			seen[ip.String()] = true
			out = append(out, ip)
		}
	}
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || ifc.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipn.IP.To4()
			if ip4 == nil || len(ipn.Mask) != net.IPv4len {
				continue
			}
			b := make(net.IP, 4)
			for i := range b {
				b[i] = ip4[i] | ^ipn.Mask[i]
			}
			add(b)
		}
	}
	add(net.IPv4bcast)
	return out
}

// manualProbeLoop keeps devices added by IP visible even where broadcast
// traffic does not reach (other subnets, filtered switches, VPNs).
func (n *Node) manualProbeLoop(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		for _, addr := range n.store.Settings().ManualPeers {
			go n.probe(ctx, addr)
		}
		// Paired devices that discovery has not heard from are asked at
		// their last known address; this keeps them usable across subnets.
		for _, tp := range n.store.TrustedList() {
			n.mu.Lock()
			_, visible := n.peers[tp.FP]
			n.mu.Unlock()
			if !visible && tp.LastAddr != "" {
				go n.probeAs(ctx, tp.LastAddr, false)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func clip(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > max {
		r = r[:max]
	}
	return string(r)
}

// Bound UDP-triggered network work and memory, including forged fingerprints.
func (n *Node) beginDiscoveryProbe(endpoint string, now time.Time) bool {
	n.discoveryMu.Lock()
	defer n.discoveryMu.Unlock()
	if n.discoveryRecent == nil {
		n.discoveryRecent = map[string]time.Time{}
	}
	for k, at := range n.discoveryRecent {
		if now.Sub(at) > 12*time.Second {
			delete(n.discoveryRecent, k)
		}
	}
	if at, ok := n.discoveryRecent[endpoint]; ok && now.Sub(at) < 2*time.Second {
		return false
	}
	if n.discoveryActive >= 4 || len(n.discoveryRecent) >= 256 {
		return false
	}
	n.discoveryRecent[endpoint] = now
	n.discoveryActive++
	return true
}
func (n *Node) endDiscoveryProbe() { n.discoveryMu.Lock(); n.discoveryActive--; n.discoveryMu.Unlock() }
