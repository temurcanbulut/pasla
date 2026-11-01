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
	recent  map[string][]time.Time
}

const (
	pairBurst      = 6
	pairBurstEvery = time.Minute
	pairMaxBlock   = 15 * time.Minute
)

func newPairGuard() *pairGuard {
	return &pairGuard{until: map[string]time.Time{}, strikes: map[string]int{}, recent: map[string][]time.Time{}}
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
	// Expire source buckets and penalty records to cap attacker-created state.
	for k, times := range g.recent {
		if len(times) == 0 || now.Sub(times[len(times)-1]) >= pairBurstEvery {
			delete(g.recent, k)
		}
	}
	for k, until := range g.until {
		if now.Sub(until) > pairMaxBlock {
			delete(g.until, k)
			delete(g.strikes, k)
		}
	}
	if _, ok := g.recent[ip]; !ok && len(g.recent) >= 2048 {
		return false, pairBurstEvery
	}
	keep := g.recent[ip][:0]
	for _, t := range g.recent[ip] {
		if now.Sub(t) < pairBurstEvery {
			keep = append(keep, t)
		}
	}
	g.recent[ip] = keep
	if len(keep) >= pairBurst {
		return false, pairBurstEvery - now.Sub(keep[0])
	}
	g.recent[ip] = append(keep, now)
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
