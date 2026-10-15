package node

import (
	"sync"
	"time"
)

// pairGuard slows down anyone who keeps opening pairing requests: a device
// (by fingerprint) or an address (by IP) whose request was turned down or
// left to expire waits 30 s, then 1, 2, 4… minutes (at most 15), and the
// whole device accepts at most six new requests a minute. Fingerprints are
// free to mint, which is why the IP address is counted too.
type pairGuard struct {
	mu      sync.Mutex
	until   map[string]time.Time
	strikes map[string]int
	recent  []time.Time
}

const (
	pairBurst      = 6
	pairBurstEvery = time.Minute
	pairMaxBlock   = 15 * time.Minute
)

func newPairGuard() *pairGuard {
	return &pairGuard{until: map[string]time.Time{}, strikes: map[string]int{}}
}

// allow reports whether a new request may start, and if not, how long to wait.
func (g *pairGuard) allow(fp, ip string, now time.Time) (bool, time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, k := range []string{"fp:" + fp, "ip:" + ip} {
		if t, ok := g.until[k]; ok && now.Before(t) {
			return false, t.Sub(now)
		}
	}
	keep := g.recent[:0]
	for _, t := range g.recent {
		if now.Sub(t) < pairBurstEvery {
			keep = append(keep, t)
		}
	}
	g.recent = keep
	if len(g.recent) >= pairBurst {
		return false, pairBurstEvery - now.Sub(g.recent[0])
	}
	g.recent = append(g.recent, now)
	return true, 0
}

// penalize records a failed attempt.
func (g *pairGuard) penalize(fp, ip string, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, k := range []string{"fp:" + fp, "ip:" + ip} {
		g.strikes[k]++
		d := 30 * time.Second << (g.strikes[k] - 1)
		if d > pairMaxBlock || d <= 0 {
			d = pairMaxBlock
		}
		g.until[k] = now.Add(d)
	}
}

// forgive clears the record after a successful pairing.
func (g *pairGuard) forgive(fp, ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, k := range []string{"fp:" + fp, "ip:" + ip} {
		delete(g.strikes, k)
		delete(g.until, k)
	}
}
