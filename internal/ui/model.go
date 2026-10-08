// Package ui is the celebration itself: a night sky with one star for every
// person who starred tuios, and four views drawn over it.
package ui

import (
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/Gaurav-Gosain/tuios-5k/internal/gh"
)

type view int

const (
	vSky view = iota
	vHistory
	vCrew
	vThanks
	nViews
)

var viewNames = [nViews]string{"sky", "history", "crew", "thanks"}

// ViewByName maps a view name to its index, for the -view flag.
func ViewByName(s string) (int, bool) {
	for i, n := range viewNames {
		if n == s {
			return i, true
		}
	}
	return 0, false
}

const (
	minW, minH = 80, 24
	fps        = 30
	installCmd = "go install github.com/Gaurav-Gosain/tuios/cmd/tuios@latest"
	repoURL    = "https://github.com/Gaurav-Gosain/tuios"
)

type keyMap struct {
	Next, Prev, Jump, Left, Right, Ends, Shoot, Replay, Copy, Skip, Quit key.Binding
}

var keys = keyMap{
	Next:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next view")),
	Prev:   key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back")),
	Jump:   key.NewBinding(key.WithKeys("1", "2", "3", "4"), key.WithHelp("1-4", "views")),
	Left:   key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "back")),
	Right:  key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "on")),
	Ends:   key.NewBinding(key.WithKeys("home", "end", "g", "G")),
	Shoot:  key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "shooting star")),
	Replay: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "replay")),
	Copy:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy install")),
	Skip:   key.NewBinding(key.WithKeys("enter")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c", "esc"), key.WithHelp("q", "quit")),
}

// Options configure a run.
type Options struct {
	Snapshot *gh.Data
	Offline  bool // never touch the network
	View     int
}

// Model is the bubbletea model.
type Model struct {
	w, h    int
	t       float64 // seconds since start
	lastT   time.Time
	started time.Time

	snap   *gh.Data
	data   *gh.Data
	cum    []int
	status string // "", "live", "offline"
	inside bool   // running in a tuios pane

	view   view
	viewAt float64

	sky *sky

	introAt float64 // when the time-lapse started
	num     float64 // the number on the sky card

	cursor   int // history cursor day; -1 hides it
	cursorAt float64

	crew    []gh.Contributor
	sel     int
	selAt   float64
	copied  float64
	offline bool
	profile colorprofile.Profile
}

// New builds the model.
func New(o Options) *Model {
	m := &Model{
		snap:    o.Snapshot,
		sky:     newSky(),
		cursor:  -1,
		offline: o.Offline,
		view:    view(o.View),
		inside:  os.Getenv("TUIOS_PANE_ID") != "" || os.Getenv("TUIOS_SOCKET") != "",
		introAt: 0.5,
		viewAt:  -10,
		selAt:   -10,
		copied:  -10,
		profile: colorprofile.TrueColor,
	}
	m.setData(o.Snapshot)
	if o.Offline {
		m.status = "offline"
	}
	return m
}

func (m *Model) setData(d *gh.Data) {
	m.data = d
	m.cum = d.Cumulative()
	// Stars that the history cannot date yet still get a place in the sky.
	m.sky.ensure(max(d.Listed(), d.Stars))
	m.crew = crewOrder(d.Contributors)
	if m.sel >= len(m.crew) {
		m.sel = 0
	}
}

type tickMsg time.Time

type dataMsg struct {
	d   *gh.Data
	err error
}

func tick() tea.Cmd {
	return tea.Tick(time.Second/fps, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) fetch() tea.Cmd {
	snap := m.snap
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		d, err := (&gh.Client{}).Refresh(ctx, snap)
		return dataMsg{d, err}
	}
}

// Init starts the clock and, unless offline, the live fetch.
func (m *Model) Init() tea.Cmd {
	m.started = time.Now()
	m.lastT = m.started
	cmds := []tea.Cmd{tick()}
	if !m.offline {
		cmds = append(cmds, m.fetch())
	}
	return tea.Batch(cmds...)
}

// introLen is how long the time-lapse of the sky filling up takes.
const introLen = 4.2

// introDay is how far through the history the time-lapse is, in days.
func (m *Model) introDay() float64 {
	p := clamp((m.t-m.introAt)/introLen, 0, 1)
	// A slow start and a soft landing, but mostly linear, so the count
	// keeps the real rhythm of the history: two surges and a quiet middle.
	p = 0.15*easeInOutCubic(p) + 0.85*p
	return p * float64(len(m.cum))
}

func (m *Model) introDone() bool { return m.t >= m.introAt+introLen }

