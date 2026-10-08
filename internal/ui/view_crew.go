package ui

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/Gaurav-Gosain/tuios-5k/internal/gh"
)

// crewOrder sorts contributors by the day of their first commit, so the
// constellation grows outward from the first one.
func crewOrder(in []gh.Contributor) []gh.Contributor {
	out := slices.Clone(in)
	slices.SortStableFunc(out, func(a, b gh.Contributor) int {
		if a.First != b.First {
			if a.First == "" {
				return 1
			}
			if b.First == "" {
				return -1
			}
			return strings.Compare(a.First, b.First)
		}
		return b.Commits - a.Commits
	})
	return out
}

// crewPoints lays the contributors on a spiral, in dot space: the first
// commit at the centre, each later person a step further round and out.
func crewPoints(n int, x, y, w, h int) [][2]float64 {
	pts := make([][2]float64, n)
	cx := float64(x*2) + float64(w*2)*0.47
	cy := float64(y*4) + float64(h*4)*0.52
	rx := float64(w*2) * 0.40
	ry := float64(h*4) * 0.47
	turns := 2.1
	for k := range n {
		f := 0.0
		if n > 1 {
			f = float64(k) / float64(n-1)
		}
		// sqrt spacing keeps the arc length between neighbours even.
		r := 0.08 + 0.92*math.Sqrt(f)
		th := -math.Pi/2 + turns*2*math.Pi*math.Sqrt(f)
		// A little deterministic wobble, so it reads as a sky and not a
		// diagram.
		j := float64(splitmix(uint64(k)+99)%1000)/1000 - 0.5
		r *= 1 + 0.06*j
		th += 0.08 * j
		if k == 0 {
			r = 0
		}
		pts[k] = [2]float64{cx + rx*r*math.Cos(th), cy + ry*r*math.Sin(th)}
	}
	return pts
}

func (m *Model) drawCrew(c *Canvas) (int, int, int, int) {
	px, py, pw, ph := m.panel()
	c.Box(px, py, pw, ph, cBorder, cPanel, "", Style{})
	n := len(m.crew)
	if n == 0 {
		return px, py, pw, ph
	}
	tx := c.Text(px+2, py, " ", Style{Fg: cBorder})
	tx = c.Text(tx, py, itoa(n), Style{Fg: cGold, Attr: attrBold})
	c.Text(tx, py, " people wrote tuios, in the order they arrived ", Style{Fg: cText})

	// The sky area of the panel, leaving the bottom line for text.
	ax, ay, aw, ah := px+2, py+2, pw-4, ph-5
	pts := crewPoints(n, ax, ay, aw, ah)
	m.crewPts = pts

	// Lines join in order when the view opens, one person at a time.
	since := m.t - m.viewAt
	if m.viewAt < 0 {
		since = m.t
	}
	per := math.Min(0.07, 1.8/float64(n))
	reached := (since - 0.2) / per // fractional index of the drawing head

	for k := 1; k < n; k++ {
		f := clamp(reached-float64(k-1), 0, 1)
		if f <= 0 {
			break
		}
		a, b := pts[k-1], pts[k]
		ex := a[0] + (b[0]-a[0])*f
		ey := a[1] + (b[1]-a[1])*f
		col := cLine
		if k <= m.sel {
			col = cBlue
		}
		c.Line(int(a[0]), int(a[1]), int(ex), int(ey), col)
	}

	// The walk: a spark runs from the last person to the next.
	if age := m.t - m.selAt; age >= 0 && age < 0.18 && m.selFrom != m.sel && m.selFrom < n {
		p := easeOutCubic(age / 0.18)
		a, b := pts[m.selFrom], pts[m.sel]
		sx, sy := a[0]+(b[0]-a[0])*p, a[1]+(b[1]-a[1])*p
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 || dy == 0 {
					c.Dot(int(sx)+dx, int(sy)+dy, cGoldHi)
				}
			}
		}
	}

	// Labels. On a narrow screen only the people that matter right now get
	// one: the selected person, the two either side, and the five with the
	// most commits. Walking reveals the rest.
	narrow := m.w < 120
	labelled := map[int]bool{}
	if narrow {
		labelled[m.sel] = true
		labelled[(m.sel+n-1)%n] = true
		labelled[(m.sel+1)%n] = true
		top := make([]int, n)
		for k := range top {
			top[k] = k
		}
		slices.SortStableFunc(top, func(a, b int) int { return m.crew[b].Commits - m.crew[a].Commits })
		for _, k := range top[:min(5, n)] {
			labelled[k] = true
		}
	}
	occ := make(map[[2]int]bool)
	type place struct {
		k    int
		x, y int
	}
	var placed []place
	for k := range n {
		if float64(k) > reached {
			continue
		}
		p := pts[k]
		occ[[2]int{int(p[0]) / 2, int(p[1]) / 4}] = true
	}
	// The tooltip of the selected person takes its space first.
	tipX, tipY, tipW, tipH := m.crewTip(pts[m.sel], ax, ay, aw, ah)
	for y := tipY; y < tipY+tipH; y++ {
		for x := tipX - 1; x <= tipX+tipW; x++ {
			occ[[2]int{x, y}] = true
		}
	}
	order := make([]int, 0, n)
	for k := range n {
		order = append(order, k)
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return m.crew[b].Commits - m.crew[a].Commits
	})
	free := func(x, y, w int) bool {
		if y < ay || y >= ay+ah || x < ax || x+w > ax+aw {
			return false
		}
		for i := x - 1; i <= x+w; i++ {
			if occ[[2]int{i, y}] {
				return false
			}
		}
		return true
	}
	for _, k := range order {
		if float64(k) > reached || k == m.sel || narrow && !labelled[k] {
			continue
		}
		p := pts[k]
		sx, sy := int(p[0])/2, int(p[1])/4
		w := TextWidth(m.crew[k].Login)
		cands := [][2]int{{sx + 2, sy}, {sx - 1 - w, sy}, {sx - w/2, sy - 1}, {sx - w/2, sy + 1}}
		for _, cd := range cands {
			if free(cd[0], cd[1], w) {
				for i := cd[0] - 1; i <= cd[0]+w; i++ {
					occ[[2]int{i, cd[1]}] = true
				}
				placed = append(placed, place{k, cd[0], cd[1]})
				break
			}
		}
	}
	for _, pl := range placed {
		login := m.crew[pl.k].Login
		st := Style{Fg: cMuted, Link: "https://github.com/" + login}
		if m.crew[pl.k].Commits >= 10 {
			st.Fg = Mix(cMuted, cText, 0.4)
		}
		c.Set(pl.x-1, pl.y, ' ', Style{})
		e := c.Text(pl.x, pl.y, login, st)
		c.Set(e, pl.y, ' ', Style{})
	}

	// The stars, sized by commits, with a twinkle.
	for k := range n {
		if float64(k) > reached {
			continue
		}
		p := pts[k]
		cm := m.crew[k].Commits
		g, base := '·', Mix(cPanel, cStarCool, 0.6)
		switch {
		case cm >= 100:
			g, base = '✦', cGoldHi
		case cm >= 10:
			g, base = '✦', cStarCool
		case cm >= 3:
			g, base = '+', Mix(cPanel, cStarCool, 0.85)
		case cm >= 2:
			g, base = '•', Mix(cPanel, cStarCool, 0.75)
		}
		tw := 0.5 + 0.5*math.Sin(m.t*(1.1+float64(k%5)*0.3)+float64(k))
		col := Mix(base, Mix(cPanel, base, 0.6), 0.4*tw)
		// A star flares as the line reaches it.
		if age := (reached - float64(k)) * per; age < 0.5 {
			col = Mix(col, cWhite, 1-age/0.5)
		}
		if k == m.sel {
			pulse := 0.5 + 0.5*math.Sin((m.t-m.selAt)*4)
			col = Mix(cGold, cGoldHi, pulse)
			if g == '·' || g == '•' {
				g = '+'
			}
		}
		c.Set(int(p[0])/2, int(p[1])/4, g, Style{Fg: col})
	}

	if float64(m.sel) <= reached {
		m.drawCrewTip(c, tipX, tipY, tipW)
	}

	// The bottom line: where the selected person lives, and the position.
	who := m.crew[m.sel]
	by := py + ph - 2
	url := "github.com/" + who.Login
	c.Text(px+3, by, url, Style{Fg: cDim, Link: "https://" + url})
	pos := "#" + itoa(m.sel+1) + " of " + itoa(n)
	c.Text(px+pw-3-TextWidth(pos), by, pos, Style{Fg: cMuted})
	return px, py, pw, ph
}

