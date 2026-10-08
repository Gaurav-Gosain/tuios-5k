// Package ui is the celebration itself: a night sky with one star for every
// person who starred tuios, and four views drawn over it.
package ui

import (
	"context"
	"errors"
	"hash/fnv"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
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
	installCmd = "go install github.com/Gaurav-Gosain/tuios/cmd/tuios@latest"
	repoURL    = "https://github.com/Gaurav-Gosain/tuios"
	repoShort  = "github.com/Gaurav-Gosain/tuios"
)

type keyMap struct {
	Next, Prev, Jump, Left, Right, Week, Marks, Ends, Shoot, Find, Replay, Copy, Skip, Help, Quit key.Binding
}

var keys = keyMap{
	Next:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next view")),
	Prev:   key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous view")),
	Jump:   key.NewBinding(key.WithKeys("1", "2", "3", "4"), key.WithHelp("1-4", "go to a view")),
	Left:   key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "back")),
	Right:  key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "on")),
	Week:   key.NewBinding(key.WithKeys("shift+left", "shift+right", "H", "L"), key.WithHelp("⇧←→", "a week")),
	Marks:  key.NewBinding(key.WithKeys("[", "]"), key.WithHelp("[ ]", "marks")),
	Ends:   key.NewBinding(key.WithKeys("home", "end", "g", "G")),
	Shoot:  key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "shooting star")),
	Find:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "find your star")),
	Replay: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "replay")),
	Copy:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy install")),
	Skip:   key.NewBinding(key.WithKeys("enter")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "keys")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c", "esc"), key.WithHelp("q", "quit")),
}

// Options configure a run.
type Options struct {
	Snapshot *gh.Data
	Offline  bool // never touch the network
	View     int
	Me       string // a GitHub login to find in the sky once it has filled
}

type findState int

const (
	findIdle findState = iota
	findTyping
	findLoading
	findFound
	findMissing
	findFailed
)

// Model is the bubbletea model.
type Model struct {
	w, h    int
	t       float64 // seconds since start
	started time.Time

	snap   *gh.Data
	data   *gh.Data
	cum    []int
	conn   string // "", "live", "offline"
	inside bool   // running in a tuios pane

	view   view
	viewAt float64

	sky *sky

	introAt float64 // when the time-lapse started
	num     float64 // the number on the sky card
	landed  float64 // when the count reached the live number

	cursor   int // history cursor day; -1 hides it
	cursorAt float64
	hist     plotGeom

	crew      []gh.Contributor
	crewPts   [][2]float64
	sel       int
	selFrom   int
	selAt     float64
	copied    float64
	offline   bool
	help      bool
	profile   colorprofile.Profile
	fastUntil float64
	panelTop  int // the first row the current panel covers, from the last frame

	// Find your star.
	input    textinput.Model
	find     findState
	findAt   float64
	me       string   // the login that was found or looked for
	pendMe   string   // -me, asked for at the start
	early    *findMsg // its answer, held until the sky has filled
	youDay   int
	youIdx   int
	youOf    int
	findNote string
}

// New builds the model.
func New(o Options) *Model {
	ti := textinput.New()
	ti.CharLimit = 39
	ti.Prompt = ""
	ti.Placeholder = ""
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
		landed:  math.Inf(1),
		profile: colorprofile.TrueColor,
		input:   ti,
		pendMe:  o.Me,
		youIdx:  -1,
	}
	m.setData(o.Snapshot)
	if o.Offline {
		m.conn = "offline"
	}
	m.sky.pan(m.camFor(m.view), -10, 0.01)
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

type findMsg struct {
	login string
	at    time.Time
	found bool
	err   error
}

func tick(fps int) tea.Cmd {
	return tea.Tick(time.Second/time.Duration(fps), func(t time.Time) tea.Msg { return tickMsg(t) })
}

func client() *gh.Client {
	return &gh.Client{Token: os.Getenv("TUIOS_5K_TOKEN")}
}

func (m *Model) fetch() tea.Cmd {
	snap := m.snap
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		d, err := client().Refresh(ctx, snap)
		return dataMsg{d, err}
	}
}

func (m *Model) findCmd(login string) tea.Cmd {
	since := m.data.StartDay()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		at, ok, err := client().FindStar(ctx, login, since)
		return findMsg{login, at, ok, err}
	}
}

