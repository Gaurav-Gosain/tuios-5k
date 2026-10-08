# tuios ✦ 5k

[tuios](https://github.com/Gaurav-Gosain/tuios) has 5,000 stars. This is a
small program to say thank you. It draws a night sky in your terminal with one
dot for each person who starred the repo.

![tuios 5k: the sky, the star history, the crew and the thank-you card](assets/demo.gif)

## Run it

```sh
go run github.com/Gaurav-Gosain/tuios-5k@latest
```

You need Go 1.25 or newer and a terminal that is at least 80×24. A terminal
with truecolor looks best. A 256-colour terminal also works.

## Views

| Key | View | What it shows |
| --- | --- | --- |
| `1` | sky | The star count. It counts up as a time-lapse of the history, and the sky fills with the stars it counts. |
| `2` | history | Stars over time, with the 1k and 5k marks. Use `←` and `→` to read any day. |
| `3` | crew | Every contributor as a star. The lines join them in the order of their first commit. Use `←` and `→` to walk the line. |
| `4` | thanks | The install command and the repo link. Press `c` to copy the install command. |

Other keys:

- `tab` and `shift+tab` go to the next and the previous view. You can also click a tab.
- `space` sends a shooting star across the sky.
- `r` plays the time-lapse again on the sky view.
- `enter` skips the time-lapse.
- `q` quits.

Run it inside a tuios pane for a small extra.

## Data

The program asks the GitHub REST API for the current star count, fork count
and contributors. It does not use a token. Bots are not shown.

GitHub now asks for a token to list the stargazers with dates. So the star
history and the sky come from a snapshot in `data/snapshot.json`, which is
built into the binary. The snapshot holds the number of stars for each day and
no names. When GitHub cannot be reached, everything comes from the snapshot,
and the status bar shows "offline snapshot".

Flags:

- `-offline` uses the snapshot only and makes no network calls.
- `-view NAME` opens a view: `sky`, `history`, `crew` or `thanks`.

To refresh the snapshot, run this with a token:

```sh
GITHUB_TOKEN=$(gh auth token) go run ./cmd/snapshot > data/snapshot.json
```

## Credits

Thank you to everyone who starred tuios, filed an issue or sent a patch.

The tuios contributors, in the order of their first commit:
[Gaurav-Gosain](https://github.com/Gaurav-Gosain), [humaidq](https://github.com/humaidq), [cr2007](https://github.com/cr2007), [ShalokShalom](https://github.com/ShalokShalom), [Cjreek](https://github.com/Cjreek), [NSPC911](https://github.com/NSPC911), [Gsirawan](https://github.com/Gsirawan), [mathomp4](https://github.com/mathomp4), [jacob-sabella](https://github.com/jacob-sabella), [kylesnowschwartz](https://github.com/kylesnowschwartz), [dodorz](https://github.com/dodorz), [SebaWag](https://github.com/SebaWag), [ethanhawkes-gif](https://github.com/ethanhawkes-gif), [actes2](https://github.com/actes2), [FaintFlower](https://github.com/FaintFlower), [noomz](https://github.com/noomz), [ronisbr](https://github.com/ronisbr), [masshirodev](https://github.com/masshirodev), [fonnesbeck](https://github.com/fonnesbeck), [nathan-poncet](https://github.com/nathan-poncet), [NotAFlightRisk](https://github.com/NotAFlightRisk), [Tim4c](https://github.com/Tim4c), [davidreuss](https://github.com/davidreuss), [neomantra](https://github.com/neomantra), [JakeChop](https://github.com/JakeChop), [fdoving](https://github.com/fdoving), [zer0ken](https://github.com/zer0ken), [chenxin-yan](https://github.com/chenxin-yan), [ci4ic4](https://github.com/ci4ic4), [juancely](https://github.com/juancely), [j4ng5y](https://github.com/j4ng5y), [dreuss](https://github.com/dreuss), [drawde-nadroj](https://github.com/drawde-nadroj).

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss) and
[Bubbles](https://github.com/charmbracelet/bubbles) by Charm.

## License

[MIT](LICENSE)