// crewTipLines are the rows of the tooltip of the selected person.
func (m *Model) crewTipLines() [][]span {
	who := m.crew[m.sel]
	word := " commits"
	if who.Commits == 1 {
		word = " commit"
	}
	lines := [][]span{
		{{who.Login, Style{Fg: cGoldHi, Bg: cPill, Attr: attrBold, Link: "https://github.com/" + who.Login}}},
		{{thousands(who.Commits), Style{Fg: cText, Bg: cPill, Attr: attrBold}}, {word, Style{Fg: cMuted, Bg: cPill}}},
	}
	if t, err := time.Parse("2006-01-02", who.First); err == nil {
		lines = append(lines, []span{{"since ", Style{Fg: cMuted, Bg: cPill}}, {date(t), Style{Fg: cText, Bg: cPill}}})
	}
	return lines
}

// crewTip places the tooltip next to the star at p, flipped near the edges.
func (m *Model) crewTip(p [2]float64, ax, ay, aw, ah int) (x, y, w, h int) {
	lines := m.crewTipLines()
	for _, l := range lines {
		lw := 0
		for _, s := range l {
			lw += TextWidth(s.s)
		}
		w = max(w, lw)
	}
	w += 4
	h = len(lines) + 2
	sx, sy := int(p[0])/2, int(p[1])/4
	x = sx + 3
	if x+w > ax+aw {
		x = sx - 2 - w
	}
	y = sy - h/2
	y = max(ay, min(ay+ah-h, y))
	return x, y, w, h
}

func (m *Model) drawCrewTip(c *Canvas, x, y, w int) {
	lines := m.crewTipLines()
	a := easeOutCubic((m.t - m.selAt) / 0.2)
	if m.selAt < 0 {
		a = 1
	}
	// A borderless chip: a border on a lighter fill shows half a cell of
	// fill outside the line.
	c.Fill(x, y, w, len(lines)+2, Mix(cPanel, cPill, a))
	for i, l := range lines {
		lx := x + 2
		for _, s := range l {
			st := s.st
			st.Fg = Mix(cPill, st.Fg, a)
			lx = c.Text(lx, y+1+i, s.s, st)
		}
	}
}
