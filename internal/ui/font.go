package ui

import (
	"image"
	"image/draw"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The display face is Go Bold, rasterized at run time into quadrant blocks:
// each cell holds a 2x2 grid of sub-pixels, drawn with ▘▝▖▗▚▞▀▄▌▐▙▛▜▟█.
// Ghostty, kitty and wezterm draw those blocks themselves, so the joins
// between cells are seamless.

// A cell is taken to be twice as tall as it is wide. The raster works in
// square pixels, eight across a cell and sixteen down it, so a quadrant
// sub-pixel is a 4x8 box of them.
const (
	pxW = 8
	pxH = 16
)

var (
	goBold     *opentype.Font
	goBoldOnce sync.Once
)

func boldFont() *opentype.Font {
	goBoldOnce.Do(func() {
		f, err := opentype.Parse(gobold.TTF)
		if err != nil {
			panic(err)
		}
		goBold = f
	})
	return goBold
}

// bigText is one string set in the display face, as quadrant coverage.
type bigText struct {
	W, H int          // in cells
	cov  [][4]float32 // per cell: top-left, top-right, bottom-left, bottom-right
}

var (
	bigCache   = map[bigKey]*bigText{}
	bigCacheMu sync.Mutex
)

type bigKey struct {
	s    string
	rows int
}

// setBig sets s with a cap height of capRows cells. Digits are tabular and
// each one starts on a cell edge, so a count that ticks up does not shift
// sideways.
func setBig(s string, capRows int) *bigText {
	k := bigKey{s, capRows}
	bigCacheMu.Lock()
	if b, ok := bigCache[k]; ok {
		bigCacheMu.Unlock()
		return b
	}
	bigCacheMu.Unlock()
	b := rasterize(s, capRows)
	bigCacheMu.Lock()
	if len(bigCache) > 512 {
		clear(bigCache)
	}
	bigCache[k] = b
	bigCacheMu.Unlock()
	return b
}

// capRatio is the cap height of Go Bold as a fraction of the em.
const capRatio = 0.729

func faceFor(capRows int) font.Face {
	size := float64(capRows*pxH) / capRatio
	f, err := opentype.NewFace(boldFont(), &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	return f
}

func rasterize(s string, capRows int) *bigText {
	face := faceFor(capRows)
	defer face.Close()
	capPx := capRows * pxH
	// Lay the string out. Digits take the widest digit's advance, rounded
	// up to whole cells; the comma and the space are rounded to cells too,
	// so every digit starts on a cell edge.
	digitAdv := fixed.I(0)
	for r := '0'; r <= '9'; r++ {
		if a, ok := face.GlyphAdvance(r); ok && a > digitAdv {
			digitAdv = a
		}
	}
	roundCell := func(a fixed.Int26_6) fixed.Int26_6 {
		px := (a.Ceil() + pxW - 1) / pxW * pxW
		return fixed.I(px)
	}
	type placed struct {
		r rune
		x fixed.Int26_6
	}
	var glyphs []placed
	x := fixed.I(0)
	prev := rune(-1)
	tabular := true
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r == ',' || r == '.') {
			tabular = false
		}
	}
	for _, r := range s {
		adv, _ := face.GlyphAdvance(r)
		if tabular {
			switch {
			case r >= '0' && r <= '9':
				gx := x + (digitAdv-adv)/2
				glyphs = append(glyphs, placed{r, gx})
				x += roundCell(digitAdv)
			default:
				glyphs = append(glyphs, placed{r, x})
				x += roundCell(adv)
			}
			continue
		}
		if prev >= 0 {
			x += face.Kern(prev, r)
		}
		glyphs = append(glyphs, placed{r, x})
		x += adv
		prev = r
	}
	// Trim the trailing side bearing of the last glyph from the width.
	wpx := x.Ceil()
	if len(glyphs) > 0 && !tabular {
		last := glyphs[len(glyphs)-1]
		if bb, _, ok := face.GlyphBounds(last.r); ok {
			wpx = (last.x + bb.Max.X).Ceil() + 1
		}
	}
	// Measure what the glyphs really cover. Overshoot of a few pixels past
	// the cap height or the baseline is dropped, so a row of digits is
	// exactly capRows tall.
	top, bottom := capPx, 0
	for _, g := range glyphs {
		bb, _, ok := face.GlyphBounds(g.r)
		if !ok {
			continue
		}
		top = max(top, -bb.Min.Y.Floor())
		bottom = max(bottom, bb.Max.Y.Ceil())
	}
	if top <= capPx+3 {
		top = capPx
	}
	if bottom <= 3 {
		bottom = 0
	}
	rows := (top + bottom + pxH - 1) / pxH
	cols := (wpx + pxW - 1) / pxW
	img := image.NewAlpha(image.Rect(0, 0, cols*pxW, rows*pxH))
	base := fixed.I(top)
	for _, g := range glyphs {
		gm := glyphMask(face, g.r, capRows)
		if gm == nil {
			continue
		}
		at := image.Pt(g.x.Round(), base.Round()).Add(gm.off)
		r := gm.img.Bounds().Sub(gm.img.Bounds().Min).Add(at)
		draw.DrawMask(img, r, image.Opaque, image.Point{}, gm.img, gm.img.Bounds().Min, draw.Over)
	}
	b := &bigText{W: cols, H: rows, cov: make([][4]float32, cols*rows)}
	for cy := range rows {
		for cx := range cols {
			var q [4]float32
			for qi := range 4 {
				x0 := cx*pxW + (qi%2)*(pxW/2)
				y0 := cy*pxH + (qi/2)*(pxH/2)
				sum := 0
				for yy := y0; yy < y0+pxH/2; yy++ {
					row := img.Pix[yy*img.Stride:]
					for xx := x0; xx < x0+pxW/2; xx++ {
						sum += int(row[xx])
					}
				}
				q[qi] = float32(sum) / float32(255*pxW/2*pxH/2)
			}
			b.cov[cy*cols+cx] = q
		}
	}
	if !tabular {
		b.trim()
	}
	return b
}

