package ui

import "math"

func (m *Model) drawThanks(c *Canvas) (int, int, int, int) {
	_, cy, cw, ch := m.content()
	big := cw >= 130 && ch >= 40
	w, h := min(cw-4, 72), 20
	if big {
		w, h = 112, 24
	}
	x := (cw - w) / 2
	y := cy + (ch-h)/2
	c.Box(x, y, w, h, cBorder, cPanel, "", Style{})

	// "thank you" in the display face, light running across it from the
	// cool accent into the warm one.
	title := "thank you"
	scale := 1
	if big {
		scale = 2
	}
	bw := bigWidth(title, scale)
	bx := x + (w-bw)/2
	since := m.t - m.viewAt
	drawBig(c, bx, y+2, title, scale, cPanel, func(col, row int) Color {
		f := float64(col) / float64(bw)
		base := Mix(cBlue, cGold, f)
		base = Mix(base, cGoldHi, 0.25*float64(3-row)/3)
		// A slow shimmer that drifts left to right.
		d := (float64(col) - math.Mod(since*14, float64(bw+40)) + 10) / 4
		return Mix(base, Hex("#ffffff"), 0.6*math.Exp(-d*d))
	})

	ly := y + 7
	if big {
		ly += 4
	}
	// The numbers, in one line.
	parts := []struct {
		s  string
		st Style
	}{
		{thousands(m.data.Stars), Style{Fg: cText, Attr: attrBold}},
		{" stars  ·  ", Style{Fg: cMuted}},
		{itoa(m.contributors()), Style{Fg: cText, Attr: attrBold}},
		{" contributors  ·  ", Style{Fg: cMuted}},
		{itoa(len(m.cum)), Style{Fg: cText, Attr: attrBold}},
		{" days", Style{Fg: cMuted}},
	}
	pw := 0
	for _, p := range parts {
		pw += TextWidth(p.s)
	}
	px := x + (w-pw)/2
	for _, p := range parts {
		px = c.Text(px, ly, p.s, p.st)
	}
	center(c, x, w, ly+2, "every star, issue and patch made tuios better.", Style{Fg: cMuted})

	// The install line, in a chip.
	chip := "$ " + installCmd
	cwid := TextWidth(chip) + 4
	cx := x + (w-cwid)/2
	cyy := ly + 6
	center(c, x, w, cyy-1, "get it", Style{Fg: cDim})
	c.Fill(cx, cyy, cwid, 1, cPill)
	prompt := Style{Fg: cDim, Bg: cPill}
	lead := "$"
	if m.t-m.copied < 1.6 {
		lead = "✓"
		prompt = Style{Fg: cGreen, Bg: cPill}
	}
	c.Text(cx+2, cyy, lead, prompt)
	c.Text(cx+4, cyy, installCmd, Style{Fg: cText, Bg: cPill})
	center(c, x, w, cyy+1, "or  brew install tuios", Style{Fg: cDim})

	center(c, x, w, cyy+3, repoURL, Style{Fg: cBlue, Attr: attrUnderline})

	// The small mark at the foot of the card.
	mark := "made with tuios"
	st := Style{Fg: cDim}
	if m.inside {
		mark = "✦ made with tuios, and you are in it"
		st = Style{Fg: cGoldLo}
	}
	c.Text(x+w-3-TextWidth(mark), y+h-2, mark, st)
	return x, y, w, h
}
