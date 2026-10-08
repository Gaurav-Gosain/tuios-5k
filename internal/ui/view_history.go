package ui

import (
	"math"
	"slices"
	"time"
)

// plotGeom is where the history chart sits inside its panel.
type plotGeom struct {
	px, py, pw, ph int // panel
	x, y, w, h     int // plot area in cells; the axis is the row under it
	ymax           int
}

func (m *Model) historyGeom() plotGeom {
	px, py, pw, ph := m.panel()
	g := plotGeom{px: px, py: py, pw: pw, ph: ph}
	g.x = px + 2 + 5
	g.y = py + 2
	g.w = pw - 4 - 5 - 1
	g.h = ph - 6
	top := 0
	if len(m.cum) > 0 {
		top = m.cum[len(m.cum)-1]
	}
	g.ymax = max(500, int(math.Ceil(float64(top)*1.08/500))*500)
	return g
}

// rowOf is the cell row a total sits on, counting from the plot top.
func (g plotGeom) eighths(v float64) int {
	return int(math.Round(v / float64(g.ymax) * float64(g.h*8)))
}

// colOf is the plot column of day d.
func (g plotGeom) colOf(d, days int) int {
	if days <= 1 {
		return g.x
	}
	return g.x + int(float64(d)/float64(days)*float64(g.w))
}

// bestDays are the k days with the most stars, best first.
func (m *Model) bestDays(k int) []int {
	idx := make([]int, len(m.data.Daily))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int { return m.data.Daily[b] - m.data.Daily[a] })
	return idx[:min(k, len(idx))]
}

func slicesSortUnique(s *[]int) {
	slices.Sort(*s)
	*s = slices.Compact(*s)
}

