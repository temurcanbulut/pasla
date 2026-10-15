package node

import (
	"github.com/temurcanbulut/pasla/internal/i18n"

	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Settings are the user-editable preferences of this device.
type Settings struct {
	Name        string   `json:"name"`
	ReceiveDir  string   `json:"receiveDir"`
	AutoAccept  bool     `json:"autoAccept"` // accept from every paired device without asking
	ManualPeers []string `json:"manualPeers"`
	Lang        string   `json:"lang,omitempty"` // "en" (default) or "tr"
	// KeepRunning keeps Pasla running after its browser window is closed,
	// so other devices can still send files. Off: closing the window quits.
	KeepRunning bool `json:"keepRunning,omitempty"`
}

// TrustedPeer is a device this one has paired with.
type TrustedPeer struct {
	FP         string    `json:"fp"`
	Name       string    `json:"name"`
	OS         string    `json:"os"`
	PairedAt   time.Time `json:"pairedAt"`
	AutoAccept bool      `json:"autoAccept"`
	LastAddr   string    `json:"lastAddr"` // host:port, a hint when discovery is silent
}

// Store persists settings and the trust list as small JSON files.
type Store struct {
	mu       sync.Mutex
	dir      string
	settings Settings
	peers    map[string]*TrustedPeer
}

func openStore(dir string, defaults Settings) (*Store, error) {
	s := &Store{dir: dir, settings: defaults, peers: map[string]*TrustedPeer{}}
	if err := readJSON(filepath.Join(dir, "settings.json"), &s.settings); err != nil {
		return nil, err
	}
	var list []*TrustedPeer
	if err := readJSON(filepath.Join(dir, "trusted.json"), &list); err != nil {
		return nil, err
	}
	for _, p := range list {
		if p != nil && len(p.FP) == 64 {
			s.peers[p.FP] = p
		}
	}
	if s.settings.Name == "" {
		s.settings.Name = defaults.Name
	}
	if s.settings.ReceiveDir == "" {
		s.settings.ReceiveDir = defaults.ReceiveDir
	}
	return s, s.saveLocked()
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(s.dir, "settings.json"), s.settings); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.dir, "trusted.json"), s.trustedLocked())
}

func (s *Store) trustedLocked() []TrustedPeer {
	out := make([]TrustedPeer, 0, len(s.peers))
	for _, p := range s.peers {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PairedAt.Before(out[j].PairedAt) })
	return out
}

func (s *Store) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := s.settings
	cp.ManualPeers = append([]string(nil), s.settings.ManualPeers...)
	return cp
}

func (s *Store) UpdateSettings(fn func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.settings)
	return s.saveLocked()
}

func (s *Store) Trusted(fp string) (TrustedPeer, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.peers[fp]
	if !ok {
		return TrustedPeer{}, false
	}
	return *p, true
}

func (s *Store) TrustedList() []TrustedPeer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trustedLocked()
}

func (s *Store) Trust(p TrustedPeer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.peers[p.FP]; ok {
		p.AutoAccept = old.AutoAccept
	}
	s.peers[p.FP] = &p
	return s.saveLocked()
}

func (s *Store) Untrust(fp string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.peers, fp)
	return s.saveLocked()
}

// UpdateTrusted changes a paired device in place; it is a no-op for strangers.
func (s *Store) UpdateTrusted(fp string, fn func(*TrustedPeer)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.peers[fp]
	if !ok {
		return nil
	}
	before := *p
	fn(p)
	if *p == before {
		return nil
	}
	return s.saveLocked()
}

// ReadLang returns the interface language saved in dir ("en" when unset),
// without opening the store. The command line uses it before a node exists.
func ReadLang(dir string) string {
	var s Settings
	_ = readJSON(filepath.Join(dir, "settings.json"), &s)
	return i18n.Norm(s.Lang)
}
