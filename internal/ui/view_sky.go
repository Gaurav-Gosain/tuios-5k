package ui

import (
	"image/color"
	"math"
)

func toRGBA(c Color) color.Color { return color.RGBA{c.R, c.G, c.B, 255} }

// drawSkyCard is the home view: the star count, which counts up as a
// time-lapse of the history while the sky fills with the stars it counts.
func (m *Model) drawSkyCard(c *Canvas) (int, int, int, int) {
	_, cy, cw, ch := m.content()
	scale := 1
	if cw >= 130 && ch >= 40 {
		scale = 2
	}
	w := min(cw-4, 62+28*(scale-1))
	h := 16 + 4*(scale-1)
	x := (cw - w) / 2
	y := cy + (ch-h)/2
	c.Box(x, y, w, h, cBorder, cPanel, "", Style{})

	n := int(math.Round(m.num))
	num := thousands(n)
	bw := bigWidth(num, scale)
	bx := x + (w-bw)/2
	by := y + 2
	// The number is starlight: pale at the top, warmer below. Once the
	// count lands, a glint runs across it, and again every few seconds.
	glint := -100.0
	if m.introDone() {
		since := m.t - (m.introAt + introLen)
		p := math.Mod(since, 7.0) / 1.1
		if p < 1 {
			glint = -6 + p*float64(bw+12)
		}
	}
	drawBig(c, bx, by, num, scale, cPanel, func(col, row int) Color {
		base := Mix(cGoldHi, cGold, float64(row)/2.5)
		if row == 3 {
			base = Mix(cGold, cGoldLo, 0.45)
		}
		d := (float64(col)/float64(scale) - float64(row)*1.5 - glint/float64(scale)) / 2.5
		return Mix(base, Hex("#ffffff"), 0.85*math.Exp(-d*d))
	})

	cap := "people starred tuios"
	capSt := Style{Fg: cText}
	if !m.introDone() {
		d := m.introDay()
		i := min(int(d), len(m.cum)-1)
		cap = "by " + date(m.day(max(0, i)))
		capSt = Style{Fg: cMuted}
	}
	center(c, x, w, by+bigHeight(scale)+1, cap, capSt)

	// A short rule, then the facts in one line.
	rule := 18
	for i := range rule {
		c.Set(x+(w-rule)/2+i, by+bigHeight(scale)+3, '─', Style{Fg: cFaint})
	}
	days := len(m.cum)
	facts := []struct{ n, l string }{
		{itoa(days), "days"},
		{itoa(m.contributors()), "contributors"},
		{itoa(m.data.Forks), "forks"},
	}
	fw := 0
	for i, f := range facts {
		fw += TextWidth(f.n) + 1 + TextWidth(f.l)
		if i > 0 {
			fw += 5
		}
	}
	fx := x + (w-fw)/2
	fy := by + bigHeight(scale) + 5
	for i, f := range facts {
		if i > 0 {
			fx = c.Text(fx, fy, "  ·  ", Style{Fg: cDim})
		}
		fx = c.Text(fx, fy, f.n, Style{Fg: cText, Attr: attrBold})
		fx = c.Text(fx+1, fy, f.l, Style{Fg: cMuted})
	}

	// The one idea, said once, quietly, after the sky has filled.
	hint := "every dot in this sky is one of them"
	a := easeOutCubic((m.t - (m.introAt + introLen + 0.4)) / 1.2)
	if a > 0 {
		center(c, x, w, y+h-3, hint, Style{Fg: Mix(cPanel, cMuted, a), Attr: attrItalic})
	}
	return x, y, w, h
}
