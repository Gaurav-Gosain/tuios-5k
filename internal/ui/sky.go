package ui

import (
	"math"
	"math/rand/v2"
)

// The sky is the background layer: one dot for every person who starred the
// repo, placed by a hash of their position in the list, so star 1,234 sits in
// the same place on every run. A band of them forms a milky way. Each one has
// a depth, and the camera pans when the view changes, so near stars slide
// further than far ones.

type star struct {
	u, v   float64 // position at camera 0, both in [0,1)
	z      float64 // depth: 1 is near, 0.2 is far
	b      float64 // base brightness in [0,1]
	col    Color
	tw     float64 // twinkle depth
	freq   float64
	phase  float64
	marker bool // the 1st star and every 1000th
}

func splitmix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// makeStar builds star i. It depends on i alone.
func makeStar(i int) star {
	r := rand.New(rand.NewPCG(splitmix(uint64(i)), 0x5eed))
	s := star{freq: 0.25 + r.Float64()*0.9, phase: r.Float64() * 2 * math.Pi}
	s.u = r.Float64()
	if r.Float64() < 0.55 {
		// The band: a gentle diagonal with a slow wave in it.
		s.v = bandV(s.u) + r.NormFloat64()*0.07
		s.v = math.Mod(s.v+1, 1)
		s.b = 0.10 + 0.45*math.Pow(r.Float64(), 3)
	} else {
		s.v = r.Float64()
		s.b = 0.12 + 0.88*math.Pow(r.Float64(), 6)
	}
	s.z = 0.2 + 0.8*math.Pow(r.Float64(), 2)
	// Near stars read a little brighter.
	s.b = clamp(s.b*(0.75+0.35*s.z), 0, 1)
	switch k := r.Float64(); {
	case k < 0.18:
		s.col = cStarWarm
	case k < 0.40:
		s.col = cStarBlue
	default:
		s.col = cStarCool
	}
	if s.b > 0.4 {
		s.tw = 0.2 + 0.4*r.Float64()
	}
	s.marker = i == 0 || (i+1)%1000 == 0
	if s.marker {
		s.b = 1
		s.col = cGoldHi
		s.tw = 0.3
		s.freq = 0.5
		s.z = 0.9
	}
	return s
}

type meteor struct {
	x, y   float64 // start, in dots
	ux, uy float64 // unit direction
	dist   float64 // how far the head travels, in dots
	trail  float64 // the longest the trail gets, in dots
	born   float64
	life   float64
}

// flareLen is how long a star that just came out stays brighter than its base.
const flareLen = 0.9

// maxFlares caps the stars flaring at once. Past it, a star comes out at its
// base brightness: the eye cannot follow more, and every flare repaints a cell.
const maxFlares = 150

type sky struct {
	stars   []star
	appear  []float64 // when each star came out, in seconds of app time
	flaring []float64 // when the flares still running started
	meteors []meteor
	rng     *rand.Rand

	// The camera eases from one position to the next.
	camFrom, camTo, camAt, camDur float64

	you      int     // the viewer's own star, or -1
	youAt    float64 // when it was found
	ringAt   float64 // when the 5,000th star rang, or -inf
	hazeKey  [3]int
	hazeGrid []Color
}

func newSky() *sky {
	return &sky{rng: rand.New(rand.NewPCG(uint64(rand.Int64()), 7)), you: -1, ringAt: math.Inf(-1), camDur: 0.6}
}

// ensure grows the star list to n.
func (s *sky) ensure(n int) {
	for i := len(s.stars); i < n; i++ {
		s.stars = append(s.stars, makeStar(i))
		s.appear = append(s.appear, math.Inf(1))
	}
}

