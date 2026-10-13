package main

import (
	_ "embed"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

//go:embed icon.png
var iconPNG []byte

const windowTitle = "Diff Finder — Files & Folders"

const (
	defaultWinW = 1360
	defaultWinH = 800
	minWinW     = 900
	minWinH     = 600
)

func main() {
	a := app.NewWithID("com.difffinder.app")
	prefs := a.Preferences()
	w := a.NewWindow(windowTitle)
	w.SetIcon(fyne.NewStaticResource("icon.png", iconPNG))
	w.SetMaster()

	// Restore geometry: maximized if it was maximized at close, otherwise
	// the last size, always centered.
	startMax := prefs.Bool("win.maximized")
	ww, wh := float64(prefs.Float("win.w")), float64(prefs.Float("win.h"))
	if ww < minWinW || wh < minWinH {
		ww, wh = defaultWinW, defaultWinH
	}
	if sw, sh := screenSize(); sw > 0 && sh > 0 {
		scale := float32(1)
		if c := w.Canvas(); c != nil && c.Scale() > 0 {
			scale = c.Scale()
		}
		if mw, mh := float64(float32(sw)/scale), float64(float32(sh)/scale); ww > mw || wh > mh {
			ww, wh = defaultWinW, defaultWinH
		}
	}
	w.Resize(fyne.NewSize(float32(ww), float32(wh)))
	w.CenterOnScreen()

	w.SetOnClosed(func() {
		maxed := isMaximized(windowTitle)
		prefs.SetBool("win.maximized", maxed)
		if !maxed {
			sz := w.Canvas().Size()
			if sz.Width >= minWinW && sz.Height >= minWinH {
				prefs.SetFloat("win.w", float64(sz.Width))
				prefs.SetFloat("win.h", float64(sz.Height))
			}
		}
	})

	filePanel := newFileComparePanel(w)
	folderPanel := newFolderComparePanel(w)

	tabs := container.NewAppTabs(
		container.NewTabItem("File Compare", filePanel.build()),
		container.NewTabItem("Folder Compare", folderPanel.build()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Cross-panel action: folder view can push a file pair into the file tab.
	activeTab := "File Compare"
	tabs.OnSelected = func(t *container.TabItem) { activeTab = t.Text }
	folderPanel.openInFileTab = func(left, right string) {
		filePanel.setPaths(left, right)
		tabs.Select(tabs.Items[0])
	}

	// Window-level drag & drop: route to the visible tab.
	// 2 dropped items -> left+right, 1 item -> first empty side, else right.
	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) == 0 {
			return
		}
		paths := make([]string, 0, len(uris))
		for _, u := range uris {
			if p := uriToPath(u); p != "" {
				paths = append(paths, p)
			}
		}
		if len(paths) == 0 {
			return
		}
		if activeTab == "Folder Compare" {
			folderPanel.setPathsFromDrop(paths)
		} else {
			filePanel.setPathsFromDrop(paths)
		}
	})

	w.SetContent(tabs)

	// Optional CLI: DiffFinder.exe <left> <right> opens a file comparison,
	// DiffFinder.exe --folder <left> <right> opens a folder comparison.
	args := os.Args[1:]
	if len(args) == 3 && args[0] == "--folder" {
		folderPanel.leftEntry.SetText(args[1])
		folderPanel.rightEntry.SetText(args[2])
		tabs.Select(tabs.Items[1])
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") {
		filePanel.setPaths(args[0], args[1])
	}

	if startMax {
		go maximizeWhenReady(windowTitle)
	}
	w.ShowAndRun()
}
