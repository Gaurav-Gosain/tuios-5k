// Command tuios-5k celebrates five thousand stars on tuios with a night sky in
// your terminal: one star for every person who starred the repo.
//
//	go run github.com/Gaurav-Gosain/tuios-5k@latest
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Gaurav-Gosain/tuios-5k/data"
	"github.com/Gaurav-Gosain/tuios-5k/internal/gh"
	"github.com/Gaurav-Gosain/tuios-5k/internal/ui"
)

func main() {
	offline := flag.Bool("offline", false, "use the built-in snapshot and do not call GitHub")
	start := flag.String("view", "sky", "view to open: sky, history, crew or thanks")
	me := flag.String("me", "", "a GitHub login to find in the sky")
	flag.Parse()

	snap, err := gh.Parse(data.Snapshot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuios-5k:", err)
		os.Exit(1)
	}
	v, ok := ui.ViewByName(*start)
	if !ok {
		fmt.Fprintf(os.Stderr, "tuios-5k: unknown view %q. Use sky, history, crew or thanks.\n", *start)
		os.Exit(2)
	}
	login := strings.TrimPrefix(*me, "@")
	if login != "" && !gh.ValidLogin(login) {
		fmt.Fprintf(os.Stderr, "tuios-5k: %q is not a GitHub login.\n", *me)
		os.Exit(2)
	}
	m := ui.New(ui.Options{Snapshot: snap, Offline: *offline, View: v, Me: login})
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tuios-5k:", err)
		os.Exit(1)
	}
	farewell(m)
}

// farewell leaves one line in the scrollback after the alt screen closes.
func farewell(m *ui.Model) {
	stars, you := m.Farewell()
	gold := lipgloss.NewStyle().Foreground(lipgloss.Color("#f4cf86"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#7d86a8"))
	bold := lipgloss.NewStyle().Bold(true)
	line := gold.Render("✦") + " " + bold.Render(ui.Thousands(stars)) + " people starred tuios"
	if you > 0 {
		line += ", and you are star " + gold.Render("≈ #"+ui.Thousands(you))
	}
	line += ". thank you.  " + muted.Render("github.com/Gaurav-Gosain/tuios")
	lipgloss.Println(line)
}
