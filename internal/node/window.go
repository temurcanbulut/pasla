package node

import (
	"sync"
	"time"

	"github.com/temurcanbulut/pasla/internal/i18n"
)

// Closing the browser window is how most people expect to close a program.
// Pasla has no window of its own, so it watches the interface's event
// stream instead: when the last open page has been gone for a little while
// (long enough to survive a reload), it quits. Running transfers finish
// first. People who want to keep receiving files with the window closed turn
// on Settings → "Keep running after I close the window".

// windowGrace is how long Pasla waits after the last page disappears.
var windowGrace = 12 * time.Second

type windowWatch struct {
	mu       sync.Mutex
	open     int       // pages currently connected
	seen     bool      // a page has connected at least once
	goneAt   time.Time // when the last page disconnected
	handled  bool      // this closing has been dealt with
	toldBg   bool      // "still running in the background" was shown
	toldWait bool      // "will close after the transfers" was shown
}

// windowOpened records a connected page and returns the function that
// records its disconnection.
func (n *Node) windowOpened() func() {
	w := &n.win
	w.mu.Lock()
	w.open++
	w.seen = true
	w.handled, w.toldWait = false, false
	w.mu.Unlock()
	return func() {
		w.mu.Lock()
		w.open--
		if w.open == 0 {
			w.goneAt = time.Now()
		}
		w.mu.Unlock()
	}
}

// checkWindow runs on every housekeeping tick.
func (n *Node) checkWindow(now time.Time) {
	w := &n.win
	w.mu.Lock()
	if !w.seen || w.open > 0 || w.handled || now.Sub(w.goneAt) < windowGrace {
		w.mu.Unlock()
		return
	}
	if n.store.Settings().KeepRunning {
		w.handled = true
		tell := !w.toldBg
		w.toldBg = true
		w.mu.Unlock()
		if tell {
			n.notify(i18n.T(n.lang(), "Pasla is still running"),
				i18n.T(n.lang(), "It keeps running so other devices can send you files. To quit, right-click the Pasla icon next to the clock."))
		}
		return
	}
	if n.transfersRunning() {
		tell := !w.toldWait
		w.toldWait = true
		w.mu.Unlock()
		if tell {
			n.notify(i18n.T(n.lang(), "Pasla is finishing the transfers"),
				i18n.T(n.lang(), "It will close by itself when they are done."))
		}
		return
	}
	w.handled = true
	w.mu.Unlock()
	n.log.Print(i18n.T(n.lang(), "The Pasla window was closed, so Pasla is quitting."))
	n.Quit()
}

// transfersRunning reports whether any transfer is waiting or under way.
func (n *Node) transfersRunning() bool {
	n.mu.Lock()
	list := make([]*Transfer, 0, len(n.transfers))
	for _, t := range n.transfers {
		list = append(list, t)
	}
	n.mu.Unlock()
	for _, t := range list {
		if !terminal(t.State()) {
			return true
		}
	}
	return false
}

func (n *Node) notify(title, text string) {
	n.log.Print(title + ". " + text)
	if n.cfg.Notify != nil {
		n.cfg.Notify(title, text)
	}
}
