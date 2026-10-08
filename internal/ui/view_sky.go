package ui

import (
	"image/color"
	"math"
)

func toRGBA(c Color) color.Color { return color.RGBA{c.R, c.G, c.B, 255} }

// numRows is the cap height of the big number, in cells.
func (m *Model) numRows() int {
	_, _, cw, ch := m.content()
	if cw >= 130 && ch >= 30 {
		return 10
	}
	return 6
}

// skyCardRect is where the card of the sky view sits.
func (m *Model) skyCardRect() (x, y, w, h int) {
	_, cy, cw, ch := m.content()
	num := setBig(thousands(max(m.data.Stars, 1000)), m.numRows())
	w = min(cw-4, max(62, num.W+24))
	h = num.H + 10
	return (cw - w) / 2, cy + (ch-h)/2, w, h
}

// glint is where the light that runs across the number is, from 0 to 1, or
// -1 when none runs. It runs once on the 5,000 beat, and then every nine
// seconds once the count has landed.
func (m *Model) glint() float64 {
	if lowColor {
		return -1
	}
	const dur = 1.1
	if !math.IsInf(m.sky.ringAt, -1) {
		if p := (m.t - m.sky.ringAt) / dur; p >= 0 && p < 1 {
			return p
		}
	}
	if !math.IsInf(m.landed, 1) {
		since := m.t - m.landed - 2
		if since >= 0 {
			if p := math.Mod(since, 9) / dur; p < 1 {
				return p
			}
		}
	}
	return -1
}

func (m *Model) glinting() bool { return m.view == vSky && m.glint() >= 0 }

// drawSkyCard is the home view: the star count, which counts up as a
// time-lapse of the history while the sky fills with the stars it counts.
func (m *Model) drawSkyCard(c *Canvas) (int, int, int, int) {
	x, y, w, h := m.skyCardRect()
	c.Box(x, y, w, h, cBorder, cPanel, "", Style{})

	n := int(math.Round(m.num))
	if m.holding() {
		n = 5000
	}
	num := setBig(thousands(n), m.numRows())
	bx := x + (w-num.W)/2
	by := y + 2
	// The number is starlight: pale at the top, gold below. A glint runs
	// across it on the beat at 5,000 and now and then after.
	g := m.glint()
	pos := -100.0
	if g >= 0 {
		pos = -8 + g*float64(num.W+16)
	}
	rows := float64(max(1, num.H-1))
	num.draw(c, bx, by, cPanel, func(col, row int) Color {
		if lowColor {
			return cGold
		}
		base := Mix(cGoldHi, cGold, math.Min(1, float64(row)/rows*1.25))
		d := (float64(col) - float64(row)*0.9 - pos) / 3
		return Mix(base, cWhite, 0.8*math.Exp(-d*d))
	})

	capY := by + num.H + 1
	switch {
	case m.holding():
		_, d := m.beat()
		s := "5,000 on " + date(m.day(int(d)))
		center(c, x, w, capY, s, Style{Fg: cGold, Attr: attrBold})
	case !m.introDone():
		d := m.introDay()
		i := min(int(d), len(m.cum)-1)
		center(c, x, w, capY, "by "+date(m.day(max(0, i))), Style{Fg: cMuted})
	default:
		center(c, x, w, capY, "people starred tuios", Style{Fg: cText})
		// New stars since the snapshot, once, under the caption.
		if more := m.data.Stars - m.snap.Stars; more > 0 && !math.IsInf(m.landed, 1) && m.t-m.landed < 2.4 {
			a := 1 - clamp((m.t-m.landed-1.8)/0.6, 0, 1)
			center(c, x, w, capY+1, "+"+itoa(more)+" since "+shortDate(m.snap.Taken), Style{Fg: Mix(cPanel, cGold, a)})
		}
	}

	// The facts in one line.
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
	fy := capY + 2
	for i, f := range facts {
		if i > 0 {
			fx = c.Text(fx, fy, "  ·  ", Style{Fg: cDim})
		}
		fx = c.Text(fx, fy, f.n, Style{Fg: cText, Attr: attrBold})
		fx = c.Text(fx+1, fy, f.l, Style{Fg: cMuted})
	}

	ny := y + h - 3
	switch m.find {
	case findFound:
		m.drawYouLine(c, x, w, ny)
	case findMissing:
		a := easeOutCubic((m.t - m.findAt) / 0.5)
		parts := []span{
			{"not in the sky yet. ", Style{Fg: Mix(cPanel, cMuted, a)}},
			{"star tuios", Style{Fg: Mix(cPanel, cBlue, a), Attr: attrUnderline, Link: repoURL}},
			{" to join.", Style{Fg: Mix(cPanel, cMuted, a)}},
		}
		drawSpans(c, x, w, ny, parts)
	default:
		// The one idea, said once, quietly, after the sky has filled.
		hint := "every dot in this sky is one of them"
		a := easeOutCubic((m.t - (m.introAt + m.introLength() + 0.4)) / 1.2)
		if a > 0 {
			center(c, x, w, ny, hint, Style{Fg: Mix(cPanel, cMuted, a), Attr: attrItalic})
		}
	}
	return x, y, w, h
}

func (m *Model) drawYouLine(c *Canvas, x, w, y int) {
	a := easeOutCubic((m.t - m.sky.youAt - 0.4) / 0.6)
	mu := Style{Fg: Mix(cPanel, cMuted, a)}
	tx := Style{Fg: Mix(cPanel, cText, a)}
	parts := []span{
		{"you", Style{Fg: Mix(cPanel, cGold, a), Attr: attrBold}},
		{" · ", Style{Fg: Mix(cPanel, cDim, a)}},
		{date(m.day(m.youDay)), tx},
		{" · ", Style{Fg: Mix(cPanel, cDim, a)}},
		{"star ≈ ", mu},
		{"#" + thousands(m.youIdx+1), Style{Fg: Mix(cPanel, cGoldHi, a), Attr: attrBold}},
	}
	if m.youOf > 1 {
		parts = append(parts,
			span{" · ", Style{Fg: Mix(cPanel, cDim, a)}},
			span{"one of " + itoa(m.youOf) + " that day", mu})
	}
	drawSpans(c, x, w, y, parts)
}

// drawSpans writes styled pieces centred in [x, x+w).
func drawSpans(c *Canvas, x, w, y int, parts []span) {
	tw := 0
	for _, p := range parts {
		tw += TextWidth(p.s)
	}
	px := x + (w-tw)/2
	for _, p := range parts {
		px = c.Text(px, y, p.s, p.st)
	}
}
