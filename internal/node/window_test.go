package node

import (
	"io"
	"log"
	"testing"
	"time"
)

func quitting(n *Node) bool {
	select {
	case <-n.quit:
		return true
	default:
		return false
	}
}

func TestWindowCloseQuits(t *testing.T) {
	n := &Node{store: &Store{}, transfers: map[string]*Transfer{}, quit: make(chan struct{}), log: discardLogger()}
	now := time.Now()

	n.checkWindow(now.Add(time.Hour))
	if quitting(n) {
		t.Fatal("quit before any window was ever opened")
	}

	closeA := n.windowOpened()
	closeB := n.windowOpened() // a second tab
	closeA()
	n.checkWindow(time.Now().Add(time.Hour))
	if quitting(n) {
		t.Fatal("quit while a tab is still open")
	}

	closeB()
	n.checkWindow(time.Now().Add(windowGrace / 2))
	if quitting(n) {
		t.Fatal("quit inside the grace period (a reload would kill Pasla)")
	}

	// A running transfer holds the quit back until it ends.
	tr := &Transfer{state: "active"}
	n.transfers["x"] = tr
	n.checkWindow(time.Now().Add(time.Hour))
	if quitting(n) {
		t.Fatal("quit during a transfer")
	}
	tr.mu.Lock()
	tr.state = "done"
	tr.mu.Unlock()
	n.checkWindow(time.Now().Add(time.Hour))
	if !quitting(n) {
		t.Fatal("did not quit after the window closed")
	}
}

func TestWindowCloseKeepRunning(t *testing.T) {
	var told int
	n := &Node{store: &Store{settings: Settings{KeepRunning: true}}, transfers: map[string]*Transfer{},
		quit: make(chan struct{}), log: discardLogger(), cfg: Config{Notify: func(string, string) { told++ }}}
	n.windowOpened()()
	for i := 0; i < 3; i++ {
		n.checkWindow(time.Now().Add(time.Hour))
	}
	n.windowOpened()()
	n.checkWindow(time.Now().Add(time.Hour))
	if quitting(n) {
		t.Fatal("quit although Keep running is on")
	}
	if told != 1 {
		t.Fatalf("background notice shown %d times, want once", told)
	}
}

func discardLogger() *log.Logger { return log.New(io.Discard, "", 0) }
