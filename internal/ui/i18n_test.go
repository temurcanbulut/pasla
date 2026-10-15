package ui

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every text the pages show must have a Turkish translation in i18n.js,
// with the same {placeholders}.
func TestTurkishDictionaryComplete(t *testing.T) {
	read := func(name string) string {
		b, err := fs.ReadFile(Static, name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	str := `"((?:[^"\\]|\\.)*)"`
	callT := regexp.MustCompile(`\bt\(` + str)
	callTN := regexp.MustCompile(`\btn\([^,]+,\s*` + str + `,\s*` + str)
	attr := regexp.MustCompile(`data-i18n(?:-title|-aria-label|-placeholder)?="([^"]*)"`)

	keys := map[string]bool{}
	for _, f := range []string{"app.js", "m/app.js"} {
		src := read(f)
		for _, m := range callT.FindAllStringSubmatch(src, -1) {
			keys[m[1]] = true
		}
		for _, m := range callTN.FindAllStringSubmatch(src, -1) {
			keys[m[1]], keys[m[2]] = true, true
		}
	}
	for _, f := range []string{"index.html", "m/index.html"} {
		for _, m := range attr.FindAllStringSubmatch(read(f), -1) {
			keys[m[1]] = true
		}
	}
	if len(keys) < 150 {
		t.Fatalf("only %d texts found; is the pattern still right?", len(keys))
	}

	dict := map[string]string{}
	src := read("i18n.js")
	body := src[strings.Index(src, "const TR = {"):strings.Index(src, "const DICTS")]
	entry := regexp.MustCompile(`(?s)` + str + `:\s*` + str + `,`)
	for _, m := range entry.FindAllStringSubmatch(body, -1) {
		dict[m[1]] = m[2]
	}

	var missing []string
	ph := regexp.MustCompile(`\{\w+\}`)
	for k := range keys {
		tr, ok := dict[k]
		if !ok {
			missing = append(missing, k)
			continue
		}
		a, b := ph.FindAllString(k, -1), ph.FindAllString(tr, -1)
		sort.Strings(a)
		sort.Strings(b)
		if strings.Join(a, ",") != strings.Join(b, ",") {
			t.Errorf("placeholders differ for %q: %v vs %v", k, a, b)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("no Turkish translation for:\n  %s", strings.Join(missing, "\n  "))
	}
}
