package node

import (
	"github.com/temurcanbulut/pasla/internal/i18n"

	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// sanitizeRelPath turns a sender-supplied relative path ("Tatil/IMG_01.jpg")
// into components that are safe on both Windows and Linux. It rejects
// anything that could escape the receive folder.
func sanitizeRelPath(p string) ([]string, error) {
	p = strings.ReplaceAll(p, "\\", "/")
	if strings.HasPrefix(p, "/") || (len(p) >= 2 && p[1] == ':') {
		return nil, i18n.E("absolute paths are not allowed")
	}
	raw := strings.Split(p, "/")
	if len(raw) > 64 {
		return nil, i18n.E("folders are nested too deeply")
	}
	out := make([]string, 0, len(raw))
	for _, c := range raw {
		c = sanitizeComponent(c)
		if c == "" {
			continue
		}
		if c == "." || c == ".." {
			return nil, i18n.E("invalid path")
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, i18n.E("file name is empty")
	}
	return out, nil
}

var winReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"CONIN$": true, "CONOUT$": true, "COM¹": true, "COM²": true, "COM³": true, "LPT¹": true, "LPT²": true, "LPT³": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

func sanitizeComponent(c string) string {
	if c == "." || c == ".." {
		return c
	}
	var b strings.Builder
	for _, r := range c {
		switch {
		case r < 32 || r == 127 || unicode.Is(unicode.Cf, r):
			continue
		case strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	s := strings.TrimRight(strings.TrimSpace(b.String()), ". ")
	if s == "" {
		return ""
	}
	base := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	if winReserved[base] {
		s = "_" + s
	}
	if len(s) > 200 {
		ext := filepath.Ext(s)
		if len(ext) > 20 {
			ext = ""
		}
		s = truncateUTF8(s, 200-len(ext)) + ext
	}
	return s
}

func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utfStart(s[n]) {
		n--
	}
	return s[:n]
}

func utfStart(b byte) bool { return b&0xC0 != 0x80 }

// uniqueName returns name, or "name (2).ext", "name (3).ext"... so that
// nothing already in dir is overwritten. taken holds names reserved by the
// current transfer that do not exist on disk yet.
func uniqueName(dir, name string, taken map[string]bool) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if ext == name { // ".bashrc"
		stem, ext = name, ""
	}
	cand := name
	for i := 2; ; i++ {
		key := strings.ToLower(filepath.Join(dir, cand))
		if !taken[key] {
			if _, err := os.Lstat(filepath.Join(dir, cand)); errors.Is(err, os.ErrNotExist) {
				taken[key] = true
				return cand
			}
		}
		cand = fmt.Sprintf("%s (%d)%s", stem, i, ext)
	}
}

// planDestinations maps every incoming relative path to a final location.
// Top-level items are renamed as a whole when they clash, so a folder
// "Fotoğraflar" arriving twice becomes "Fotoğraflar (2)" with its contents
// intact instead of mixing into the old one.
func planDestinations(root string, rel [][]string) []string {
	return planDestinationsTaken(root, rel, map[string]bool{})
}

func planDestinationsTaken(root string, rel [][]string, taken map[string]bool) []string {
	top := map[string]string{}
	out := make([]string, len(rel))
	used := map[string]bool{}
	for i, parts := range rel {
		key := strings.ToLower(parts[0])
		if len(parts) == 1 {
			key = "\x00file\x00" + key
		}
		t, ok := top[key]
		if !ok {
			t = uniqueName(root, parts[0], taken)
			top[key] = t
		}
		full := filepath.Join(append([]string{root, t}, parts[1:]...)...)
		// Same path twice inside one transfer: keep both.
		if used[strings.ToLower(full)] {
			dir, name := filepath.Split(full)
			full = filepath.Join(dir, uniqueName(dir, name, taken))
		}
		used[strings.ToLower(full)] = true
		out[i] = full
	}
	return out
}

func mkdirAll(dir string) error { return os.MkdirAll(dir, 0o755) }

// Reserve complete top-level destinations across all in-flight batches, before
// any file exists on disk. File creation separately remains exclusive/no-clobber.
func (n *Node) reserveDestinations(t *Transfer, root string, rel [][]string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	taken := map[string]bool{}
	for _, other := range n.transfers {
		other.mu.Lock()
		if other.Dir == "in" && !terminal(other.state) {
			for _, dest := range other.dest {
				rp, err := filepath.Rel(root, dest)
				if err == nil && rp != ".." && !strings.HasPrefix(rp, ".."+string(filepath.Separator)) {
					top := strings.SplitN(rp, string(filepath.Separator), 2)[0]
					taken[strings.ToLower(filepath.Join(root, top))] = true
				}
			}
		}
		other.mu.Unlock()
	}
	dest := planDestinationsTaken(root, rel, taken)
	t.mu.Lock()
	t.dest = dest
	t.reveal = revealTarget(root, dest)
	t.mu.Unlock()
}
