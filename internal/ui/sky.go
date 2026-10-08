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
	vx, vy float64 // dots per second
	born   float64
	life   float64
}

type sky struct {
	stars   []star
	appear  []float64 // when each star came out, in seconds of app time
	meteors []meteor
	cam     float64 // eased camera position in [0,1) of a screen
	rng     *rand.Rand
}

func newSky() *sky {
	return &sky{rng: rand.New(rand.NewPCG(uint64(rand.Int64()), 7))}
}

// ensure grows the star list to n.
func (s *sky) ensure(n int) {
	for i := len(s.stars); i < n; i++ {
		s.stars = append(s.stars, makeStar(i))
		s.appear = append(s.appear, math.Inf(1))
	}
}

// reveal marks stars up to n as out, at time now.
func (s *sky) reveal(n int, now float64) {
	s.ensure(n)
	for i := 0; i < n; i++ {
		if math.IsInf(s.appear[i], 1) {
			s.appear[i] = now
		}
	}
}

func (s *sky) shoot(w, h int, now float64) {
	wd, hd := float64(w*2), float64(h*4)
	dir := 1.0
	if s.rng.Float64() < 0.5 {
		dir = -1
	}
	ang := (14 + s.rng.Float64()*22) * math.Pi / 180
	speed := wd * (0.55 + 0.25*s.rng.Float64())
	x := wd * (0.1 + 0.8*s.rng.Float64())
	y := hd * (0.04 + 0.30*s.rng.Float64())
	s.meteors = append(s.meteors, meteor{
		x: x, y: y,
		vx: dir * speed * math.Cos(ang), vy: speed * math.Sin(ang),
		born: now, life: 0.9 + 0.5*s.rng.Float64(),
	})
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
	cHazeA = Hex("#151b38")
	cHazeB = Hex("#1d1634")
)

// drawHaze tints the background along the milky way. It is static apart
// from the camera, so it costs the terminal nothing once drawn.
func (s *sky) drawHaze(c *Canvas) {
	for y := 0; y < c.H; y++ {
		v := (float64(y) + 0.5) / float64(c.H)
		for x := 0; x < c.W; x++ {
			u := math.Mod((float64(x)+0.5)/float64(c.W)-s.cam*0.35*0.3+4, 1)
			d := v - bandV(u)
			d -= math.Round(d)
			g := math.Exp(-(d * d) / (2 * 0.085 * 0.085))
			if g < 0.03 {
				continue
			}
			n := noise(u*9, v*5, 9)*0.6 + noise(u*23, v*13, 23)*0.4
			k := g * (0.35 + 0.65*n)
			col := Mix(cHazeA, cHazeB, noise(u*3, v*2, 3))
			if cell := c.At(x, y); cell != nil {
				cell.Bg = Mix(cNight, col, math.Round(k*12)/12)
			}
		}
	}
}

// draw paints the sky over the whole canvas at time t.
func (s *sky) draw(c *Canvas, t float64, shown int, haze bool) {
	wd, hd := c.W*2, c.H*4
	if wd == 0 || hd == 0 {
		return
	}
	if haze {
		s.drawHaze(c)
	}
	// On a small terminal five thousand stars crowd into a few thousand
	// cells and the sky turns to static. Thin out the faint ones there:
	// every dot is still one person, though not every person gets a dot.
	keep := math.Min(1, 0.06*float64(wd*hd)/float64(max(1, len(s.stars))))
	var markers [][2]int
	var markerCol []Color
	for i := 0; i < shown && i < len(s.stars); i++ {
		st := &s.stars[i]
		if !st.marker && st.b < 0.3 && float64(splitmix(uint64(i)+7)%1000)/1000 >= keep {
			continue
		}
		x := math.Mod(st.u*float64(wd)+s.cam*float64(wd)*0.35*st.z, float64(wd))
		if x < 0 {
			x += float64(wd)
		}
		y := st.v * float64(hd)
		b := st.b * (1 - st.tw*0.5*(1+math.Sin(t*st.freq+st.phase)))
		// A star that just came out flares, then settles.
		if age := t - s.appear[i]; age >= 0 && age < 0.9 {
			f := 1 - age/0.9
			b = math.Max(b, 0.95*f*f)
		}
		if b < 0.5/starLevels {
			continue
		}
		col := starColor(st.col, b)
		if st.marker {
			markers = append(markers, [2]int{int(x) / 2, int(y) / 4})
			markerCol = append(markerCol, col)
			continue
		}
		c.Dot(int(x), int(y), col)
	}
	for k, m := range markers {
		cell := c.At(m[0], m[1])
		if cell != nil && (cell.R == ' ' || (cell.R >= 0x2800 && cell.R <= 0x28FF)) {
			cell.R = '✦'
			cell.Fg = markerCol[k]
		}
	}
	s.drawMeteors(c, t)
}

func (s *sky) drawMeteors(c *Canvas, t float64) {
	live := s.meteors[:0]
	for _, m := range s.meteors {
		age := t - m.born
		if age > m.life {
			continue
		}
		live = append(live, m)
		// Fade in fast, burn, fade out.
		a := math.Min(1, age/0.08) * math.Min(1, (m.life-age)/0.35)
		hx, hy := m.x+m.vx*age, m.y+m.vy*age
		sp := math.Hypot(m.vx, m.vy)
		ux, uy := m.vx/sp, m.vy/sp
		trail := math.Min(sp*age, float64(c.W)*0.45)
		for d := 0.0; d < trail; d += 0.7 {
			f := 1 - d/trail
			col := Mix(cNight, Mix(cBlue, cGoldHi, f), a*math.Pow(f, 1.4))
			c.Dot(int(hx-ux*d), int(hy-uy*d), col)
		}
		head := Mix(cNight, Hex("#ffffff"), a)
		c.Dot(int(hx), int(hy), head)
		c.Dot(int(hx), int(hy)+1, head)
	}
	s.meteors = live
}
