package i18n

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every message the Go code can show must have a Turkish translation, and
// the translation must use the same number of arguments.
func TestCatalogueComplete(t *testing.T) {
	re := regexp.MustCompile(`(?:i18n\.(?:E|T)\((?:[a-zA-Z.()]+, )?|\bt\()"((?:[^"\\]|\\.)*)"`)
	var missing []string
	seen := map[string]bool{}
	for _, dir := range []string{"../node", "../../cmd/pasla"} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range re.FindAllStringSubmatch(string(src), -1) {
				key := strings.ReplaceAll(m[1], `\"`, `"`)
				if seen[key] {
					continue
				}
				seen[key] = true
				if !Has(TR, key) {
					missing = append(missing, key)
				}
			}
		}
	}
	if len(seen) < 50 {
		t.Fatalf("only %d messages found; is the pattern still right?", len(seen))
	}
	if len(missing) > 0 {
		t.Fatalf("no Turkish translation for:\n  %s", strings.Join(missing, "\n  "))
	}
	verbs := regexp.MustCompile(`%(\[\d+\])?[sv]`)
	for en, trText := range tr {
		if a, b := len(verbs.FindAllString(en, -1)), len(verbs.FindAllString(trText, -1)); a != b {
			t.Errorf("%q: %d arguments in English, %d in Turkish", en, a, b)
		}
		if strings.Contains(en, "%d") || strings.Contains(en, "%q") {
			t.Errorf("%q: use %%s or %%v so arguments survive the wire", en)
		}
	}
}

func TestTranslate(t *testing.T) {
	m := E("invalid file: %s", "a.txt")
	if m.Error() != "invalid file: a.txt" || m.In(TR) != "geçersiz dosya: a.txt" {
		t.Fatal(m.Error(), m.In(TR))
	}
	// Nested messages are translated too; plain errors pass through.
	outer := E("could not save the file: %v", E("disk error: %v", errors.New("EIO")))
	if got := Text(TR, outer); got != "dosya kaydedilemedi: disk hatası: EIO" {
		t.Fatal(got)
	}
	// A message rebuilt from the wire translates like the original.
	w := FromWire(m.Format, m.StringArgs())
	if w.In(TR) != m.In(TR) {
		t.Fatal(w.In(TR))
	}
	// Errors wrapping a message are found with errors.As.
	if got := Text(TR, fmt.Errorf("ctx: %w", m)); got != "geçersiz dosya: a.txt" {
		t.Fatal(got)
	}
	if T(TR, "Sending %v file(s) (%s) to %s…", 3, "1 MB", "Ofis") != "Ofis cihazına 3 dosya (1 MB) gönderiliyor…" {
		t.Fatal(T(TR, "Sending %v file(s) (%s) to %s…", 3, "1 MB", "Ofis"))
	}
	if Norm("tr-TR") != TR || Norm("de") != EN || Norm("") != EN {
		t.Fatal("Norm")
	}
}
