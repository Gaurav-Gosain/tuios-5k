package ui

import (
	"math"
	"time"
)

// plotGeom is where the history chart sits inside its panel.
type plotGeom struct {
	px, py, pw, ph int // panel
	x, y, w, h     int // plot area in cells
	ymax           int
}

func (m *Model) historyGeom() plotGeom {
	px, py, pw, ph := m.panel()
	g := plotGeom{px: px, py: py, pw: pw, ph: ph}
	g.x = px + 2 + 5
	g.y = py + 3
	g.w = pw - 4 - 5 - 1
	g.h = ph - 3 - 4
	top := 0
	if len(m.cum) > 0 {
		top = m.cum[len(m.cum)-1]
	}
	g.ymax = (top/1000 + 1) * 1000
	return g
}

func (m *Model) plotDots() int { return max(1, m.historyGeom().w*2) }

// dotX and dotY map a day and a total to dot space.
func (g plotGeom) dotX(day, days int) int {
	if days <= 1 {
		return g.x * 2
	}
	return g.x*2 + int(math.Round(float64(day)/float64(days-1)*float64(g.w*2-1)))
}

func (g plotGeom) dotY(v int) int {
	f := float64(v) / float64(g.ymax)
	return g.y*4 + int(math.Round((1-f)*float64(g.h*4-1)))
}

