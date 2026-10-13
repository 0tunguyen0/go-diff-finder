# Diff Finder

A Windows desktop app (Go + Fyne) that compares **files** and **folders** and shows
differences in an easy-to-read visual format. Drag & drop is supported everywhere:
drop 1–2 files or folders anywhere on the window.

## Features

### General
- Window opens centered; maximized state is remembered across restarts

### File Compare
- Side-by-side and unified views with red/green highlighting
- Summary counts (added / removed / unchanged)
- ▲ Prev / ▼ Next buttons jump between changed lines (wraps around)
- Full-text panel is collapsed by default; click a row to expand, click again to collapse
- Options: ignore whitespace, ignore case, skip blank lines
- Binary-file detection (shows sizes instead of garbage)
- Click any row to see the full line; Copy diff / Export (.txt)

### Folder Compare
- Recursive comparison with SHA-256 content check
- Statuses: `Same`, `Different`, `Left only`, `Right only` (+ errors)
- Filter by name, “Only differences” toggle, ignore-case matching
- Select a `Different` file → “Open selected in File Compare”
- Export CSV report

## Requirements (build only)

- Go 1.22+
- A C compiler for Fyne (CGO), e.g. WinLibs GCC:
  `winget install -e --id BrechtSanders.WinLibs.POSIX.UCRT`

## Run / build

```bat
go run .
build.bat
```

Always build with `build.bat` (or the equivalent `go build` line below). A bare
`go build` / `go build ./...` does **not** produce the release exe: the former
names it after the folder and keeps the console window, the latter writes no
binary at all.

```bat
go build -trimpath -ldflags "-H windowsgui -s -w" -o DiffFinder.exe .
```

`-H windowsgui` removes the console window, `-s -w` strips debug info to keep
the exe small. `build.bat` produces `DiffFinder.exe` in the project folder. The exe icon comes
from `icon.png` (embedded in the window via `go:embed`, and baked into the
exe via `icon.ico` + `app_windows_amd64.syso` — regenerate with `rsrc`
after changing the icon).

Optional command line:
```bat
DiffFinder.exe <left-file> <right-file>
DiffFinder.exe --folder <left-dir> <right-dir>
```

## Tests

```bat
go test ./...
```

## Layout

```
main.go            app bootstrap, tabs, window drag & drop, geometry memory
ui_file.go         file-compare panel
ui_folder.go       folder-compare panel
ui_file_test.go    headless UI tests (navigation, detail pane, summary colors)
helpers.go         path/size/time/format helpers
winstate_windows.go  Win32 maximize detection (stdlib syscall only)
winstate_other.go    non-Windows stub
icon.png           app icon (embedded + exe icon source)
icon.ico / app_windows_amd64.syso  exe icon resources
internal/diff      line-diff engine (Myers via sergi/go-diff) + tests
internal/folder    recursive folder comparison + tests
```
