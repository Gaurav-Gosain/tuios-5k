package ui

import "strings"

// A mono-line display face, five pixels wide and seven tall with one row of
// descender, drawn with half blocks so a pixel is about square. Thin strokes
// and round corners keep a big number light on the page.

var glyphs = map[rune][]string{
	'0': {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###.", "....."},
	'1': {"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", ".###.", "....."},
	'2': {".###.", "#...#", "....#", "...#.", "..#..", ".#...", "#####", "....."},
	'3': {".###.", "#...#", "....#", "..##.", "....#", "#...#", ".###.", "....."},
	'4': {"...#.", "..##.", ".#.#.", "#..#.", "#####", "...#.", "...#.", "....."},
	'5': {"#####", "#....", "####.", "....#", "....#", "#...#", ".###.", "....."},
	'6': {"..##.", ".#...", "#....", "####.", "#...#", "#...#", ".###.", "....."},
	'7': {"#####", "....#", "...#.", "..#..", ".#...", ".#...", ".#...", "....."},
	'8': {".###.", "#...#", "#...#", ".###.", "#...#", "#...#", ".###.", "....."},
	'9': {".###.", "#...#", "#...#", ".####", "....#", "...#.", ".##..", "....."},
	',': {"..", "..", "..", "..", "..", ".#", ".#", "#."},
	' ': {"...", "...", "...", "...", "...", "...", "...", "..."},
	't': {".#...", ".#...", "####.", ".#...", ".#...", ".#..#", "..##.", "....."},
	'h': {"#....", "#....", "#.##.", "##..#", "#...#", "#...#", "#...#", "....."},
	'a': {".....", ".....", ".###.", "....#", ".####", "#...#", ".####", "....."},
	'n': {".....", ".....", "#.##.", "##..#", "#...#", "#...#", "#...#", "....."},
	'k': {"#....", "#....", "#..#.", "#.#..", "##...", "#.#..", "#..#.", "....."},
	'y': {".....", ".....", "#...#", "#...#", "#...#", ".####", "....#", ".###."},
	'o': {".....", ".....", ".###.", "#...#", "#...#", "#...#", ".###.", "....."},
	'u': {".....", ".....", "#...#", "#...#", "#...#", "#..##", ".##.#", "....."},
	'.': {".", ".", ".", ".", ".", ".", "#", "."},
}

// bigWidth is the column width of s in the display face at a scale, one
// pixel of tracking between glyphs.
func bigWidth(s string, scale int) int {
	return bigWidth1(s) * scale
}

func bigWidth1(s string) int {
	w := 0
	for i, r := range s {
		g, ok := glyphs[r]
		if !ok {
			continue
		}
		if i > 0 {
			w++
		}
		w += len(g[0])
	}
	return w
}

// bigHeight is the row count of the display face at a scale.
func bigHeight(scale int) int { return 4 * scale }

// drawBig draws s in the display face with its top-left at x, y. colour
// picks the colour of each cell from its column and row inside the text,
// with row in [0,4) whatever the scale. At scale 2 a pixel is two columns by
// one row, which is square in most fonts.
func drawBig(c *Canvas, x, y int, s string, scale int, bg Color, colour func(col, row int) Color) {
	if scale >= 2 {
		drawBig2(c, x, y, s, bg, colour)
		return
	}
	col := 0
	for i, r := range s {
		g, ok := glyphs[r]
		if !ok {
			continue
		}
		if i > 0 {
			col++
		}
		gw := len(g[0])
		for row := range 4 {
			top, bot := g[row*2], g[row*2+1]
			for k := 0; k < gw; k++ {
				ch := halfBlock(top[k] == '#', bot[k] == '#')
				if ch == ' ' {
					continue
				}
				c.Set(x+col+k, y+row, ch, Style{Fg: colour(col+k, row), Bg: bg})
			}
		}
		col += gw
	}
}

// drawBig2 draws the face at twice the size. The bitmap goes through
// Scale2x, which rounds the diagonals instead of just fattening the pixels,
// and is then drawn with half blocks like the small size.
func drawBig2(c *Canvas, x, y int, s string, bg Color, colour func(col, row int) Color) {
	col := 0
	for i, r := range s {
		g, ok := glyphs[r]
		if !ok {
			continue
		}
		if i > 0 {
			col++
		}
		gw := len(g[0])
		at := func(px, py int) bool {
			if px < 0 || py < 0 || px >= gw || py >= 8 {
				return false
			}
			return g[py][px] == '#'
		}
		big := make([][]bool, 16)
		for j := range big {
			big[j] = make([]bool, gw*2)
		}
		for py := range 8 {
			for px := range gw {
				p := at(px, py)
				a, b, cc, d := at(px, py-1), at(px+1, py), at(px-1, py), at(px, py+1)
				e0, e1, e2, e3 := p, p, p, p
				if cc == a && cc != d && a != b {
					e0 = a
				}
				if a == b && a != cc && b != d {
					e1 = b
				}
				if d == cc && d != b && cc != a {
					e2 = cc
				}
				if b == d && b != a && d != cc {
					e3 = d
				}
				big[py*2][px*2], big[py*2][px*2+1] = e0, e1
				big[py*2+1][px*2], big[py*2+1][px*2+1] = e2, e3
			}
		}
		for row := range 8 {
			for k := range gw * 2 {
				ch := halfBlock(big[row*2][k], big[row*2+1][k])
				if ch == ' ' {
					continue
				}
				c.Set(x+col*2+k, y+row, ch, Style{Fg: colour(col*2+k, row/2), Bg: bg})
			}
		}
		col += gw
	}
}

func halfBlock(top, bot bool) rune {
	switch {
	case top && bot:
		return '█'
	case top:
		return '▀'
	case bot:
		return '▄'
	}
	return ' '
}

// thousands formats n with commas.
func thousands(n int) string {
	s := strings.Builder{}
	str := itoa(n)
	for i, r := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			s.WriteByte(',')
		}
		s.WriteRune(r)
	}
	return s.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