var eighthRune = [9]rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func (m *Model) drawHistory(c *Canvas) (int, int, int, int) {
	g := m.historyGeom()
	m.hist = g
	c.Box(g.px, g.py, g.pw, g.ph, cBorder, cPanel, "", Style{})
	days := len(m.cum)
	if days == 0 {
		return g.px, g.py, g.pw, g.ph
	}
	ay := g.y + g.h

	// The headline sits in the top border, and the best day on its right.
	bs := Style{Fg: cBorder}
	tx := c.Text(g.px+2, g.py, " ", bs)
	tx = c.Text(tx, g.py, thousands(m.data.Stars), Style{Fg: cGold, Attr: attrBold})
	c.Text(tx, g.py, " stars in "+itoa(days)+" days ", Style{Fg: cText})
	best := m.bestDays(1)[0]
	right := []span{
		{" best day ", Style{Fg: cMuted}},
		{date(m.day(best)), Style{Fg: cText}},
		{" +" + itoa(m.data.Daily[best]) + " ", Style{Fg: cGold}},
	}
	rw := 0
	for _, r := range right {
		rw += TextWidth(r.s)
	}
	rx := g.px + g.pw - 3 - rw
	for _, r := range right {
		rx = c.Text(rx, g.py, r.s, r.st)
	}

	// The area draws itself in, left to right, a moment after the view
	// opens.
	reveal := easeInOutCubic((m.t - m.viewAt - 0.15) / 1.6)
	if m.viewAt < 0 {
		reveal = easeInOutCubic((m.t - 0.3) / 1.6)
	}
	edge := int(math.Ceil(reveal * float64(g.w)))

	// The area: eighth blocks, one column per cell, so its edge follows
	// the total to an eighth of a cell. The top cell of each column is the
	// line, bright, cooling from blue to gold over time. Under it the light
	// falls away toward the axis.
	cursorCol := -1
	if m.cursor >= 0 && m.cursor < days {
		cursorCol = g.colOf(m.cursor, days) - g.x
	}
	crest := make([]int, g.w) // the row of each column's top cell
	for cx := 0; cx < g.w; cx++ {
		crest[cx] = -1
		if cx >= edge {
			continue
		}
		d := float64(cx+1) / float64(g.w) * float64(days)
		e := g.eighths(m.cumAt(d))
		if e <= 0 {
			continue
		}
		full, part := e/8, e%8
		f := float64(cx) / float64(g.w)
		line := Mix(cBlue, cGold, f*f)
		if lowColor && f < 0.6 {
			line = cBlue
		} else if lowColor {
			line = cGold
		}
		topRow := ay - full
		if part > 0 {
			topRow = ay - full - 1
		}
		crest[cx] = topRow
		// The body fades by row, the same on every column, so the fill
		// reads as one surface and not as a row of bars.
		body := func(y int) Color {
			depth := float64(y-g.y) / float64(max(1, g.h))
			return Mix(cPanel, line, 0.07+0.5*math.Pow(1-depth, 1.6))
		}
		for y := topRow; y < ay; y++ {
			cell := c.At(g.x+cx, y)
			if cell == nil {
				continue
			}
			if y == topRow {
				ink := Mix(body(y), line, 0.75)
				if cx == cursorCol {
					ink = cGoldHi
				}
				if lowColor {
					ink = line
				}
				r := eighthRune[part]
				if part == 0 {
					r = '█'
				}
				cell.R, cell.Fg, cell.Bg = r, ink, cPanel
				continue
			}
			cell.R = ' '
			cell.Bg = body(y)
			if lowColor {
				cell.Bg = cPill
			}
		}
	}

	// Gridlines every thousand, dotted, with labels on the left. They go
	// in after the area, so they show through its body.
	grid := Mix(cFaint, cBlueLo, 0.35)
	for v := 1000; v < g.ymax; v += 1000 {
		row := ay - int(math.Round(float64(v)/float64(g.ymax)*float64(g.h)))
		for dx := g.x * 2; dx < (g.x+g.w)*2; dx += 3 {
			c.Dot(dx, row*4+3, grid)
		}
		lbl := itoa(v/1000) + "k"
		c.Text(g.x-2-TextWidth(lbl), row, lbl, Style{Fg: cDim})
	}
	c.Text(g.x-3, ay-1, "0", Style{Fg: cDim})

	// The axis, with a tick at the start of each month.
	axis := Style{Fg: cFaint}
	for x := g.x - 1; x < g.x+g.w; x++ {
		c.Set(x, ay, '─', axis)
	}
	c.Set(g.x-1, ay, '╰', axis)
	for y := g.y; y < ay; y++ {
		c.Set(g.x-1, y, '│', axis)
	}
	start := m.data.StartDay()
	monthCols := float64(g.w) * 30.4 / float64(days)
	every := 1
	if monthCols < 6 {
		every = 2
	}
	lastEnd := -10
	k := 0
	for mo := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC); ; mo = mo.AddDate(0, 1, 0) {
		d := int(mo.Sub(start).Hours() / 24)
		if d >= days {
			break
		}
		d = max(d, 0)
		cx := g.colOf(d, days)
		if d > 0 {
			c.Set(cx, ay, '┴', axis)
		}
		jan := mo.Month() == time.January
		lbl := lowerMonth(mo)
		if jan {
			lbl = itoa(mo.Year())
		}
		show := k%every == 0 || jan
		k++
		if show && cx > lastEnd+1 && cx+TextWidth(lbl) <= g.x+g.w {
			st := Style{Fg: cDim}
			if jan {
				st = Style{Fg: cMuted}
			}
			lastEnd = c.Text(cx, ay+1, lbl, st)
		}
	}

	// The cursor: a dotted rule from the top of the plot down to the line.
	if cursorCol >= 0 && cursorCol < g.w && crest[cursorCol] >= 0 {
		dx := (g.x+cursorCol)*2 + 1
		for yy := g.y * 4; yy < crest[cursorCol]*4; yy += 2 {
			c.Dot(dx, yy, Mix(cPanel, cGold, 0.7))
		}
	}

	// The three best days, small, in gold, over their columns.
	labelled := map[int]bool{}
	for _, d := range m.bestDays(3) {
		cx := g.colOf(d, days) - g.x
		if cx < 0 || cx >= g.w || crest[cx] < 0 {
			continue
		}
		lbl := "+" + itoa(m.data.Daily[d])
		lx := g.x + cx - TextWidth(lbl)/2
		ly := crest[cx] - 1
		if ly < g.y || labelled[ly*1000+lx/6] {
			continue
		}
		labelled[ly*1000+lx/6] = true
		c.Text(lx, ly, lbl, Style{Fg: cGoldLo})
	}

	// Milestones at 1k and 5k: a star over the line, and the day.
	for _, ms := range []int{1000, 5000} {
		d := dayOf(m.cum, ms)
		if d < 0 {
			continue
		}
		cx := g.colOf(d, days) - g.x
		if cx >= edge || cx < 0 || cx >= g.w || crest[cx] < 0 {
			continue
		}
		sx, sy := g.x+cx, crest[cx]-1
		if ms == 5000 {
			sy--
		}
		lbl := itoa(ms/1000) + "k"
		when := "  " + date(m.day(d))
		lw := TextWidth(lbl) + TextWidth(when)
		lx := sx + 2
		if lx+lw >= g.x+g.w {
			lx = sx - 2 - lw
		}
		// A dotted drop from the star to the line.
		for yy := sy*4 + 4; yy < crest[cx]*4; yy += 2 {
			c.Dot(sx*2+1, yy, cDim)
		}
		c.Set(sx, sy, '✦', Style{Fg: cGoldHi})
		c.Set(lx-1, sy, ' ', Style{})
		e := c.Text(lx, sy, lbl, Style{Fg: cGold, Attr: attrBold})
		e = c.Text(e, sy, when, Style{Fg: cMuted})
		c.Set(e, sy, ' ', Style{})
	}

	if cursorCol >= 0 && cursorCol < g.w && crest[cursorCol] >= 0 {
		m.drawTooltip(c, g, g.x+cursorCol, crest[cursorCol])
	}
	return g.px, g.py, g.pw, g.ph
}