// glyphMask is one glyph's coverage, copied out of the face (which reuses its
// buffer), with its top-left corner relative to the pen.
type glyphImg struct {
	img *image.Alpha
	off image.Point
}

type glyphKey struct {
	r    rune
	rows int
}

var glyphCache = map[glyphKey]*glyphImg{}

func glyphMask(face font.Face, r rune, capRows int) *glyphImg {
	k := glyphKey{r, capRows}
	if g, ok := glyphCache[k]; ok {
		return g
	}
	dr, mask, mp, _, ok := face.Glyph(fixed.Point26_6{}, r)
	if !ok || dr.Empty() {
		glyphCache[k] = nil
		return nil
	}
	a := image.NewAlpha(image.Rect(0, 0, dr.Dx(), dr.Dy()))
	draw.Draw(a, a.Bounds(), mask, mp, draw.Src)
	if r == '0' {
		unslash(a)
	}
	g := &glyphImg{a, dr.Min}
	glyphCache[k] = g
	return g
}

// unslash takes the slash out of Go Bold's zero. It fills the counter, then
// keeps only the ring within one stroke of the outline, measured with the
// glyph's own stroke widths: the sides are heavier than the top and bottom.
func unslash(a *image.Alpha) {
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	ink := func(x, y int) bool { return a.Pix[y*a.Stride+x] >= 128 }
	// Flood the outside from the border.
	out := make([]bool, w*h)
	var stack []int
	for x := range w {
		stack = append(stack, x, (h-1)*w+x)
	}
	for y := range h {
		stack = append(stack, y*w, y*w+w-1)
	}
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := i%w, i/w
		if out[i] || ink(x, y) {
			continue
		}
		out[i] = true
		if x > 0 {
			stack = append(stack, i-1)
		}
		if x < w-1 {
			stack = append(stack, i+1)
		}
		if y > 0 {
			stack = append(stack, i-w)
		}
		if y < h-1 {
			stack = append(stack, i+w)
		}
	}
	// The stroke widths: the left side at mid-height, the top at mid-width.
	tx, ty := 0, 0
	for x := 0; x < w && (tx == 0 || ink(x, h/2)); x++ {
		if ink(x, h/2) {
			tx++
		}
	}
	for y := 0; y < h && (ty == 0 || ink(w/2, y)); y++ {
		if ink(w/2, y) {
			ty++
		}
	}
	if tx == 0 || ty == 0 {
		return
	}
	// Distance to the outside, with a step across costing ty/tx of a step
	// down, so one stroke is ty either way.
	kx := float64(ty) / float64(tx)
	kd := math.Hypot(kx, 1)
	d := make([]float64, w*h)
	for i := range d {
		if out[i] {
			d[i] = 0
		} else {
			d[i] = 1e9
		}
	}
	at := func(x, y int) float64 {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 0
		}
		return d[y*w+x]
	}
	for y := range h {
		for x := range w {
			i := y*w + x
			d[i] = min(d[i], at(x-1, y)+kx, at(x, y-1)+1, at(x-1, y-1)+kd, at(x+1, y-1)+kd)
		}
	}
	for y := h - 1; y >= 0; y-- {
		for x := w - 1; x >= 0; x-- {
			i := y*w + x
			d[i] = min(d[i], at(x+1, y)+kx, at(x, y+1)+1, at(x+1, y+1)+kd, at(x-1, y+1)+kd)
		}
	}
	for y := range h {
		for x := range w {
			i := y*w + x
			if out[i] {
				continue
			}
			ring := clamp(float64(ty)+0.5-d[i], 0, 1)
			a.Pix[y*a.Stride+x] = uint8(255 * ring)
		}
	}
}