// reveal marks stars up to n as out, at time now. When many come out in one
// frame, only an even spread of them flares.
func (s *sky) reveal(n int, now float64) {
	s.ensure(n)
	live := s.flaring[:0]
	for _, t := range s.flaring {
		if now-t < flareLen {
			live = append(live, t)
		}
	}
	s.flaring = live
	var fresh []int
	for i := 0; i < n; i++ {
		if math.IsInf(s.appear[i], 1) {
			fresh = append(fresh, i)
		}
	}
	if len(fresh) == 0 {
		return
	}
	budget := max(0, maxFlares-len(s.flaring))
	stride := len(fresh)
	if budget > 0 {
		stride = (len(fresh) + budget - 1) / budget
	}
	for k, i := range fresh {
		if budget > 0 && k%stride == 0 || s.stars[i].marker {
			s.appear[i] = now
			s.flaring = append(s.flaring, now)
		} else {
			s.appear[i] = math.Inf(-1)
		}
	}
}

// pan eases the camera to target, over dur seconds.
func (s *sky) pan(target, now, dur float64) {
	if target == s.camTo {
		return
	}
	s.camFrom = s.camAt2(now)
	s.camTo = target
	s.camAt = now
	s.camDur = dur
}

// camAt2 is the camera position at time t.
func (s *sky) camAt2(t float64) float64 {
	p := easeInOutCubic((t - s.camAt) / s.camDur)
	return lerp(s.camFrom, s.camTo, p)
}

// moving reports whether the camera is still easing.
func (s *sky) moving(t float64) bool { return t-s.camAt < s.camDur }

// shoot sends a shooting star. top is the first row the current panel
// covers: when there is room above it, the meteor stays in that band of
// open sky, so it is seen and not lost behind the card.
func (s *sky) shoot(w, h, top int, now float64) {
	wd, hd := float64(w*2), float64(h*4)
	dir := 1.0
	if s.rng.Float64() < 0.5 {
		dir = -1
	}
	trail := 2 * (18 + 10*s.rng.Float64())
	m := meteor{trail: trail, born: now, life: 0.7 + 0.3*s.rng.Float64()}
	band := float64(top-1) * 4 // dots of open sky under the header
	if band >= 16 {
		ang := (6 + s.rng.Float64()*8) * math.Pi / 180
		m.y = 4 + band*(0.1+0.3*s.rng.Float64())
		m.dist = math.Min(trail*2.4, (4+band-2-m.y)/math.Sin(ang))
		m.ux, m.uy = dir*math.Cos(ang), math.Sin(ang)
	} else {
		ang := (14 + s.rng.Float64()*20) * math.Pi / 180
		m.y = hd * (0.05 + 0.2*s.rng.Float64())
		m.dist = math.Min(trail*2.2, (hd*0.4-m.y)/math.Sin(ang))
		m.ux, m.uy = dir*math.Cos(ang), math.Sin(ang)
	}
	// Start where the whole flight stays on screen.
	span := m.dist * math.Abs(m.ux)
	lo, hi := 0.05*wd, 0.95*wd-span
	if dir < 0 {
		lo, hi = 0.05*wd+span, 0.95*wd
	}
	m.x = lo + (hi-lo)*s.rng.Float64()
	if hi < lo {
		m.x = wd / 2
	}
	s.meteors = append(s.meteors, m)
}

// starLevels is how many brightness steps a star can show. Few steps keep
// the twinkle calm and keep the terminal from repainting every cell on every
// frame: a cell changes only when a star crosses a step.
const starLevels = 7

func starColor(base Color, b float64) Color {
	q := math.Round(clamp(b, 0, 1)*starLevels) / starLevels
	if lowColor {
		// Faint tints land on odd cube colours; greys step evenly.
		return Mix(cNight, Hex("#eeeeee"), q)
	}
	return Mix(cNight, base, q)
}

// bandV is the centre line of the milky way at u. It is periodic in u, so
// the band has no seam where the sky wraps round.
func bandV(u float64) float64 {
	return 0.5 + 0.27*math.Sin(2*math.Pi*u+2.2) + 0.05*math.Sin(4*math.Pi*u+0.5)
}