// shown is how many stars are in the sky right now.
func (m *Model) shown() int {
	if m.introDone() {
		return len(m.sky.stars)
	}
	d := m.introDay()
	i := int(d)
	if i <= 0 {
		return 0
	}
	if i >= len(m.cum) {
		return m.cum[len(m.cum)-1]
	}
	prev := m.cum[i-1]
	return prev + int(float64(m.cum[i]-prev)*(d-float64(i)))
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.ColorProfileMsg:
		m.profile = msg.Profile
		if m.profile < colorprofile.TrueColor {
			use256()
		}
	case tickMsg:
		now := time.Time(msg)
		dt := now.Sub(m.lastT).Seconds()
		m.lastT = now
		m.t = now.Sub(m.started).Seconds()
		m.step(dt)
		return m, tick()
	case dataMsg:
		if msg.d != nil {
			m.setData(msg.d)
		}
		if msg.err == nil || errors.Is(msg.err, gh.ErrNeedsAuth) {
			m.status = "live"
		} else {
			m.status = "offline"
		}
	case tea.MouseClickMsg:
		if msg.Y == 0 {
			for i, r := range tabRanges(m.w) {
				if msg.X >= r[0] && msg.X < r[1] {
					m.go2(view(i))
				}
			}
		}
	case tea.KeyPressMsg:
		return m, m.key(msg)
	}
	return m, nil
}

func (m *Model) step(dt float64) {
	m.sky.reveal(m.shown(), m.t)
	target := float64(m.data.Stars)
	if !m.introDone() {
		listed := float64(max(1, m.data.Listed()))
		m.num = float64(m.shown()) / listed * target
	} else {
		m.num += (target - m.num) * math.Min(1, dt*5)
		if math.Abs(target-m.num) < 0.5 {
			m.num = target
		}
	}
	// The camera pans a little per view, so the sky moves with parallax.
	camTarget := float64(m.view) * 0.3
	m.sky.cam += (camTarget - m.sky.cam) * math.Min(1, dt*2.2)
	// A shooting star now and then, unprompted.
	if m.introDone() && m.sky.rng.Float64() < dt/22 {
		m.sky.shoot(m.w, m.h, m.t)
	}
}

func (m *Model) go2(v view) {
	if v == m.view {
		return
	}
	m.view = v
	m.viewAt = m.t
}

func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Quit):
		return tea.Quit
	case key.Matches(msg, keys.Next):
		m.go2((m.view + 1) % nViews)
	case key.Matches(msg, keys.Prev):
		m.go2((m.view + nViews - 1) % nViews)
	case key.Matches(msg, keys.Jump):
		m.go2(view(msg.String()[0] - '1'))
	case key.Matches(msg, keys.Shoot):
		m.sky.shoot(m.w, m.h, m.t)
	case key.Matches(msg, keys.Skip):
		if !m.introDone() {
			m.introAt = m.t - introLen
		}
	case key.Matches(msg, keys.Replay):
		if m.view == vSky {
			m.replay()
		}
	case key.Matches(msg, keys.Copy):
		if m.view == vThanks {
			m.copied = m.t
			return tea.SetClipboard(installCmd)
		}
	case key.Matches(msg, keys.Left), key.Matches(msg, keys.Right), key.Matches(msg, keys.Ends):
		m.move(msg.String())
	}
	return nil
}

func (m *Model) replay() {
	m.introAt = m.t
	m.num = 0
	for i := range m.sky.appear {
		m.sky.appear[i] = math.Inf(1)
	}
}

func (m *Model) move(k string) {
	switch m.view {
	case vHistory:
		n := len(m.cum)
		step := max(1, n/m.plotDots())
		if m.cursor < 0 {
			m.cursor = n - 1
			if k == "right" || k == "l" {
				m.cursor = 0
			}
		} else {
			switch k {
			case "left", "h":
				m.cursor = max(0, m.cursor-step)
			case "right", "l":
				m.cursor = min(n-1, m.cursor+step)
			}
		}
		switch k {
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = n - 1
		}
		m.cursorAt = m.t
	case vCrew:
		n := len(m.crew)
		if n == 0 {
			return
		}
		switch k {
		case "left", "h":
			m.sel = (m.sel + n - 1) % n
		case "right", "l":
			m.sel = (m.sel + 1) % n
		case "home", "g":
			m.sel = 0
		case "end", "G":
			m.sel = n - 1
		}
		m.selAt = m.t
	}
}

// View draws one frame.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "tuios ✦ 5k"
	v.BackgroundColor = toRGBA(cNight)
	return v
}

func (m *Model) render() string {
	if m.w == 0 || m.h == 0 {
		return ""
	}
	c := NewCanvas(m.w, m.h, cNight)
	m.sky.draw(c, m.t, len(m.sky.stars), m.profile >= colorprofile.TrueColor)
	if m.w < minW || m.h < minH {
		m.drawTooSmall(c)
		return c.Render()
	}
	m.drawHeader(c)
	m.drawFooter(c)
	fade := 1 - easeOutCubic((m.t-m.viewAt)/0.45)
	var x, y, w, h int
	switch m.view {
	case vSky:
		x, y, w, h = m.drawSkyCard(c)
	case vHistory:
		x, y, w, h = m.drawHistory(c)
	case vCrew:
		x, y, w, h = m.drawCrew(c)
	case vThanks:
		x, y, w, h = m.drawThanks(c)
	}
	c.Fade(x, y, w, h, cPanel, fade)
	return c.Render()
}

