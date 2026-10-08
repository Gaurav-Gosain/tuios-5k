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
	c.Box(px, py, pw, ph, cBorder, cPanel, "crew", Style{Fg: cText})
	n := len(m.crew)
	if n == 0 {
		return px, py, pw, ph
	}
	tx := c.Text(px+3, py+1, itoa(n), Style{Fg: cGold, Attr: attrBold})
	c.Text(tx, py+1, " people with commits, joined in the order they arrived", Style{Fg: cMuted})

	// The sky area of the panel, leaving the top line and the bottom
	// line for text.
	ax, ay, aw, ah := px+2, py+3, pw-4, ph-6
	pts := crewPoints(n, ax, ay, aw, ah)

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
		col := Mix(cFaint, cBlueLo, 0.6)
		if k <= m.sel {
			col = Mix(cBlueLo, cBlue, 0.45)
		}
		if k == m.sel || k == m.sel+1 {
			col = cBlue
		}
		c.Line(int(a[0]), int(a[1]), int(ex), int(ey), col)
	}

	// Labels: the selected person first, then by commits, each placed
	// right of its star or left of it, skipped if it would collide.
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
	order := make([]int, 0, n)
	for k := range n {
		order = append(order, k)
	}
	slices.SortStableFunc(order, func(a, b int) int {
		if a == m.sel {
			return -1
		}
		if b == m.sel {
			return 1
		}
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
		if float64(k) > reached {
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
		st := Style{Fg: cDim}
		if m.crew[pl.k].Commits >= 10 {
			st = Style{Fg: cMuted}
		}
		if pl.k == m.sel {
			st = Style{Fg: cGoldHi, Attr: attrBold}
		}
		c.Set(pl.x-1, pl.y, ' ', Style{})
		e := c.Text(pl.x, pl.y, m.crew[pl.k].Login, st)
		c.Set(e, pl.y, ' ', Style{})
	}

	// The stars, sized by commits, with a twinkle.
	for k := range n {
		if float64(k) > reached {
			continue
		}
		p := pts[k]
		cm := m.crew[k].Commits
		g, base := '·', Mix(cPanel, cStarCool, 0.55)
		switch {
		case cm >= 100:
			g, base = '✦', cGoldHi
		case cm >= 10:
			g, base = '✦', cStarCool
		case cm >= 3:
			g, base = '+', Mix(cPanel, cStarCool, 0.8)
		case cm >= 2:
			g, base = '•', Mix(cPanel, cStarCool, 0.7)
		}
		tw := 0.5 + 0.5*math.Sin(m.t*(1.1+float64(k%5)*0.3)+float64(k))
		col := Mix(base, Mix(cPanel, base, 0.6), 0.5*tw)
		// A star flares as the line reaches it.
		if age := (reached - float64(k)) * per; age < 0.5 {
			col = Mix(col, Hex("#ffffff"), 1-age/0.5)
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

	// The person under the cursor, on the bottom line.
	who := m.crew[m.sel]
	by := py + ph - 2
	x := c.Text(px+3, by, who.Login, Style{Fg: cGoldHi, Attr: attrBold})
	commits := " commits"
	if who.Commits == 1 {
		commits = " commit"
	}
	x = c.Text(x+3, by, thousands(who.Commits), Style{Fg: cText})
	x = c.Text(x, by, commits, Style{Fg: cMuted})
	if t, err := time.Parse("2006-01-02", who.First); err == nil {
		x = c.Text(x, by, "   since ", Style{Fg: cMuted})
		x = c.Text(x, by, date(t), Style{Fg: cText})
	}
	pos := "#" + itoa(m.sel+1) + " of " + itoa(n)
	c.Text(px+pw-3-TextWidth(pos), by, pos, Style{Fg: cDim})
	_ = x
	return px, py, pw, ph
}