// noise is smooth value noise in [0,1], for the clumps in the haze. It
// repeats every period along x, which must be a whole number, to match the
// wrap of the sky.
func noise(x, y, period float64) float64 {
	xi, yi := math.Floor(x), math.Floor(y)
	fx, fy := x-xi, y-yi
	h := func(i, j float64) float64 {
		i = math.Mod(math.Mod(i, period)+period, period)
		return float64(splitmix(uint64(int64(i)*73856093^int64(j)*19349663))%1024) / 1023
	}
	sx, sy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a := h(xi, yi) + (h(xi+1, yi)-h(xi, yi))*sx
	b := h(xi, yi+1) + (h(xi+1, yi+1)-h(xi, yi+1))*sx
	return a + (b-a)*sy
}

var (
	cHazeA = Hex("#1a2246")
	cHazeB = Hex("#251b40")
)

// hazeParallax is how far the haze moves with the camera: it is the
// farthest layer.
const hazeParallax = 0.15

// drawHaze tints the background along the milky way. The grid is kept until
// the size or the camera's whole-column offset changes, and the terminal
// repaints none of it in between.
func (s *sky) drawHaze(c *Canvas, cam float64) {
	shift := int(math.Round(cam * hazeParallax * float64(c.W)))
	key := [3]int{c.W, c.H, shift}
	if key != s.hazeKey || s.hazeGrid == nil {
		s.hazeKey = key
		s.hazeGrid = make([]Color, c.W*c.H)
		for y := 0; y < c.H; y++ {
			v := (float64(y) + 0.5) / float64(c.H)
			for x := 0; x < c.W; x++ {
				u := math.Mod((float64(x-shift)+0.5)/float64(c.W)+8, 1)
				d := v - bandV(u)
				d -= math.Round(d)
				g := math.Exp(-(d * d) / (2 * 0.12 * 0.12))
				if g < 0.03 {
					continue
				}
				n := noise(u*9, v*5, 9)
				k := 0.5 * g * (0.4 + 0.6*n)
				col := Mix(cHazeA, cHazeB, noise(u*3, v*2, 3))
				s.hazeGrid[y*c.W+x] = Mix(cNight, col, math.Round(k*24)/24)
			}
		}
	}
	for i, col := range s.hazeGrid {
		if col.Set {
			c.cells[i].Bg = col
		}
	}
}

// skyOpts are the per-frame settings of the sky.
type skyOpts struct {
	t      float64
	shown  int
	haze   bool
	dim    float64 // 0 is full brightness, 1 is gone
	youRow [2]int  // the rows the viewer's star may sit in
}

// skyHits are where things landed, for the views to label.
type skyHits struct {
	you      [2]int // cell of the viewer's star; x is -1 when none
	fivek    [2]int // cell of the 5,000th star; x is -1 when not out
	ringDots int
}

// pos is where star i is drawn, in dots, with the camera at cam.
func (s *sky) pos(i int, cam float64, wd, hd int) (float64, float64) {
	st := &s.stars[i]
	x := math.Mod(st.u*float64(wd)+cam*float64(wd)*st.z, float64(wd))
	if x < 0 {
		x += float64(wd)
	}
	return x, st.v * float64(hd)
}

// camFor is the camera position closest to near that puts star i at dot
// column x.
func (s *sky) camFor(i int, x float64, wd int, near float64) float64 {
	st := &s.stars[i]
	period := 1 / st.z
	base := (x/float64(wd) - st.u) / st.z
	k := math.Round((near - base) / period)
	return base + k*period
}