// drawTooltip is the small rounded card that reads the day under the cursor.
func (m *Model) drawTooltip(c *Canvas, g plotGeom, col, crestRow int) {
	d := m.cursor
	var parts []span
	parts = append(parts, span{date(m.day(d)), Style{Fg: cText, Bg: cPill}})
	if n := m.data.Daily[d]; n > 0 {
		word := " stars"
		if n == 1 {
			word = " star"
		}
		parts = append(parts, span{"  +" + itoa(n), Style{Fg: cGold, Bg: cPill, Attr: attrBold}}, span{word, Style{Fg: cMuted, Bg: cPill}})
	} else {
		parts = append(parts, span{"  no new stars", Style{Fg: cMuted, Bg: cPill}})
	}
	parts = append(parts, span{"  " + thousands(m.cum[d]), Style{Fg: cText, Bg: cPill}}, span{" total", Style{Fg: cMuted, Bg: cPill}})
	tw := 0
	for _, p := range parts {
		tw += TextWidth(p.s)
	}
	w, h := tw+4, 3
	x := col + 2
	if x+w > g.x+g.w {
		x = col - 1 - w
	}
	y := crestRow - 4
	if y < g.y {
		y = min(crestRow+1, g.y+g.h-h)
	}
	c.Fill(x, y, w, h, cPill)
	px := x + 2
	for _, p := range parts {
		px = c.Text(px, y+1, p.s, p.st)
	}
}

// dayOf is the first day the running total reached v, or -1.
func dayOf(cum []int, v int) int {
	for i, c := range cum {
		if c >= v {
			return i
		}
	}
	return -1
}

func lowerMonth(t time.Time) string {
	return [...]string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}[t.Month()-1]
}