// Init starts the clock and, unless offline, the live fetch.
func (m *Model) Init() tea.Cmd {
	m.started = time.Now()
	cmds := []tea.Cmd{tick(30)}
	if !m.offline {
		cmds = append(cmds, m.fetch())
	}
	// -me asks GitHub at once, and the answer waits for the sky to fill.
	if m.pendMe != "" {
		cmds = append(cmds, m.submit(m.pendMe))
	}
	return tea.Batch(cmds...)
}

// The time-lapse: the count runs through the history in introLen seconds,
// holds on 5,000 for holdLen, then finishes.
const (
	introLen = 5.5
	holdLen  = 0.45
)

// cumAt is the running total at fractional day d.
func (m *Model) cumAt(d float64) float64 {
	n := len(m.cum)
	if n == 0 || d <= 0 {
		return 0
	}
	if d >= float64(n) {
		return float64(m.cum[n-1])
	}
	i := int(d)
	prev := 0
	if i > 0 {
		prev = m.cum[i-1]
	}
	return float64(prev) + float64(m.cum[i]-prev)*(d-float64(i))
}

// progress maps a day to how far through the time-lapse it is. Half of it
// is the calendar and half is the stars, so the quiet middle still passes as
// dates running fast, and the surges get room to land.
func (m *Model) progress(d float64) float64 {
	n := float64(max(1, len(m.cum)))
	total := math.Max(1, m.cumAt(n))
	return 0.5*d/n + 0.5*m.cumAt(d)/total
}

