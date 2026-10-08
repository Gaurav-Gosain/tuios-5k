package ui

// The palette: one night, one warm accent for light, one cool accent for
// structure. Everything else is a step between them.
var (
	cNight   = Hex("#090c17") // the sky, and the whole background
	cPanel   = Hex("#0b0e1b") // a little lifted from the sky
	cPill    = Hex("#1a2038")
	cBorder  = Hex("#2a3150")
	cBorderH = Hex("#3a4370")
	cText    = Hex("#d9def0")
	cMuted   = Hex("#7d86a8")
	cDim     = Hex("#474f6e")
	cFaint   = Hex("#2a3150")
	cGold    = Hex("#f4cf86")
	cGoldHi  = Hex("#fff4d8")
	cGoldLo  = Hex("#d99a4e")
	cBlue    = Hex("#8b9cff")
	cBlueLo  = Hex("#3d4a8f")
	cGreen   = Hex("#8fd6a8")

	// Star temperatures.
	cStarCool = Hex("#dce4ff")
	cStarWarm = Hex("#ffe6c4")
	cStarBlue = Hex("#a9bbff")
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
	cPanel = Hex("#121212")
	cPill = Hex("#303030")
	cBorder = Hex("#444444")
	cFaint = Hex("#3a3a3a")
	cDim = Hex("#585858")
	cBlueLo = Hex("#303030")
}
