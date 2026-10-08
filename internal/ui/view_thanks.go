package ui

import "math"

// thanksLayout picks the largest "thank you" that fits the card, closing the
// gaps of the rhythm before it shrinks the title. The rhythm, top to bottom:
// title, 1, stats, 1, tagline, 2, chip, brew, 1, link, 1, mark.
func thanksLayout(maxW, maxH, maxRows int) (*bigText, int, []int) {
	configs := []struct {
		pad  int
		gaps []int
	}{
		{1, []int{1, 1, 2, 1, 1}},
		{0, []int{1, 1, 2, 1, 1}},
		{0, []int{1, 1, 1, 1, 1}},
		{0, []int{1, 1, 1, 0, 1}},
		{0, []int{1, 1, 1, 0, 0}},
		{0, []int{0, 1, 1, 0, 0}},
	}
	for rows := maxRows; rows >= 3; rows-- {
		b := setBig("thank you", rows)
		if b.W > maxW {
			continue
		}
		for _, cf := range configs {
			n := 2 + 2*cf.pad + 6 + b.H
			for _, g := range cf.gaps {
				n += g
			}
			if n <= maxH {
				return b, cf.pad, cf.gaps
			}
		}
	}
	last := configs[len(configs)-1]
	return setBig("thank you", 3), last.pad, last.gaps
}

func (m *Model) drawThanks(c *Canvas) (int, int, int, int) {
	_, cy, cw, ch := m.content()
	big := cw >= 130 && ch >= 30
	w := min(cw-4, 72)
	if big {
		w = 100
	}
	maxRows := 6
	if big {
		maxRows = 9
	}
	title, pad, gaps := thanksLayout(w-6, ch, maxRows)
	fixed := func() int {
		n := 2 + 2*pad + 6
		for _, g := range gaps {
			n += g
		}
		return n
	}
	h := fixed() + title.H
	x := (cw - w) / 2
	y := cy + (ch-h)/2
	c.Box(x, y, w, h, cBorder, cPanel, "", Style{})

	// "thank you", light running across it from the cool accent into the
	// warm one. A slow shimmer passes when the view opens, then every
	// six seconds.
	since := m.t - m.viewAt
	if m.viewAt < 0 {
		since = m.t
	}
	sweep := -100.0
	if p := math.Mod(since, 6) / 1.4; p < 1 && !lowColor {
		sweep = -10 + p*float64(title.W+20)
	}
	bx := x + (w-title.W)/2
	ly := y + 1 + pad
	title.draw(c, bx, ly, cPanel, func(col, row int) Color {
		if lowColor {
			return cGold
		}
		f := float64(col) / float64(title.W)
		base := Mix(cBlue, cGold, f)
		base = Mix(base, cGoldHi, 0.25*(1-float64(row)/float64(title.H)))
		d := (float64(col) - float64(row)*0.8 - sweep) / 4
		return Mix(base, cWhite, 0.6*math.Exp(-d*d))
	})
	ly += title.H + gaps[0]

	// The numbers, in one line.
	drawSpans(c, x, w, ly, []span{
		{thousands(m.data.Stars), Style{Fg: cText, Attr: attrBold}},
		{" stars  ·  ", Style{Fg: cMuted}},
		{itoa(m.contributors()), Style{Fg: cText, Attr: attrBold}},
		{" contributors  ·  ", Style{Fg: cMuted}},
		{itoa(len(m.cum)), Style{Fg: cText, Attr: attrBold}},
		{" days", Style{Fg: cMuted}},
	})
	ly += 1 + gaps[1]
	center(c, x, w, ly, "every star, issue and patch made tuios better.", Style{Fg: cMuted})
	ly += 1 + gaps[2]

	// The install line, in a chip.
	cwid := TextWidth(installCmd) + 6
	cx := x + (w-cwid)/2
	chipBg := cPill
	if m.t-m.copied < 1.6 {
		chipBg = Mix(cPill, cGold, 0.18*(1-(m.t-m.copied)/1.6))
	}
	c.Fill(cx, ly, cwid, 1, chipBg)
	c.Text(cx+2, ly, "$", Style{Fg: cDim, Bg: chipBg})
	c.Text(cx+4, ly, installCmd, Style{Fg: cText, Bg: chipBg})
	ly++
	center(c, x, w, ly, "or brew install tuios", Style{Fg: cDim})
	ly += 1 + gaps[3]

	center(c, x, w, ly, repoShort, Style{Fg: cBlue, Attr: attrUnderline, Link: repoURL})
	ly += 1 + gaps[4]

	// The small mark at the foot of the card.
	mark := "made with tuios"
	st := Style{Fg: cDim}
	if m.inside {
		mark = "✦ made with tuios, and you are in it"
		st = Style{Fg: cGoldLo}
	}
	center(c, x, w, ly, mark, st)
	return x, y, w, h
}