// dayAt inverts progress.
func (m *Model) dayAt(p float64) float64 {
	lo, hi := 0.0, float64(len(m.cum))
	for range 32 {
		mid := (lo + hi) / 2
		if m.progress(mid) < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

func introEase(u float64) float64 {
	u = clamp(u, 0, 1)
	return 0.25*easeInOutCubic(u) + 0.75*u
}

// beat is the time into the time-lapse at which the count reaches 5,000,
// and the fractional day it happens on. It is -1 when there is no 5,000.
func (m *Model) beat() (float64, float64) {
	n := len(m.cum)
	if n == 0 || m.cum[n-1] < 5000 {
		return -1, 0
	}
	i := dayOf(m.cum, 5000)
	prev := 0
	if i > 0 {
		prev = m.cum[i-1]
	}
	d := float64(i) + float64(5000-prev)/float64(max(1, m.cum[i]-prev))
	p := m.progress(d)
	lo, hi := 0.0, 1.0
	for range 32 {
		mid := (lo + hi) / 2
		if introEase(mid) < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2 * introLen, d
}

// introDay is how far through the history the time-lapse is, in days.
func (m *Model) introDay() float64 {
	tl := m.t - m.introAt
	if tl <= 0 {
		return 0
	}
	if b, d := m.beat(); b >= 0 {
		switch {
		case tl < b:
		case tl < b+holdLen:
			return d
		default:
			tl -= holdLen
		}
	}
	return m.dayAt(introEase(tl / introLen))
}

// holding reports whether the count is resting on 5,000.
func (m *Model) holding() bool {
	b, _ := m.beat()
	tl := m.t - m.introAt
	return b >= 0 && tl >= b && tl < b+holdLen+0.5
}

func (m *Model) introLength() float64 {
	if b, _ := m.beat(); b >= 0 {
		return introLen + holdLen
	}
	return introLen
}

func (m *Model) introDone() bool { return m.t >= m.introAt+m.introLength() }

// shown is how many stars are in the sky right now.
func (m *Model) shown() int {
	if m.introDone() {
		return len(m.sky.stars)
	}
	return int(m.cumAt(m.introDay()))
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.sky.pan(m.camFor(m.view), m.t, 0.01)
	case tea.ColorProfileMsg:
		m.profile = msg.Profile
		if m.profile < colorprofile.TrueColor {
			use256()
		}
	case tickMsg:
		m.t = time.Time(msg).Sub(m.started).Seconds()
		cmd := m.step()
		return m, tea.Batch(tick(m.fps()), cmd)
	case dataMsg:
		if msg.d != nil {
			m.setData(msg.d)
		}
		if msg.err == nil || errors.Is(msg.err, gh.ErrNeedsAuth) {
			m.conn = "live"
		} else {
			m.conn = "offline"
		}
	case findMsg:
		m.found(msg)
	case tea.MouseClickMsg:
		m.click(msg.X, msg.Y, msg.Button)
	case tea.MouseMotionMsg:
		if msg.Button == tea.MouseLeft {
			m.drag(msg.X, msg.Y)
		}
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp, tea.MouseWheelLeft:
			m.move("left")
		case tea.MouseWheelDown, tea.MouseWheelRight:
			m.move("right")
		}
	case tea.KeyPressMsg:
		return m, m.key(msg)
	}
	return m, nil
}

// fps is the frame rate the next tick asks for: 30 while something moves,
// 20 while the count runs, 10 while only the twinkle does.
func (m *Model) fps() int {
	switch {
	case m.t < m.fastUntil || m.sky.busy(m.t) || m.t-m.viewAt < 0.5 || m.t-m.selAt < 0.25:
		return 30
	case !m.introDone() || m.find == findLoading:
		return 20
	case m.view == vHistory && m.t-m.viewAt < 2:
		return 30
	case m.view == vCrew && m.t-m.viewAt < 2.6:
		return 30
	case m.glinting():
		return 20
	}
	return 10
}

func (m *Model) step() tea.Cmd {
	m.sky.reveal(m.shown(), m.t)
	if b, _ := m.beat(); b >= 0 && math.IsInf(m.sky.ringAt, -1) && m.t-m.introAt >= b {
		m.sky.ringAt = m.t
	}
	target := float64(m.data.Stars)
	if !m.introDone() {
		m.num = m.cumAt(m.introDay())
	} else {
		if math.IsInf(m.landed, 1) && math.Abs(target-m.num) < 0.5 {
			m.landed = m.t
		}
		m.num += (target - m.num) * 0.25
		if math.Abs(target-m.num) < 0.5 {
			m.num = target
		}
	}
	// A shooting star now and then, unprompted.
	if m.introDone() && m.sky.rng.Float64() < 1.0/float64(m.fps())/25 {
		m.sky.shoot(m.w, m.h, m.panelTop, m.t)
	}
	if m.early != nil && m.introDone() && m.t > m.introAt+m.introLength()+0.9 {
		msg := *m.early
		m.early = nil
		m.found(msg)
	}
	return nil
}

// camFor is where the camera rests on a view. On the sky view with a found
// star, it rests where that star sits in open sky beside the card.
func (m *Model) camFor(v view) float64 {
	base := 0.4 * float64(v)
	if v != vSky || m.youIdx < 0 || m.w == 0 {
		return base
	}
	x, _, w, _ := m.skyCardRect()
	left, right := x, m.w-(x+w)
	col := float64(x+w) + float64(right)*0.45
	if left > right {
		col = float64(left) * 0.55
	}
	return m.sky.camFor(m.youIdx, col*2+1, m.w*2, m.sky.camTo)
}

func (m *Model) go2(v view) {
	if v == m.view {
		return
	}
	m.view = v
	m.viewAt = m.t
	m.sky.pan(m.camFor(v), m.t, 0.6)
}

func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	if m.find == findTyping {
		switch msg.String() {
		case "enter":
			v := strings.TrimSpace(strings.TrimPrefix(m.input.Value(), "@"))
			if !gh.ValidLogin(v) {
				m.findNote = "type a github login"
				m.findAt = m.t
				return nil
			}
			return m.submit(v)
		case "esc", "ctrl+c":
			m.find = findIdle
			m.input.Blur()
			return nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.findNote = ""
		return cmd
	}
	if m.help {
		m.help = false
		if key.Matches(msg, keys.Quit) && msg.String() != "q" {
			return nil
		}
	}
	switch {
	case key.Matches(msg, keys.Quit):
		return tea.Quit
	case key.Matches(msg, keys.Help):
		m.help = true
	case key.Matches(msg, keys.Next):
		m.go2((m.view + 1) % nViews)
	case key.Matches(msg, keys.Prev):
		m.go2((m.view + nViews - 1) % nViews)
	case key.Matches(msg, keys.Jump):
		m.go2(view(msg.String()[0] - '1'))
	case key.Matches(msg, keys.Shoot):
		m.sky.shoot(m.w, m.h, m.panelTop, m.t)
	case key.Matches(msg, keys.Find):
		return m.openFind()
	case key.Matches(msg, keys.Skip):
		if !m.introDone() {
			m.introAt = m.t - m.introLength()
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
	case key.Matches(msg, keys.Week):
		if msg.String() == "shift+left" || msg.String() == "H" {
			m.move("week-")
		} else {
			m.move("week+")
		}
	case key.Matches(msg, keys.Marks):
		m.move(msg.String())
	case key.Matches(msg, keys.Left), key.Matches(msg, keys.Right), key.Matches(msg, keys.Ends):
		m.move(msg.String())
	}
	return nil
}

func (m *Model) openFind() tea.Cmd {
	if m.find == findLoading {
		return nil
	}
	m.find = findTyping
	m.findNote = ""
	if m.input.Value() == "" {
		if m.me != "" {
			m.input.SetValue(m.me)
		} else {
			m.input.SetValue(gitHubUser())
		}
	}
	m.input.CursorEnd()
	return m.input.Focus()
}

// gitHubUser is the login in `git config github.user`, when it is set.
func gitHubUser() string {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "config", "--get", "github.user").Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	if !gh.ValidLogin(s) {
		return ""
	}
	return s
}

func (m *Model) submit(login string) tea.Cmd {
	m.input.Blur()
	m.me = login
	m.findAt = m.t
	if m.offline {
		m.find = findFailed
		m.findNote = "find needs github. run it without -offline."
		return nil
	}
	m.find = findLoading
	m.findNote = ""
	return m.findCmd(login)
}

func (m *Model) found(msg findMsg) {
	if m.pendMe != "" && msg.login == m.pendMe && !m.introDone() {
		m.pendMe = ""
		m.early = &msg
		return
	}
	m.pendMe = ""
	m.findAt = m.t
	switch {
	case msg.err != nil:
		m.find = findFailed
		switch {
		case errors.Is(msg.err, gh.ErrNoUser):
			m.findNote = "github has no user " + msg.login + "."
		case errors.Is(msg.err, gh.ErrRateLimited):
			m.findNote = "github says wait an hour. try again later."
		default:
			m.findNote = "github did not answer. try again later."
		}
		return
	case !msg.found:
		m.find = findMissing
		m.youIdx = -1
		m.sky.you = -1
		m.go2(vSky)
		return
	}
	m.find = findFound
	start := m.data.StartDay()
	day := int(msg.at.UTC().Sub(start).Hours() / 24)
	day = max(0, day)
	h := fnv.New32a()
	h.Write([]byte(strings.ToLower(msg.login)))
	n := len(m.cum)
	switch {
	case day < n && m.data.Daily[day] > 0:
		prev := 0
		if day > 0 {
			prev = m.cum[day-1]
		}
		m.youIdx = prev + int(h.Sum32()%uint32(m.data.Daily[day]))
		m.youOf = m.data.Daily[day]
	default:
		// Starred after the history ends: the newest place in the sky.
		m.youIdx = max(0, len(m.sky.stars)-1)
		m.youOf = 0
	}
	m.youDay = day
	m.sky.ensure(m.youIdx + 1)
	m.sky.you = m.youIdx
	m.sky.youAt = m.t
	if m.view != vSky {
		m.view = vSky
		m.viewAt = m.t
	}
	m.sky.pan(m.camFor(vSky), m.t, 1.6)
}

func (m *Model) replay() {
	m.introAt = m.t
	m.num = 0
	m.landed = math.Inf(1)
	m.sky.ringAt = math.Inf(-1)
	for i := range m.sky.appear {
		m.sky.appear[i] = math.Inf(1)
	}
}

// marks are the days [ and ] jump between: the 1k and 5k marks and the
// three best days.
func (m *Model) marks() []int {
	var out []int
	for _, v := range []int{1000, 5000} {
		if d := dayOf(m.cum, v); d >= 0 {
			out = append(out, d)
		}
	}
	out = append(out, m.bestDays(3)...)
	slicesSortUnique(&out)
	return out
}

func (m *Model) move(k string) {
	switch m.view {
	case vHistory:
		n := len(m.cum)
		if n == 0 {
			return
		}
		if m.hist.w == 0 {
			m.hist = m.historyGeom()
		}
		step := max(1, (n+m.hist.w-1)/max(1, m.hist.w))
		if m.cursor < 0 {
			switch k {
			case "[":
				m.cursor = n
			case "]":
				m.cursor = -1
			default:
				m.cursor = n - 1
				if k == "right" || k == "l" {
					m.cursor = 0
				}
				m.cursorAt = m.t
				return
			}
		}
		switch k {
		case "left", "h":
			m.cursor = max(0, m.cursor-step)
		case "right", "l":
			m.cursor = min(n-1, m.cursor+step)
		case "week-":
			m.cursor = max(0, m.cursor-7)
		case "week+":
			m.cursor = min(n-1, m.cursor+7)
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = n - 1
		case "[":
			mk := m.marks()
			for i := len(mk) - 1; i >= 0; i-- {
				if mk[i] < m.cursor {
					m.cursor = mk[i]
					break
				}
			}
		case "]":
			for _, d := range m.marks() {
				if d > m.cursor {
					m.cursor = d
					break
				}
			}
		}
		m.cursor = max(0, min(n-1, m.cursor))
		m.cursorAt = m.t
	case vCrew:
		n := len(m.crew)
		if n == 0 {
			return
		}
		prev := m.sel
		switch k {
		case "left", "h":
			m.sel = (m.sel + n - 1) % n
		case "right", "l":
			m.sel = (m.sel + 1) % n
		case "home", "g":
			m.sel = 0
		case "end", "G":
			m.sel = n - 1
		default:
			return
		}
		m.walk(prev)
	}
}

func (m *Model) walk(prev int) {
	m.selFrom = prev
	m.selAt = m.t
}

func (m *Model) click(x, y int, b tea.MouseButton) {
	if b != tea.MouseLeft {
		return
	}
	if y == 0 {
		for i, r := range tabRanges(m.w) {
			if x >= r[0] && x < r[1] {
				m.go2(view(i))
			}
		}
		return
	}
	switch m.view {
	case vCrew:
		best, bd := -1, 6.0
		for k, p := range m.crewPts {
			d := math.Hypot(float64(x)-p[0]/2, (float64(y)-p[1]/4)*2)
			if d < bd {
				best, bd = k, d
			}
		}
		if best >= 0 && best != m.sel {
			prev := m.sel
			m.sel = best
			m.walk(prev)
		}
	case vHistory:
		m.drag(x, y)
	}
}

func (m *Model) drag(x, y int) {
	if m.view != vHistory || m.hist.w == 0 {
		return
	}
	g := m.hist
	if y < g.py || y >= g.py+g.ph {
		return
	}
	n := len(m.cum)
	f := clamp((float64(x)-float64(g.x)+0.5)/float64(g.w), 0, 1)
	m.cursor = int(math.Round(f * float64(n-1)))
	m.cursorAt = m.t
}

// View draws one frame.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "tuios ✦ " + thousands(m.data.Stars)
	v.BackgroundColor = toRGBA(cNight)
	return v
}

