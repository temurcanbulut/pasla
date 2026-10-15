// Package qr is a small QR code encoder: byte mode, error correction level
// M, versions 1 to 10 (up to 213 bytes). That covers the short URLs the app
// shows and keeps the program free of outside dependencies.
package qr

import (
	"errors"
	"fmt"
	"strings"
)

// Code is a square matrix of modules; true is dark.
type Code struct {
	Size    int
	Version int
	mod     [][]bool
	fn      [][]bool
}

// Dark reports whether the module at column x, row y is dark.
func (c *Code) Dark(x, y int) bool { return c.mod[y][x] }

type block struct {
	ec             int // error correction codewords per block
	n1, d1, n2, d2 int // blocks and data codewords in groups 1 and 2
}

// Error correction level M, versions 1..10.
var levelM = [...]block{
	{10, 1, 16, 0, 0}, {16, 1, 28, 0, 0}, {26, 1, 44, 0, 0}, {18, 2, 32, 0, 0}, {24, 2, 43, 0, 0},
	{16, 4, 27, 0, 0}, {18, 4, 31, 0, 0}, {22, 2, 38, 2, 39}, {22, 3, 36, 2, 37}, {26, 4, 43, 1, 44},
}

var alignment = [...][]int{
	nil, {6, 18}, {6, 22}, {6, 26}, {6, 30}, {6, 34}, {6, 22, 38}, {6, 24, 42}, {6, 26, 46}, {6, 28, 50},
}

// Encode builds the smallest code that holds data.
func Encode(data []byte) (*Code, error) {
	for v := 1; v <= len(levelM); v++ {
		b := levelM[v-1]
		capBits := (b.n1*b.d1 + b.n2*b.d2) * 8
		countBits := 8
		if v >= 10 {
			countBits = 16
		}
		if 4+countBits+8*len(data) <= capBits {
			return build(v, data, countBits), nil
		}
	}
	return nil, errors.New("qr: veri çok uzun")
}

func build(v int, data []byte, countBits int) *Code {
	b := levelM[v-1]
	dataLen := b.n1*b.d1 + b.n2*b.d2

	// Bit stream: mode, length, bytes, terminator, padding.
	var bits []bool
	put := func(val, n int) {
		for i := n - 1; i >= 0; i-- {
			bits = append(bits, (val>>i)&1 == 1)
		}
	}
	put(0b0100, 4)
	put(len(data), countBits)
	for _, c := range data {
		put(int(c), 8)
	}
	for i := 0; i < 4 && len(bits) < dataLen*8; i++ {
		bits = append(bits, false)
	}
	for len(bits)%8 != 0 {
		bits = append(bits, false)
	}
	cw := make([]byte, 0, dataLen)
	for i := 0; i < len(bits); i += 8 {
		var x byte
		for j := 0; j < 8; j++ {
			if bits[i+j] {
				x |= 1 << (7 - j)
			}
		}
		cw = append(cw, x)
	}
	for pad := byte(0xEC); len(cw) < dataLen; pad ^= 0xEC ^ 0x11 {
		cw = append(cw, pad)
	}

	// Split into blocks, add Reed-Solomon codewords, interleave.
	div := rsDivisor(b.ec)
	var blocks, ecs [][]byte
	off := 0
	for g, cnt := range []int{b.n1, b.n2} {
		size := b.d1
		if g == 1 {
			size = b.d2
		}
		for i := 0; i < cnt; i++ {
			blk := cw[off : off+size]
			off += size
			blocks = append(blocks, blk)
			ecs = append(ecs, rsRemainder(blk, div))
		}
	}
	var final []byte
	for i := 0; i < b.d1 || i < b.d2; i++ {
		for _, blk := range blocks {
			if i < len(blk) {
				final = append(final, blk[i])
			}
		}
	}
	for i := 0; i < b.ec; i++ {
		for _, e := range ecs {
			final = append(final, e[i])
		}
	}

	size := 17 + 4*v
	c := &Code{Size: size, Version: v}
	c.mod = grid(size)
	c.fn = grid(size)
	c.drawFunctionPatterns()
	c.drawCodewords(final)

	best, bestScore := 0, -1
	for m := 0; m < 8; m++ {
		c.applyMask(m)
		c.drawFormat(m)
		if s := c.penalty(); bestScore < 0 || s < bestScore {
			best, bestScore = m, s
		}
		c.applyMask(m) // XOR again to undo
	}
	c.applyMask(best)
	c.drawFormat(best)
	return c
}

func grid(n int) [][]bool {
	g := make([][]bool, n)
	for i := range g {
		g[i] = make([]bool, n)
	}
	return g
}

func (c *Code) set(x, y int, dark bool) {
	c.mod[y][x] = dark
	c.fn[y][x] = true
}

func (c *Code) drawFunctionPatterns() {
	n := c.Size
	for i := 0; i < n; i++ {
		c.set(6, i, i%2 == 0)
		c.set(i, 6, i%2 == 0)
	}
	for _, p := range [][2]int{{3, 3}, {n - 4, 3}, {3, n - 4}} {
		for dy := -4; dy <= 4; dy++ {
			for dx := -4; dx <= 4; dx++ {
				x, y := p[0]+dx, p[1]+dy
				if x < 0 || y < 0 || x >= n || y >= n {
					continue
				}
				d := max(abs(dx), abs(dy))
				c.set(x, y, d != 2 && d != 4)
			}
		}
	}
	al := alignment[c.Version-1]
	for i, ax := range al {
		for j, ay := range al {
			if (i == 0 && j == 0) || (i == 0 && j == len(al)-1) || (i == len(al)-1 && j == 0) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					c.set(ax+dx, ay+dy, max(abs(dx), abs(dy)) != 1)
				}
			}
		}
	}
	c.drawFormat(0) // reserve the format areas; real bits come later
	if c.Version >= 7 {
		rem := c.Version
		for i := 0; i < 12; i++ {
			rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
		}
		bits := c.Version<<12 | rem
		for i := 0; i < 18; i++ {
			dark := (bits>>i)&1 == 1
			a, b := n-11+i%3, i/3
			c.set(a, b, dark)
			c.set(b, a, dark)
		}
	}
}

