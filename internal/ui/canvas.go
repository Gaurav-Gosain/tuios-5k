package ui

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Color is an RGB colour. The zero value means "keep what is there".
type Color struct {
	R, G, B uint8
	Set     bool
}

// Hex parses #rrggbb.
func Hex(s string) Color {
	s = strings.TrimPrefix(s, "#")
	v, _ := strconv.ParseUint(s, 16, 32)
	return Color{uint8(v >> 16), uint8(v >> 8), uint8(v), true}
}

// Mix blends from a toward b by t in [0,1], in sRGB space.
func Mix(a, b Color, t float64) Color {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	f := func(x, y uint8) uint8 {
		return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5)
	}
	return Color{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), true}
}

// luma is a cheap brightness used to decide which of two dots in one cell
// wins the cell's colour.
func (c Color) luma() int { return int(c.R)*3 + int(c.G)*6 + int(c.B) }

const (
	attrBold = 1 << iota
	attrItalic
	attrUnderline
)

// Cell is one terminal cell. Every glyph this program draws is one column
// wide, so there is no wide-character bookkeeping.
type Cell struct {
	R    rune
	Fg   Color
	Bg   Color
	Attr uint8
}

// Canvas is a grid of cells that renders to one ANSI string per frame.
type Canvas struct {
	W, H  int
	cells []Cell
	clip  rect
}

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && y >= r.y && x < r.x+r.w && y < r.y+r.h
}

// NewCanvas returns a canvas filled with spaces on bg.
func NewCanvas(w, h int, bg Color) *Canvas {
	c := &Canvas{W: w, H: h, cells: make([]Cell, w*h)}
	for i := range c.cells {
		c.cells[i] = Cell{R: ' ', Bg: bg}
	}
	c.clip = rect{0, 0, w, h}
	return c
}

// Clip limits drawing to a rectangle until Unclip.
func (c *Canvas) Clip(x, y, w, h int) { c.clip = rect{x, y, w, h} }

// Unclip lets drawing reach the whole canvas again.
func (c *Canvas) Unclip() { c.clip = rect{0, 0, c.W, c.H} }

// At returns the cell at x, y, or nil outside the canvas or the clip.
func (c *Canvas) At(x, y int) *Cell {
	if x < 0 || y < 0 || x >= c.W || y >= c.H || !c.clip.contains(x, y) {
		return nil
	}
	return &c.cells[y*c.W+x]
}

// Style is how text is drawn. A colour left unset keeps the cell's colour.
type Style struct {
	Fg, Bg Color
	Attr   uint8
}

// Set writes one rune.
func (c *Canvas) Set(x, y int, r rune, st Style) {
	cell := c.At(x, y)
	if cell == nil {
		return
	}
	cell.R = r
	if st.Fg.Set {
		cell.Fg = st.Fg
	}
	if st.Bg.Set {
		cell.Bg = st.Bg
	}
	cell.Attr = st.Attr
}

// Text writes s from x, y and returns the column after it.
func (c *Canvas) Text(x, y int, s string, st Style) int {
	for _, r := range s {
		c.Set(x, y, r, st)
		x++
	}
	return x
}

// TextWidth is the column count of s. Every rune here is one column.
func TextWidth(s string) int { return utf8.RuneCountInString(s) }

// Fill paints a rectangle with spaces on bg.
func (c *Canvas) Fill(x, y, w, h int, bg Color) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			if cell := c.At(xx, yy); cell != nil {
				*cell = Cell{R: ' ', Bg: bg}
			}
		}
	}
}

