package ui

// The palette: one night, one warm accent for light, one cool accent for
// structure. Everything else is a step between them.
var (
	cNight   = Hex("#090c17") // the sky, and the whole background
	cPanel   = Hex("#0f1325") // a surface, lifted clear of the sky
	cPill    = Hex("#1a2038")
	cBorder  = Hex("#343c62")
	cBorderH = Hex("#4a5590")
	cText    = Hex("#d9def0")
	cMuted   = Hex("#7d86a8")
	cDim     = Hex("#4f5778")
	cFaint   = Hex("#2a3150")
	cGold    = Hex("#f4cf86")
	cGoldHi  = Hex("#fff4d8")
	cGoldLo  = Hex("#d99a4e")
	cBlue    = Hex("#8b9cff")
	cBlueLo  = Hex("#3d4a8f")
	cLine    = Hex("#4a5590")
	cGreen   = Hex("#8fd6a8")
	cWhite   = Hex("#ffffff")

	// Star temperatures.
	cStarCool = Hex("#dce4ff")
	cStarWarm = Hex("#ffe6c4")
	cStarBlue = Hex("#a9bbff")

	// Two tokens from the tuios chrome: the accent its mode pill wears in
	// the dock (charmtone Charple) and the ink on it (charmtone Butter).
	// They are the only purple in the app, so the header reads as the tuios
	// dock without a word about it.
	cCharple = Hex("#6b50ff")
	cButter  = Hex("#fffaf1")
)

// lowColor is set when the terminal shows 256 colours or fewer. The sky
// then goes grey, and the soft fills that would band are left out.
var lowColor bool

// use256 swaps the night colours for exact xterm-256 greys. The truecolor
// navies would round to a loud blue or to black, and the steps between
// them would vanish.
func use256() {
	lowColor = true
	cNight = Hex("#121212")
	cPanel = Hex("#1c1c1c")
	cPill = Hex("#303030")
	cBorder = Hex("#4e4e4e")
	cBorderH = Hex("#626262")
	cFaint = Hex("#3a3a3a")
	cDim = Hex("#626262")
	cBlueLo = Hex("#444444")
	cLine = Hex("#585858")
}