// Farewell is the line the program leaves in the scrollback when it quits.
func (m *Model) Farewell() (stars int, you int) {
	you = -1
	if m.find == findFound {
		you = m.youIdx + 1
	}
	return m.data.Stars, you
}

func (m *Model) render() string {
	if m.w == 0 || m.h == 0 {
		return ""
	}
	c := NewCanvas(m.w, m.h, cNight)
	small := m.w < minW || m.h < minH
	o := skyOpts{t: m.t, shown: m.shown(), haze: m.profile >= colorprofile.TrueColor, youRow: [2]int{2, m.h - 3}}
	if small {
		o.dim = 0.5
	}
	hits := m.sky.draw(c, o)
	if small {
		m.drawTooSmall(c)
		return c.Render()
	}
	if m.view == vSky && hits.you[0] >= 0 {
		m.labelYou(c, hits.you)
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
	m.panelTop = y
	if m.help {
		m.drawHelp(c)
	}
	return c.Render()
}

// labelYou writes "you" beside the viewer's star, on the side with room.
func (m *Model) labelYou(c *Canvas, at [2]int) {
	a := easeOutCubic((m.t - m.sky.youAt - 0.6) / 0.6)
	if a <= 0 {
		return
	}
	st := Style{Fg: Mix(cNight, cGold, a), Attr: attrBold}
	cx, _, cw, _ := m.skyCardRect()
	// Past the ring round the star, on the side away from the card.
	if at[0]+8 < m.w && (at[0] > cx+cw || at[0]+8 < cx) {
		c.SkyText(at[0]+4, at[1], "you", st)
	} else {
		c.SkyText(at[0]-6, at[1], "you", st)
	}
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

const pillGap = 1

func tabRanges(w int) [][2]int {
	// Tabs are right-aligned pills: " n name " with one bare column between.
	total := 0
	for i, n := range viewNames {
		total += TextWidth(n) + 4
		if i > 0 {
			total += pillGap
		}
	}
	x := w - 2 - total
	out := make([][2]int, len(viewNames))
	for i, n := range viewNames {
		tw := TextWidth(n) + 4
		out[i] = [2]int{x, x + tw}
		x += tw + pillGap
	}
	return out
}

// drawHeader is styled after the tuios dock: a filled mode pill in the tuios
// accent on the left, and the views as workspace pills on the right. The
// current one is bold and underlined on the same quiet fill.
func (m *Model) drawHeader(c *Canvas) {
	pill := Style{Fg: cButter, Bg: cCharple, Attr: attrBold}
	x := c.Text(2, 0, " tuios ", pill)
	x = c.Text(x+1, 0, "✦", Style{Fg: cGold})
	c.Text(x+1, 0, "5k", Style{Fg: cGold, Attr: attrBold})
	for i, r := range tabRanges(m.w) {
		active := view(i) == m.view
		st := Style{Fg: cMuted, Bg: cPill}
		num := Style{Fg: cDim, Bg: cPill}
		if active {
			st = Style{Fg: cGoldHi, Bg: cPill, Attr: attrBold | attrUnderline}
			num = Style{Fg: cGold, Bg: cPill, Attr: attrBold | attrUnderline}
		}
		c.Text(r[0], 0, " ", Style{Bg: cPill})
		c.Text(r[0]+1, 0, itoa(i+1), num)
		c.Text(r[0]+2, 0, " "+viewNames[i], st)
		c.Text(r[1]-1, 0, " ", Style{Bg: cPill})
	}
}

type helpItem struct{ k, d string }

func (m *Model) helpItems() []helpItem {
	var items []helpItem
	add := func(b key.Binding) { items = append(items, helpItem{b.Help().Key, b.Help().Desc}) }
	switch m.view {
	case vSky:
		add(keys.Shoot)
		add(keys.Find)
	case vHistory:
		items = append(items, helpItem{"←→", "scrub"}, helpItem{"[ ]", "marks"})
	case vCrew:
		items = append(items, helpItem{"←→", "walk"})
		add(keys.Shoot)
	case vThanks:
		add(keys.Copy)
		add(keys.Shoot)
	}
	add(keys.Next)
	add(keys.Help)
	add(keys.Quit)
	return items
}

type span struct {
	s  string
	st Style
}

func (m *Model) status() []span {
	var parts []span
	add := func(s string, st Style) { parts = append(parts, span{s, st}) }
	switch {
	case m.t-m.copied < 1.6:
		add("sent to clipboard", Style{Fg: cText})
		return parts
	case m.find == findLoading:
		dots := strings.Repeat("·", 1+int(m.t*4)%3)
		add("looking for your star "+dots, Style{Fg: cMuted, Attr: attrItalic})
		return parts
	case (m.find == findFailed || m.find == findTyping) && m.findNote != "" && m.t-m.findAt < 5:
		add(m.findNote, Style{Fg: cGoldLo})
		return parts
	}
	if m.inside {
		add("✦ inside tuios", Style{Fg: cGold})
		add("   ", Style{})
	}
	switch m.conn {
	case "live":
		add("● ", Style{Fg: cGreen})
		add("live", Style{Fg: cMuted})
	case "offline":
		add("offline snapshot · "+shortDate(m.data.Taken), Style{Fg: cDim, Attr: attrItalic})
	default:
		add("○ ", Style{Fg: cDim})
		add("connecting", Style{Fg: cDim})
	}
	return parts
}

func (m *Model) drawFooter(c *Canvas) {
	y := m.h - 1
	parts := m.status()
	sw := 0
	for _, p := range parts {
		sw += TextWidth(p.s)
	}
	sx := m.w - 2 - sw
	x := sx
	for _, p := range parts {
		x = c.Text(x, y, p.s, p.st)
	}
	if m.find == findTyping {
		m.drawInput(c, y, sx)
		return
	}
	// Help, left-aligned. When it would run into the status, items go from
	// the middle out, so the first key of the view and "q quit" stay.
	items := m.helpItems()
	width := func() int {
		w := 0
		for i, it := range items {
			w += TextWidth(it.k) + 1 + TextWidth(it.d)
			if i > 0 {
				w += 3
			}
		}
		return w
	}
	for len(items) > 2 && 2+width() > sx-2 {
		items = append(items[:len(items)-2], items[len(items)-1])
	}
	x = 2
	for i, it := range items {
		if i > 0 {
			x = c.Text(x, y, " · ", Style{Fg: cFaint})
		}
		x = c.Text(x, y, it.k, Style{Fg: cText})
		x = c.Text(x+1, y, it.d, Style{Fg: cMuted})
	}
}

// drawInput is the one-line login prompt that replaces the help.
func (m *Model) drawInput(c *Canvas, y, limit int) {
	x := c.Text(2, y, "find your star", Style{Fg: cGold, Attr: attrBold})
	x = c.Text(x+2, y, "github login", Style{Fg: cMuted})
	x = c.Text(x+1, y, "›", Style{Fg: cDim})
	x++
	fw := 24
	c.Fill(x, y, fw, 1, cPill)
	val := m.input.Value()
	pos := m.input.Position()
	vx := x + 1
	c.Text(vx, y, val, Style{Fg: cText, Bg: cPill})
	// The cursor: a block over the character it sits on.
	blink := math.Mod(m.t-m.findAt, 1.0) < 0.6 || m.t-m.findAt < 0.6
	cur := ' '
	if r := []rune(val); pos < len(r) {
		cur = r[pos]
	}
	if blink {
		c.Set(vx+pos, y, cur, Style{Fg: cPanel, Bg: cGold})
	}
	hx := x + fw + 2
	hint := "enter find · esc cancel"
	if hx+TextWidth(hint) < limit-2 {
		e := c.Text(hx, y, "enter", Style{Fg: cText})
		e = c.Text(e+1, y, "find", Style{Fg: cMuted})
		e = c.Text(e, y, " · ", Style{Fg: cFaint})
		e = c.Text(e, y, "esc", Style{Fg: cText})
		c.Text(e+1, y, "cancel", Style{Fg: cMuted})
	}
}

func (m *Model) drawHelp(c *Canvas) {
	rows := []helpItem{
		{"tab  shift+tab", "next and previous view"},
		{"1  2  3  4", "sky, history, crew, thanks"},
		{"space", "send a shooting star"},
		{"f", "find your own star"},
		{"← →", "scrub the history, walk the crew"},
		{"⇧← ⇧→", "move a week in the history"},
		{"[  ]", "jump between marks"},
		{"r", "play the count again"},
		{"enter", "skip the count"},
		{"c", "copy the install line"},
		{"q", "quit"},
	}
	w, h := 58, len(rows)+4
	x, y := (m.w-w)/2, (m.h-h)/2
	// Dim what is behind, so the card reads as on top.
	c.Fade(0, 1, m.w, m.h-2, cNight, 0.6)
	c.Box(x, y, w, h, cBorderH, cPanel, "keys", Style{Fg: cText, Bg: cPanel})
	for i, r := range rows {
		c.Text(x+3, y+2+i, r.k, Style{Fg: cGold})
		c.Text(x+19, y+2+i, r.d, Style{Fg: cMuted})
	}
}

func (m *Model) drawTooSmall(c *Canvas) {
	lines := []span{
		{"tuios ✦ 5k", Style{Fg: cGold, Attr: attrBold}},
		{"", Style{}},
		{"make the window at least 80×24", Style{Fg: cText}},
		{"it is " + itoa(m.w) + "×" + itoa(m.h) + " now", Style{Fg: cMuted}},
		{"", Style{}},
		{"press q to quit", Style{Fg: cDim}},
	}
	w := 36
	h := len(lines) + 2
	x := (m.w - w) / 2
	y := (m.h - h) / 2
	// A cleared margin of one cell round the card.
	c.Fill(x-1, y, w+2, h, cNight)
	c.Box(x, y, w, h, cBorder, cPanel, "", Style{})
	for i, l := range lines {
		center(c, x, w, y+1+i, l.s, l.st)
	}
}

// date writes a day the way the app shows dates: 8 nov 2025.
func date(t time.Time) string {
	return strings.ToLower(t.Format("2 Jan 2006"))
}

// shortDate writes 8 oct.
func shortDate(t time.Time) string {
	return strings.ToLower(t.Format("2 Jan"))
}

func (m *Model) day(i int) time.Time {
	return m.data.StartDay().AddDate(0, 0, i)
}

func (m *Model) contributors() int { return len(m.crew) }

// center writes s centred in [x, x+w).
func center(c *Canvas, x, w, y int, s string, st Style) {
	c.Text(x+(w-TextWidth(s))/2, y, s, st)
}
