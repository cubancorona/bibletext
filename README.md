# Holy Bible Study

A clean, modern desktop app for reading and searching the Bible, built in Go with
[Fyne](https://fyne.io/). It presents the **World English Bible (WEB)**, a public
domain translation, in a calm, responsive reading layout.

## Features

- 📖 **Responsive reading** — scripture flows as a centred column that wraps to the
  window width with a comfortable line length, and superscript verse numbers.
- 🔍 **Smart search** — keyword search across every verse with the matched terms
  highlighted, plus reference lookups like `John 3:16`, `Ps 23`, or `1 Cor 13`
  (common abbreviations are understood). An exact verse reference jumps straight
  to that verse in context.
- 🧭 **Quick navigation** — filterable book list, previous/next chapter, and a
  chapter picker grid.
- 🕮 **Recent history** — a slim, unobtrusive bar of recently read chapters you can
  jump back to, or clear.
- 🌗 **Light & dark mode** — toggle a warm "paper" light theme or an easy-on-the-eyes
  dark theme from the header.
- 📋 **Copy** — copy the current chapter to the clipboard.
- ⌨️ **Keyboard shortcuts** — `Cmd/Ctrl+F` focuses search, `Esc` clears it.

## Requirements

- Go 1.21 or newer
- Fyne v2.4.1 (and its system dependencies — see the [Fyne getting started guide](https://docs.fyne.io/started/))

## Build & run

```bash
go mod download
go build -o holy-bible .
./holy-bible
```

On first launch the app downloads the World English Bible from
[bible-api.com](https://bible-api.com/) (about 30–60 seconds) and saves a local
cache in your OS cache directory. Every later launch loads instantly from that
cache and works offline. Set `HOLY_BIBLE_CACHE_PATH` to override the cache
location.

## Project structure

| File | Responsibility |
| --- | --- |
| `main.go` | Entry point: load data, open the window |
| `bible.go` | Bible data model, search, reference parsing & book aliases |
| `cache.go` | Versioned, atomic on-disk cache |
| `fetch_bible_data.go` | API client with retry / backoff / rate-limit handling |
| `theme.go` | Colour palette and the light/dark Fyne theme |
| `state.go` | App state plus navigation / search / history logic |
| `ui.go` | Top-level layout, header, theme toggle, keyboard shortcuts |
| `sidebar.go` | Persistent navigation sidebar (search + book filter + list) |
| `reading.go` | Reading view: flowing column, verse highlight, chapter picker |
| `search.go` | Search results view with match highlighting |
| `history.go` | Slim recent-history bar |

## Tests

```bash
go test ./...          # everything, including in-memory UI render tests
go test -race ./...     # logic tests (UI-render tests are skipped — see below)
```

The widget tests that use `fyne.io/fyne/v2/test` are excluded under `-race`
because Fyne's test app clears its font cache on a background goroutine, which the
race detector flags against text measurement. That race is in Fyne's test harness,
not in this app or the real renderer.

## License

Application code is provided for educational and devotional use. The bundled
scripture text is the **World English Bible**, which is in the public domain.

---

> "Your word is a lamp to my feet and a light to my path." — Psalm 119:105
