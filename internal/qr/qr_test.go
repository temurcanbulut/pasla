package qr

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestVersionSelection(t *testing.T) {
	cases := map[int]int{1: 1, 14: 1, 15: 2, 26: 2, 62: 4, 213: 10}
	for n, want := range cases {
		c, err := Encode([]byte(strings.Repeat("a", n)))
		if err != nil {
			t.Fatal(n, err)
		}
		if c.Version != want || c.Size != 17+4*want {
			t.Errorf("%d bayt: sürüm %d, beklenen %d", n, c.Version, want)
		}
	}
	if _, err := Encode(make([]byte, 214)); err == nil {
		t.Fatal("214 bayt reddedilmeliydi")
	}
}

func TestFinderPatterns(t *testing.T) {
	c, _ := Encode([]byte("https://192.168.1.20:47823/#k=abc"))
	n := c.Size
	for _, o := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		for i := 0; i < 7; i++ {
			if !c.Dark(o[0]+i, o[1]) || !c.Dark(o[0], o[1]+i) || !c.Dark(o[0]+3, o[1]+3) {
				t.Fatalf("bulucu desen bozuk: %v", o)
			}
		}
	}
	if !c.Dark(8, n-8) {
		t.Fatal("koyu modül eksik")
	}
}

// The matrix for a fixed input must never change by accident. This value
// was checked by decoding the code with an independent reader (OpenCV).
func TestGolden(t *testing.T) {
	c, _ := Encode([]byte("https://192.168.1.20:47823/#k=Ab9_-Ab9_-Ab9_-Ab9_-Ab"))
	var b strings.Builder
	for y := 0; y < c.Size; y++ {
		for x := 0; x < c.Size; x++ {
			if c.Dark(x, y) {
				b.WriteByte('1')
			} else {
				b.WriteByte('0')
			}
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	if got := hex.EncodeToString(sum[:8]); got != golden {
		t.Fatalf("matris değişti: %s", got)
	}
}

const golden = "2ab3e95557818846"