// draw paints the sky. Rows 0 and h-1 belong to the chrome, so the sky
// stays out of them.
func (s *sky) draw(c *Canvas, o skyOpts) skyHits {
	hits := skyHits{you: [2]int{-1, -1}, fivek: [2]int{-1, -1}}
	wd, hd := c.W*2, c.H*4
	if wd == 0 || hd == 0 {
		return hits
	}
	cam := s.camAt2(o.t)
	if o.haze && o.dim < 0.5 {
		s.drawHaze(c, cam)
	}
	c.Clip(0, 1, c.W, c.H-2)
	defer c.Unclip()

	// On a small terminal five thousand stars crowd into a few thousand
	// cells. Thin out the faint ones there: every dot is still one person,
	// though not every person gets a dot.
	keep := math.Min(1, 0.06*float64(wd*hd)/float64(max(1, len(s.stars))))
	// One dot per cell, or per two cells on a narrow screen: a cell with
	// three dots in it reads as static. The brightest star wins the cell.
	pair := 1
	if c.W < 100 {
		pair = 2
	}
	type slot struct {
		bit rune
		col Color
		b   float64
	}
	gw := (c.W + pair - 1) / pair
	grid := make([]slot, gw*c.H)
	var markers [][2]int
	var markerCol []Color
	for i := 0; i < o.shown && i < len(s.stars); i++ {
		st := &s.stars[i]
		if i == s.you {
			continue
		}
		if !st.marker && st.b < 0.3 && float64(splitmix(uint64(i)+7)%1000)/1000 >= keep {
			continue
		}
		x, y := s.pos(i, cam, wd, hd)
		b := st.b * (1 - st.tw*0.5*(1+math.Sin(o.t*st.freq+st.phase)))
		// A star that just came out flares, then settles, in three steps.
		if age := o.t - s.appear[i]; age >= 0 && age < flareLen {
			f := math.Ceil((1-age/flareLen)*3) / 3
			b = math.Max(b, 0.95*f*f)
		}
		b *= 1 - o.dim
		if b < 0.5/starLevels {
			continue
		}
		col := starColor(st.col, b)
		cx, cy := int(x)/2, int(y)/4
		if st.marker {
			markers = append(markers, [2]int{cx, cy})
			markerCol = append(markerCol, col)
			if i == 4999 {
				hits.fivek = [2]int{cx, cy}
			}
			continue
		}
		k := cy*gw + cx/pair
		if k < 0 || k >= len(grid) {
			continue
		}
		if b > grid[k].b {
			grid[k] = slot{brailleBit[int(y)%4][int(x)%2], col, b}
			if pair == 2 {
				// Keep the dot in the cell it fell in.
				grid[k].bit |= rune(cx%2) << 16
			}
		}
	}
	for k, sl := range grid {
		if sl.b == 0 {
			continue
		}
		cy := k / gw
		cx := (k % gw) * pair
		if pair == 2 {
			cx += int(sl.bit >> 16)
		}
		cell := c.At(cx, cy)
		if cell != nil && cell.R == ' ' {
			cell.R = 0x2800 | (sl.bit & 0xFF)
			cell.Fg = sl.col
			cell.Attr = 0
		}
	}
	for k, m := range markers {
		cell := c.At(m[0], m[1])
		if cell != nil && (cell.R == ' ' || (cell.R >= 0x2800 && cell.R <= 0x28FF)) {
			cell.R = '✦'
			cell.Fg = markerCol[k]
		}
	}
	// The ring round the 5,000th star: eight dots that grow and fade.
	if age := o.t - s.ringAt; age >= 0 && age < 0.6 && hits.fivek[0] >= 0 {
		x, y := s.pos(4999, cam, wd, hd)
		p := easeOutCubic(age / 0.6)
		r := 2 + 9*p
		col := Mix(cNight, cGoldHi, 1-p*p)
		for k := range 8 {
			a := float64(k) * math.Pi / 4
			c.Dot(int(x+r*math.Cos(a)), int(y+r*math.Sin(a)*0.9), col)
		}
		hits.ringDots = 8
	}
	if s.you >= 0 && s.you < len(s.stars) && s.you < o.shown {
		hits.you = s.drawYou(c, cam, wd, hd, o)
	}
	s.drawMeteors(c, o.t)
	return hits
}