// Box draws a rounded border around a filled rectangle. A title sits in the
// top edge, after one rule. The border cells keep the background under them,
// so the line sits on the sky and the fill starts inside it.
func (c *Canvas) Box(x, y, w, h int, border, fill Color, title string, titleSt Style) {
	c.Fill(x+1, y+1, w-2, h-2, fill)
	bs := Style{Fg: border}
	for xx := x + 1; xx < x+w-1; xx++ {
		c.Set(xx, y, '─', bs)
		c.Set(xx, y+h-1, '─', bs)
	}
	for yy := y + 1; yy < y+h-1; yy++ {
		c.Set(x, yy, '│', bs)
		c.Set(x+w-1, yy, '│', bs)
	}
	c.Set(x, y, '╭', bs)
	c.Set(x+w-1, y, '╮', bs)
	c.Set(x, y+h-1, '╰', bs)
	c.Set(x+w-1, y+h-1, '╯', bs)
	if title != "" && w > TextWidth(title)+6 {
		c.Set(x+2, y, ' ', bs)
		end := c.Text(x+3, y, title, titleSt)
		c.Set(end, y, ' ', bs)
	}
}

// brailleBit maps a dot inside a cell (column 0-1, row 0-3) to its bit.
var brailleBit = [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

// Dot lights one braille dot. Dot space is twice the cell width and four
// times the cell height. A dot never covers text: it lands only on a blank
// cell or one that already holds braille. When two dots share a cell, the
// brighter colour wins.
func (c *Canvas) Dot(px, py int, col Color) {
	if px < 0 || py < 0 {
		return
	}
	cell := c.At(px/2, py/4)
	if cell == nil {
		return
	}
	bit := brailleBit[py%4][px%2]
	switch {
	case cell.R == ' ':
		cell.R = 0x2800 | bit
		cell.Fg = col
		cell.Attr = 0
	case cell.R >= 0x2800 && cell.R <= 0x28FF:
		cell.R |= bit
		if col.luma() > cell.Fg.luma() {
			cell.Fg = col
		}
	}
}

// Line draws a braille line between two dots.
func (c *Canvas) Line(x0, y0, x1, y1 int, col Color) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	e := dx + dy
	for {
		c.Dot(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}

// Fade pulls every foreground in a rectangle toward a colour. It is how a
// panel fades in: draw it whole, then fade what is not there yet.
func (c *Canvas) Fade(x, y, w, h int, to Color, t float64) {
	if t <= 0 {
		return
	}
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			if cell := c.At(xx, yy); cell != nil && cell.Fg.Set {
				cell.Fg = Mix(cell.Fg, to, t)
			}
		}
	}
}

// Render writes the canvas as ANSI text, emitting SGR only on change.
func (c *Canvas) Render() string {
	var b strings.Builder
	b.Grow(c.W * c.H * 12)
	for y := 0; y < c.H; y++ {
		var cur Cell
		first := true
		for x := 0; x < c.W; x++ {
			cell := c.cells[y*c.W+x]
			if first || cell.Fg != cur.Fg || cell.Bg != cur.Bg || cell.Attr != cur.Attr {
				writeSGR(&b, cell)
				cur = cell
				first = false
			}
			b.WriteRune(cell.R)
		}
		b.WriteString("\x1b[m")
		if y < c.H-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func writeSGR(b *strings.Builder, cell Cell) {
	b.WriteString("\x1b[0")
	if cell.Attr&attrBold != 0 {
		b.WriteString(";1")
	}
	if cell.Attr&attrItalic != 0 {
		b.WriteString(";3")
	}
	if cell.Attr&attrUnderline != 0 {
		b.WriteString(";4")
	}
	if cell.Fg.Set {
		writeRGB(b, "38", cell.Fg)
	}
	if cell.Bg.Set {
		writeRGB(b, "48", cell.Bg)
	}
	b.WriteByte('m')
}

func writeRGB(b *strings.Builder, kind string, c Color) {
	b.WriteString(";")
	b.WriteString(kind)
	b.WriteString(";2;")
	b.WriteString(strconv.Itoa(int(c.R)))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa(int(c.G)))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa(int(c.B)))
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func easeOutCubic(t float64) float64 {
	t = clamp(t, 0, 1)
	return 1 - math.Pow(1-t, 3)
}

func easeInOutCubic(t float64) float64 {
	t = clamp(t, 0, 1)
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}