// content is the area between the header and the footer, with a row of sky
// above and below.
func (m *Model) content() (x, y, w, h int) {
	return 0, 2, m.w, m.h - 4
}

// panel is the rectangle of a full panel view, centred, with sky around it.
func (m *Model) panel() (x, y, w, h int) {
	_, cy, cw, ch := m.content()
	w = min(cw-4, 136)
	if cw >= 120 {
		w = min(cw-16, 136)
	}
	h = min(ch, 42)
	return (cw - w) / 2, cy + (ch-h)/2, w, h
}

func tabRanges(w int) [][2]int {
	// Tabs are right-aligned: " n name " with one space between.
	total := 0
	for i, n := range viewNames {
		total += TextWidth(n) + 4
		if i > 0 {
			total++
		}
	}
	x := w - 2 - total
	out := make([][2]int, len(viewNames))
	for i, n := range viewNames {
		tw := TextWidth(n) + 4
		out[i] = [2]int{x, x + tw}
		x += tw + 1
	}
	return out
}

func (m *Model) drawHeader(c *Canvas) {
	x := c.Text(2, 0, "✦", Style{Fg: cGold})
	x = c.Text(x+1, 0, "tuios", Style{Fg: cText, Attr: attrBold})
	c.Text(x+1, 0, "5k", Style{Fg: cGold})
	for i, r := range tabRanges(m.w) {
		active := view(i) == m.view
		st := Style{Fg: cMuted}
		num := Style{Fg: cDim}
		if active {
			st = Style{Fg: cGoldHi, Bg: cPill, Attr: attrBold}
			num = Style{Fg: cGoldLo, Bg: cPill}
		}
		c.Text(r[0], 0, " ", st)
		c.Text(r[0]+1, 0, itoa(i+1), num)
		c.Text(r[0]+2, 0, " "+viewNames[i]+" ", st)
	}
}

type helpItem struct{ k, d string }

func (m *Model) helpItems() []helpItem {
	var items []helpItem
	add := func(b key.Binding) { items = append(items, helpItem{b.Help().Key, b.Help().Desc}) }
	switch m.view {
	case vSky:
		add(keys.Shoot)
		add(keys.Replay)
	case vHistory:
		items = append(items, helpItem{"←→", "scrub"})
		add(keys.Shoot)
	case vCrew:
		items = append(items, helpItem{"←→", "walk"})
		add(keys.Shoot)
	case vThanks:
		add(keys.Copy)
		add(keys.Shoot)
	}
	add(keys.Next)
	add(keys.Quit)
	return items
}

func (m *Model) drawFooter(c *Canvas) {
	y := m.h - 1
	// The status, right-aligned.
	var parts []struct {
		s  string
		st Style
	}
	add := func(s string, st Style) {
		parts = append(parts, struct {
			s  string
			st Style
		}{s, st})
	}
	if m.inside {
		add("✦ inside tuios", Style{Fg: cGold})
		add("   ", Style{})
	}
	switch m.status {
	case "live":
		add("● ", Style{Fg: cGreen})
		add("live", Style{Fg: cMuted})
	case "offline":
		add("offline snapshot", Style{Fg: cDim, Attr: attrItalic})
	default:
		add("○ ", Style{Fg: cDim})
		add("connecting", Style{Fg: cDim})
	}
	sw := 0
	for _, p := range parts {
		sw += TextWidth(p.s)
	}
	sx := m.w - 2 - sw
	x := sx
	for _, p := range parts {
		x = c.Text(x, y, p.s, p.st)
	}
	// Help, left-aligned, dropping items that would run into the status.
	x = 2
	for i, it := range m.helpItems() {
		w := TextWidth(it.k) + 1 + TextWidth(it.d)
		if i > 0 {
			w += 3
		}
		if x+w > sx-2 {
			break
		}
		if i > 0 {
			x = c.Text(x, y, " · ", Style{Fg: cFaint})
		}
		x = c.Text(x, y, it.k, Style{Fg: cText})
		x = c.Text(x+1, y, it.d, Style{Fg: cMuted})
	}
}

func (m *Model) drawTooSmall(c *Canvas) {
	lines := []string{"tuios ✦ 5k", "", "make the window at least 80×24", "it is " + itoa(m.w) + "×" + itoa(m.h) + " now"}
	y := m.h/2 - len(lines)/2
	for i, l := range lines {
		st := Style{Fg: cMuted}
		if i == 0 {
			st = Style{Fg: cGold, Attr: attrBold}
		}
		c.Text((m.w-TextWidth(l))/2, y+i, l, st)
	}
}

// date writes a day the way the app shows dates: 8 nov 2025.
func date(t time.Time) string {
	return strings.ToLower(t.Format("2 Jan 2006"))
}

func (m *Model) day(i int) time.Time {
	return m.data.StartDay().AddDate(0, 0, i)
}

func (m *Model) contributors() int { return len(m.crew) }

// center writes s centred in [x, x+w).
func center(c *Canvas, x, w, y int, s string, st Style) {
	c.Text(x+(w-TextWidth(s))/2, y, s, st)
}