func (c *Code) drawFormat(mask int) {
	data := 0<<3 | mask // level M has format bits 00
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	bits := (data<<10 | rem) ^ 0x5412
	bit := func(i int) bool { return (bits>>i)&1 == 1 }
	n := c.Size
	for i := 0; i <= 5; i++ {
		c.set(8, i, bit(i))
	}
	c.set(8, 7, bit(6))
	c.set(8, 8, bit(7))
	c.set(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		c.set(14-i, 8, bit(i))
	}
	for i := 0; i < 8; i++ {
		c.set(n-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		c.set(8, n-15+i, bit(i))
	}
	c.set(8, n-8, true)
}

func (c *Code) drawCodewords(data []byte) {
	n := c.Size
	i := 0
	for right := n - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < n; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				y := vert
				if (right+1)&2 == 0 {
					y = n - 1 - vert
				}
				if !c.fn[y][x] && i < len(data)*8 {
					c.mod[y][x] = (data[i>>3]>>(7-i&7))&1 == 1
					i++
				}
			}
		}
	}
}

func (c *Code) applyMask(m int) {
	for y := 0; y < c.Size; y++ {
		for x := 0; x < c.Size; x++ {
			if c.fn[y][x] {
				continue
			}
			var inv bool
			switch m {
			case 0:
				inv = (x+y)%2 == 0
			case 1:
				inv = y%2 == 0
			case 2:
				inv = x%3 == 0
			case 3:
				inv = (x+y)%3 == 0
			case 4:
				inv = (x/3+y/2)%2 == 0
			case 5:
				inv = x*y%2+x*y%3 == 0
			case 6:
				inv = (x*y%2+x*y%3)%2 == 0
			case 7:
				inv = ((x+y)%2+x*y%3)%2 == 0
			}
			if inv {
				c.mod[y][x] = !c.mod[y][x]
			}
		}
	}
}

// penalty implements the four scoring rules of ISO/IEC 18004 §8.8.2.
func (c *Code) penalty() int {
	n := c.Size
	score := 0
	at := func(x, y int, rows bool) bool {
		if rows {
			return c.mod[y][x]
		}
		return c.mod[x][y]
	}
	for _, rows := range []bool{true, false} {
		for y := 0; y < n; y++ {
			run := 1
			for x := 1; x < n; x++ {
				if at(x, y, rows) == at(x-1, y, rows) {
					run++
					continue
				}
				if run >= 5 {
					score += run - 2
				}
				run = 1
			}
			if run >= 5 {
				score += run - 2
			}
			for x := 0; x+11 <= n; x++ {
				var p [11]bool
				for k := range p {
					p[k] = at(x+k, y, rows)
				}
				a := !p[0] && !p[1] && !p[2] && !p[3] && p[4] && !p[5] && p[6] && p[7] && p[8] && !p[9] && p[10]
				b := p[0] && !p[1] && p[2] && p[3] && p[4] && !p[5] && p[6] && !p[7] && !p[8] && !p[9] && !p[10]
				if a || b {
					score += 40
				}
			}
		}
	}
	dark := 0
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if c.mod[y][x] {
				dark++
			}
			if x+1 < n && y+1 < n {
				v := c.mod[y][x]
				if c.mod[y][x+1] == v && c.mod[y+1][x] == v && c.mod[y+1][x+1] == v {
					score += 3
				}
			}
		}
	}
	total := n * n
	k := 0
	for abs(dark*20-total*10) > (k+1)*total {
		k++
	}
	return score + 10*k
}

// ---- Reed-Solomon over GF(2^8), polynomial 0x11D ----------------------------

func gfMul(x, y byte) byte {
	var z int
	for i := 7; i >= 0; i-- {
		z = (z << 1) ^ ((z >> 7) * 0x11D)
		z ^= int((y>>i)&1) * int(x)
	}
	return byte(z)
}

func rsDivisor(deg int) []byte {
	res := make([]byte, deg)
	res[deg-1] = 1
	root := byte(1)
	for i := 0; i < deg; i++ {
		for j := range res {
			res[j] = gfMul(res[j], root)
			if j+1 < len(res) {
				res[j] ^= res[j+1]
			}
		}
		root = gfMul(root, 2)
	}
	return res
}

func rsRemainder(data, div []byte) []byte {
	res := make([]byte, len(div))
	for _, b := range data {
		factor := b ^ res[0]
		copy(res, res[1:])
		res[len(res)-1] = 0
		for i := range res {
			res[i] ^= gfMul(div[i], factor)
		}
	}
	return res
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// SVG renders the code with a four-module quiet zone. Dark modules use
// currentColor so the page decides the colors.
func (c *Code) SVG() string {
	q := 4
	n := c.Size + 2*q
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges" role="img" aria-label="QR kod">`, n, n)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n, n)
	for y := 0; y < c.Size; y++ {
		for x := 0; x < c.Size; x++ {
			if c.mod[y][x] {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+q, y+q)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}
