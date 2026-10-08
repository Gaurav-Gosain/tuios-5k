// Command tuios-5k celebrates five thousand stars on tuios with a night sky in
// your terminal: one star for every person who starred the repo.
//
//	go run github.com/Gaurav-Gosain/tuios-5k@latest
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/Gaurav-Gosain/tuios-5k/data"
	"github.com/Gaurav-Gosain/tuios-5k/internal/gh"
	"github.com/Gaurav-Gosain/tuios-5k/internal/ui"
)

func main() {
	offline := flag.Bool("offline", false, "use the built-in snapshot and do not call GitHub")
	start := flag.String("view", "sky", "view to open: sky, history, crew or thanks")
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
	m := ui.New(ui.Options{Snapshot: snap, Offline: *offline, View: v})
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tuios-5k:", err)
		os.Exit(1)
	}
}