func (m *Model) drawHistory(c *Canvas) (int, int, int, int) {
	g := m.historyGeom()
	c.Box(g.px, g.py, g.pw, g.ph, cBorder, cPanel, "star history", Style{Fg: cText})
	days := len(m.cum)
	if days == 0 {
		return g.px, g.py, g.pw, g.ph
	}

	// The line draws itself in, left to right, a moment after the view
	// opens.
	reveal := easeInOutCubic((m.t - m.viewAt - 0.15) / 1.6)
	if m.viewAt < 0 {
		reveal = easeInOutCubic((m.t - 0.3) / 1.6)
	}
	upto := int(reveal * float64(days-1))

	// Gridlines every thousand, dotted, with labels on the left.
	for v := 1000; v < g.ymax; v += 1000 {
		dy := g.dotY(v)
		for dx := g.x * 2; dx < (g.x+g.w)*2; dx += 3 {
			c.Dot(dx, dy, cFaint)
		}
		lbl := itoa(v/1000) + "k"
		c.Text(g.x-2-TextWidth(lbl), dy/4, lbl, Style{Fg: cDim})
	}
	c.Text(g.x-3, g.y+g.h-1, "0", Style{Fg: cDim})

	// The axis, with a tick at the start of each month.
	ay := g.y + g.h
	for x := g.x - 1; x < g.x+g.w; x++ {
		c.Set(x, ay, '─', Style{Fg: cFaint})
	}
	c.Set(g.x-1, ay, '╰', Style{Fg: cFaint})
	for y := g.y; y < ay; y++ {
		c.Set(g.x-1, y, '│', Style{Fg: cFaint})
	}
	start := m.data.StartDay()
	lastEnd := -10
	for mo := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC); ; mo = mo.AddDate(0, 1, 0) {
		d := int(mo.Sub(start).Hours() / 24)
		if d >= days {
			break
		}
		if d < 0 {
			d = 0
		}
		cx := g.dotX(d, days) / 2
		lbl := lowerMonth(mo)
		if mo.Month() == time.January {
			lbl = itoa(mo.Year())
		}
		if d > 0 {
			c.Set(cx, ay, '┴', Style{Fg: cFaint})
		}
		if cx > lastEnd+1 && cx+TextWidth(lbl) <= g.x+g.w {
			st := Style{Fg: cDim}
			if mo.Month() == time.January {
				st = Style{Fg: cMuted}
			}
			lastEnd = c.Text(cx, ay+1, lbl, st)
		}
	}

	// The area under the line: a cool glow that thins toward the axis.
	// Each cell is tinted by how much of it lies under the line, so the
	// edge of the area is as fine as the braille line, not a staircase.
	topAt := func(dx int) float64 {
		f := float64(dx-g.x*2) / float64(max(1, g.w*2-1)) * float64(days-1)
		i := int(f)
		if i >= days-1 {
			return float64(g.dotY(m.cum[days-1]))
		}
		a, b := float64(g.dotY(m.cum[i])), float64(g.dotY(m.cum[i+1]))
		return a + (b-a)*(f-float64(i))
	}
	edge := g.dotX(upto, days)
	if lowColor {
		edge = -1 // the soft fill would band into blocks
	}
	for cx := 0; cx < g.w; cx++ {
		x := g.x + cx
		if x*2 > edge {
			break
		}
		t0, t1 := topAt(x*2), topAt(x*2+1)
		for y := int(math.Min(t0, t1)) / 4; y < ay; y++ {
			cover := 0.0
			for _, top := range []float64{t0, t1} {
				cover += clamp((float64(y*4+4)-top)/4, 0, 1) / 2
			}
			if cover <= 0 {
				continue
			}
			f := 1 - float64(y-g.y)/float64(max(1, g.h))
			if cell := c.At(x, y); cell != nil {
				cell.Bg = Mix(cPanel, cBlueLo, cover*(0.12+0.38*f))
			}
		}
	}

	// The line itself.
	for d := 1; d <= upto; d++ {
		f := float64(d) / float64(days)
		col := Mix(cBlue, cGold, f*f)
		x0, y0, x1, y1 := g.dotX(d-1, days), g.dotY(m.cum[d-1]), g.dotX(d, days), g.dotY(m.cum[d])
		c.Line(x0, y0, x1, y1, col)
	}
	// Its leading edge glows while it draws.
	if upto < days-1 {
		hx, hy := g.dotX(upto, days), g.dotY(m.cum[upto])
		c.Dot(hx, hy, cGoldHi)
		c.Dot(hx+1, hy, cGoldHi)
	}

	// The cursor: a dotted rule and a bright point on the line. It goes in
	// before the milestone labels, so a label stays whole when they meet.
	if m.cursor >= 0 && m.cursor < days {
		d := m.cursor
		dx, dy := g.dotX(d, days), g.dotY(m.cum[d])
		for yy := g.y * 4; yy < ay*4; yy += 2 {
			if yy < dy-1 || yy > dy+1 {
				c.Dot(dx, yy, cBlue)
			}
		}
		pulse := 0.5 + 0.5*math.Sin((m.t-m.cursorAt)*5)
		c.Set(dx/2, dy/4, '◆', Style{Fg: Mix(cGold, cGoldHi, pulse)})
	}

	// Milestones at 1k and 5k: a star on the line, a dotted drop to the
	// axis, and the day it happened.
	for _, ms := range []int{1000, 5000} {
		d := dayOf(m.cum, ms)
		if d < 0 || d > upto {
			continue
		}
		dx, dy := g.dotX(d, days), g.dotY(m.cum[d])
		for yy := dy + 4; yy < ay*4; yy += 2 {
			c.Dot(dx, yy, cDim)
		}
		cx, cy := dx/2, dy/4
		c.Set(cx, cy, '✦', Style{Fg: cGoldHi})
		lbl := itoa(ms/1000) + "k"
		when := "  " + date(m.day(d))
		lw := TextWidth(lbl) + TextWidth(when)
		lx := cx + 2
		if lx+lw >= g.x+g.w {
			lx = cx - 2 - lw
		}
		ly := cy - 1
		if ly < g.y {
			ly = cy + 1
		}
		c.Set(lx-1, ly, ' ', Style{})
		e := c.Text(lx, ly, lbl, Style{Fg: cGold, Attr: attrBold})
		e = c.Text(e, ly, when, Style{Fg: cMuted})
		c.Set(e, ly, ' ', Style{})
	}

	// The top line: the headline on the left, the cursor or best day on
	// the right.
	ty := g.py + 1
	tx := c.Text(g.px+3, ty, thousands(m.data.Stars), Style{Fg: cGold, Attr: attrBold})
	c.Text(tx, ty, " stars in "+itoa(days)+" days", Style{Fg: cMuted})

	var right []struct {
		s  string
		st Style
	}
	add := func(s string, st Style) {
		right = append(right, struct {
			s  string
			st Style
		}{s, st})
	}
	if m.cursor >= 0 && m.cursor < days {
		d := m.cursor
		add(date(m.day(d)), Style{Fg: cText})
		add("   +"+itoa(m.data.Daily[d]), Style{Fg: cGold})
		add(" that day   ", Style{Fg: cMuted})
		add(thousands(m.cum[d]), Style{Fg: cText})
		add(" total", Style{Fg: cMuted})
	} else {
		best := 0
		for i, v := range m.data.Daily {
			if v > m.data.Daily[best] {
				best = i
			}
		}
		add("best day  ", Style{Fg: cMuted})
		add(date(m.day(best)), Style{Fg: cText})
		add("  +"+itoa(m.data.Daily[best]), Style{Fg: cGold})
	}
	rw := 0
	for _, r := range right {
		rw += TextWidth(r.s)
	}
	rx := g.px + g.pw - 3 - rw
	for _, r := range right {
		rx = c.Text(rx, ty, r.s, r.st)
	}
	return g.px, g.py, g.pw, g.ph
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