// trim drops empty columns on both sides.
func (b *bigText) trim() {
	empty := func(cx int) bool {
		for cy := range b.H {
			for _, v := range b.cov[cy*b.W+cx] {
				if v >= 0.04 {
					return false
				}
			}
		}
		return true
	}
	l, r := 0, b.W
	for l < r && empty(l) {
		l++
	}
	for r > l && empty(r-1) {
		r--
	}
	if l == 0 && r == b.W {
		return
	}
	w := r - l
	cov := make([][4]float32, w*b.H)
	for cy := range b.H {
		copy(cov[cy*w:(cy+1)*w], b.cov[cy*b.W+l:cy*b.W+r])
	}
	b.W, b.cov = w, cov
}

// quadRune maps lit quadrants (bit 0 top-left, 1 top-right, 2 bottom-left,
// 3 bottom-right) to the block that shows them.
var quadRune = [16]rune{' ', '▘', '▝', '▀', '▖', '▌', '▞', '▛', '▗', '▚', '▐', '▜', '▄', '▙', '▟', '█'}

// draw paints the text with its top-left at x, y on a ground of bg. colour
// picks each cell's ink from its column and row.
//
// The edges are crisp. Soft edges were tried, with a second colour per cell
// for the partly covered quadrants, and at the size of a terminal cell they
// read as smudges round the strokes, not as smoothing.
func (b *bigText) draw(c *Canvas, x, y int, bg Color, colour func(col, row int) Color) {
	for cy := range b.H {
		for cx := range b.W {
			bits := 0
			for i, v := range b.cov[cy*b.W+cx] {
				if v >= 0.45 {
					bits |= 1 << i
				}
			}
			if bits == 0 {
				continue
			}
			c.Set(x+cx, y+cy, quadRune[bits], Style{Fg: colour(cx, cy), Bg: bg})
		}
	}
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

// lerp is a plain linear blend.
func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// Thousands formats n with commas, for the line the program prints on exit.
func Thousands(n int) string { return thousands(n) }