// drawYou draws the viewer's star: gold, in a soft ring of braille dots
// that breathes. The neighbouring cells are cleared first, so the ring
// reads cleanly.
func (s *sky) drawYou(c *Canvas, cam float64, wd, hd int, o skyOpts) [2]int {
	x, y := s.pos(s.you, cam, wd, hd)
	cx, cy := int(x)/2, int(y)/4
	cy = max(o.youRow[0], min(o.youRow[1], cy))
	age := o.t - s.youAt
	in := easeOutCubic(age / 0.8)
	pulse := 0.5 + 0.5*math.Sin(age*2.4)
	for dy := -1; dy <= 1; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if cell := c.At(cx+dx, cy+dy); cell != nil && (cell.R >= 0x2800 && cell.R <= 0x28FF || cell.R == '✦') {
				cell.R = ' '
			}
		}
	}
	// The ring, in dot space round the centre of the cell.
	px, py := float64(cx*2)+1, float64(cy*4)+1.5
	r := 3.2 + 0.4*pulse
	ring := Mix(cNight, cGoldLo, in*(0.45+0.25*pulse))
	for k := range 14 {
		a := float64(k) / 14 * 2 * math.Pi
		dx, dy := px+r*math.Cos(a), py+r*0.8*math.Sin(a)
		if int(dx)/2 == cx && int(dy)/4 == cy {
			continue
		}
		c.Dot(int(math.Round(dx)), int(math.Round(dy)), ring)
	}
	if cell := c.At(cx, cy); cell != nil {
		cell.R = '✦'
		cell.Fg = Mix(cNight, Mix(cGold, cGoldHi, pulse), in)
		cell.Attr = attrBold
	}
	return [2]int{cx, cy}
}

func (s *sky) drawMeteors(c *Canvas, t float64) {
	live := s.meteors[:0]
	for _, m := range s.meteors {
		age := t - m.born
		if age > m.life+0.35 {
			continue
		}
		live = append(live, m)
		p := clamp(age/m.life, 0, 1)
		// Fast out of the gate, slowing as it burns up.
		travel := m.dist * (1 - (1-p)*(1-p))
		hx, hy := m.x+m.ux*travel, m.y+m.uy*travel
		if age <= m.life {
			a := math.Min(1, age/0.06) * math.Min(1, (m.life-age)/0.3+0.15)
			trail := math.Min(travel, m.trail) * (1 - 0.5*p*p)
			// The normal, for the thick front of the trail.
			nx, ny := -m.uy, m.ux
			for d := 0.0; d < trail; d += 0.5 {
				f := 1 - d/trail
				col := Mix(cNight, Mix(cBlue, cGoldHi, f*f), a*(0.3+0.7*math.Pow(f, 0.8)))
				px, py := hx-m.ux*d, hy-m.uy*d
				c.Dot(int(px), int(py), col)
				if d < trail*0.3 {
					c.Dot(int(px+nx), int(py+ny), col)
				}
			}
			head := Mix(cNight, cWhite, a)
			for dy := range 2 {
				for dx := range 2 {
					c.Dot(int(hx)+dx, int(hy)+dy, head)
				}
			}
			continue
		}
		// A tiny sparkle where it burned out.
		f := 1 - (age-m.life)/0.35
		col := Mix(cNight, cGoldHi, f)
		c.Dot(int(hx)+2, int(hy), col)
		c.Dot(int(hx)-1, int(hy)-2, col)
		c.Dot(int(hx), int(hy)+2, col)
	}
	s.meteors = live
}

// busy reports whether anything in the sky needs a fast frame rate.
func (s *sky) busy(t float64) bool {
	return len(s.meteors) > 0 || s.moving(t) || len(s.flaring) > 0 || t-s.ringAt < 0.6 || t-s.youAt < 0.8
}
