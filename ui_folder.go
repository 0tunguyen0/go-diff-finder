package main

import (
	"encoding/csv"
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"difffinder/internal/folder"
)

type folderComparePanel struct {
	win fyne.Window

	leftEntry  *widget.Entry
	rightEntry *widget.Entry
	summary    *widget.RichText
	table      *widget.Table
	filter     *widget.Entry

	recursive   *widget.Check
	withContent *widget.Check
	onlyDiff    *widget.Check
	ignoreCase  *widget.Check

	entries  []folder.Entry
	filtered []folder.Entry
	sum      folder.Summary
	selRow   int
	busy     *widget.ProgressBarInfinite

	openInFileTab func(left, right string)
}

func newFolderComparePanel(w fyne.Window) *folderComparePanel {
	return &folderComparePanel{win: w, selRow: -1}
}

func (p *folderComparePanel) build() fyne.CanvasObject {
	p.leftEntry = widget.NewEntry()
	p.leftEntry.SetPlaceHolder("Left folder — type, Browse, or drop a folder here")
	p.rightEntry = widget.NewEntry()
	p.rightEntry.SetPlaceHolder("Right folder — type, Browse, or drop a folder here")
	p.leftEntry.OnChanged = func(string) { p.compare() }
	p.rightEntry.OnChanged = func(string) { p.compare() }

	browseL := widget.NewButton("Browse…", func() { p.browseFolderInto(p.leftEntry) })
	browseR := widget.NewButton("Browse…", func() { p.browseFolderInto(p.rightEntry) })
	swap := widget.NewButton("⇄ Swap", func() {
		l, r := p.leftEntry.Text, p.rightEntry.Text
		p.leftEntry.SetText(r)
		p.rightEntry.SetText(l)
	})

	leftRow := container.NewBorder(nil, nil, nil, browseL, p.leftEntry)
	rightRow := container.NewBorder(nil, nil, nil, browseR, p.rightEntry)
	row1 := container.NewGridWithColumns(2, leftRow, rightRow)

	p.recursive = widget.NewCheck("Subfolders", func(bool) { p.compare() })
	p.recursive.SetChecked(true)
	p.withContent = widget.NewCheck("Compare content", func(bool) { p.compare() })
	p.withContent.SetChecked(true)
	p.ignoreCase = widget.NewCheck("Ignore case", func(bool) { p.compare() })
	p.ignoreCase.SetChecked(true)
	p.onlyDiff = widget.NewCheck("Only differences", func(bool) { p.applyFilter() })

	p.filter = widget.NewEntry()
	p.filter.SetPlaceHolder("Filter by name…")
	p.filter.OnChanged = func(string) { p.applyFilter() }

	compareBtn := widget.NewButton("Compare", func() { p.compare() })
	compareBtn.Importance = widget.HighImportance
	viewBtn := widget.NewButton("Open selected →", func() { p.openSelected() })
	exportBtn := widget.NewButton("Export CSV…", func() { p.exportCSV() })
	optsBtn := widget.NewButton("⚙", func() {
		dialog.NewCustom("Folder options", "Close",
			container.NewVBox(p.recursive, p.withContent, p.ignoreCase), p.win).Show()
	})
	row2 := container.NewBorder(nil, nil,
		container.NewHBox(compareBtn, swap),
		container.NewHBox(p.onlyDiff, viewBtn, exportBtn, optsBtn),
		p.filter)

	p.busy = widget.NewProgressBarInfinite()
	p.busy.Hide()

	p.summary = widget.NewRichText(sumSeg("Pick two folders to compare. You can also drop folders anywhere on the window."))
	p.summary.Wrapping = fyne.TextWrapWord
	hint := widget.NewLabel("select a “Different” file → open it above")
	hint.TextStyle = fyne.TextStyle{Italic: true}
	statusBar := container.NewHBox(p.busy, p.summary, layout.NewSpacer(), hint)

	p.table = widget.NewTable(p.tableSize, p.createCell, p.updateCell)
	p.table.OnSelected = func(id widget.TableCellID) { p.selRow = id.Row }
	for c, w := range map[int]float32{0: 130, 1: 420, 2: 110, 3: 110, 4: 260} {
		p.table.SetColumnWidth(c, w)
	}

	top := container.NewVBox(row1, row2, widget.NewSeparator())
	return container.NewBorder(top, statusBar, nil, nil, container.NewMax(p.table))
}

// ---- paths ----

func (p *folderComparePanel) setPathsFromDrop(paths []string) {
	switch {
	case len(paths) >= 2:
		p.leftEntry.SetText(paths[0])
		p.rightEntry.SetText(paths[1])
		p.compare()
	case len(paths) == 1:
		if strings.TrimSpace(p.leftEntry.Text) == "" {
			p.leftEntry.SetText(paths[0])
		} else {
			p.rightEntry.SetText(paths[0])
		}
		p.compare()
	}
}

func (p *folderComparePanel) browseFolderInto(e *widget.Entry) {
	d := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			return
		}
		e.SetText(uriToPath(u))
	}, p.win)
	d.Show()
}

// ---- compare ----

func (p *folderComparePanel) options() folder.Options {
	return folder.Options{
		Recursive:      p.recursive.Checked,
		CompareContent: p.withContent.Checked,
		IgnoreCase:     p.ignoreCase.Checked,
		IncludeHidden:  false,
	}
}

func (p *folderComparePanel) compare() {
	if p.leftEntry == nil || p.table == nil {
		return
	}
	l, r := strings.TrimSpace(p.leftEntry.Text), strings.TrimSpace(p.rightEntry.Text)
	if l == "" || r == "" {
		p.entries = nil
		p.filtered = nil
		p.setSummary(sumSeg("Pick two folders to compare. You can also drop folders anywhere on the window."))
		p.table.Refresh()
		return
	}
	if !dirExists(l) || !dirExists(r) {
		p.entries = nil
		p.filtered = nil
		p.setSummary(sumSeg("Both sides must be existing folders."))
		p.table.Refresh()
		return
	}
	p.busy.Show()
	go func(opt folder.Options) {
		entries, sum, err := folder.Compare(l, r, opt)
		fyne.Do(func() {
			p.busy.Hide()
			if err != nil {
				p.entries = nil
				p.filtered = nil
				p.setSummary(sumSeg("Error: " + err.Error()))
			} else {
				p.entries = entries
				p.sum = sum
				p.setSummary(sumParts(sum, "")...)
			}
			p.applyFilter()
		})
	}(p.options())
}

func (p *folderComparePanel) applyFilter() {
	if p.table == nil {
		return
	}
	q := ""
	only := false
	if p.filter != nil {
		q = strings.ToLower(strings.TrimSpace(p.filter.Text))
	}
	if p.onlyDiff != nil {
		only = p.onlyDiff.Checked
	}
	p.filtered = p.filtered[:0]
	p.selRow = -1
	for _, e := range p.entries {
		if only && e.Status == folder.Same {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.RelPath), q) {
			continue
		}
		p.filtered = append(p.filtered, e)
	}
	extra := ""
	if len(p.filtered) != len(p.entries) {
		extra = fmt.Sprintf("   (showing %d of %d)", len(p.filtered), len(p.entries))
	}
	if p.entries != nil {
		p.setSummary(sumParts(p.sum, extra)...)
	}
	p.table.Refresh()
}

// sumParts renders a folder summary: same count green, different count red.
func sumParts(sum folder.Summary, extra string) []widget.RichTextSegment {
	return []widget.RichTextSegment{
		sumAdded(fmt.Sprintf("%d same", sum.Same)),
		sumSeg(", "),
		sumRemoved(fmt.Sprintf("%d different", sum.Different)),
		sumSeg(fmt.Sprintf(", %d left-only, %d right-only (%d total)%s",
			sum.LeftOnly, sum.RightOnly, sum.Total, extra)),
	}
}

// setSummary replaces the status bar segments.
func (p *folderComparePanel) setSummary(segs ...widget.RichTextSegment) {
	p.summary.Segments = segs
	p.summary.Refresh()
}

// ---- table ----

func (p *folderComparePanel) tableSize() (int, int) { return len(p.filtered), 5 }

func (p *folderComparePanel) createCell() fyne.CanvasObject {
	l := widget.NewLabel("")
	l.TextStyle = fyne.TextStyle{Monospace: true}
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

func (p *folderComparePanel) updateCell(id widget.TableCellID, obj fyne.CanvasObject) {
	l := obj.(*widget.Label)
	if id.Row < 0 || id.Row >= len(p.filtered) {
		l.SetText("")
		return
	}
	e := p.filtered[id.Row]
	switch id.Col {
	case 0:
		l.SetText(statusGlyph(e.Status) + " " + e.Status.String())
	case 1:
		if e.IsDir {
			l.SetText(e.RelPath + "/")
		} else {
			l.SetText(e.RelPath)
		}
	case 2:
		if e.Status == folder.RightOnly {
			l.SetText("—")
		} else {
			l.SetText(formatSize(e.LeftSize))
		}
	case 3:
		if e.Status == folder.LeftOnly {
			l.SetText("—")
		} else {
			l.SetText(formatSize(e.RightSize))
		}
	case 4:
		l.SetText(e.Detail)
	}
}

func statusGlyph(s folder.Status) string {
	switch s {
	case folder.Same:
		return "＝"
	case folder.Different:
		return "≠"
	case folder.LeftOnly:
		return "→"
	case folder.RightOnly:
		return "←"
	default:
		return "!"
	}
}

// ---- actions ----

func (p *folderComparePanel) selectedEntry() *folder.Entry {
	if p.table == nil || len(p.filtered) == 0 {
		return nil
	}
	// widget.Table has no single "selected row" getter pre-v2.5 for all cases;
	// use the first Different entry if none explicitly tracked.
	// Track via OnSelected id stored below.
	if p.selRow >= 0 && p.selRow < len(p.filtered) {
		return &p.filtered[p.selRow]
	}
	return nil
}

func (p *folderComparePanel) openSelected() {
	e := p.selectedEntry()
	if e == nil {
		dialog.ShowInformation("Nothing selected", "Click a row that is “Different”, then press this button.", p.win)
		return
	}
	if e.IsDir || e.Status != folder.Different {
		dialog.ShowInformation("Not a file pair",
			"Select a file row with status “Different” to open it in File Compare.", p.win)
		return
	}
	if p.openInFileTab == nil {
		return
	}
	l := filepath.Join(strings.TrimSpace(p.leftEntry.Text), filepath.FromSlash(e.RelPath))
	r := filepath.Join(strings.TrimSpace(p.rightEntry.Text), filepath.FromSlash(e.RelPath))
	p.openInFileTab(l, r)
}

func (p *folderComparePanel) exportCSV() {
	if len(p.filtered) == 0 {
		dialog.ShowInformation("Nothing to export", "Compare two folders first.", p.win)
		return
	}
	d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()
		w := csv.NewWriter(uc)
		_ = w.Write([]string{"status", "path", "is_dir", "left_size", "right_size", "left_mod", "right_mod", "detail"})
		for _, e := range p.filtered {
			_ = w.Write([]string{
				e.Status.String(), e.RelPath, fmt.Sprint(e.IsDir),
				fmt.Sprint(e.LeftSize), fmt.Sprint(e.RightSize),
				formatTime(e.LeftMod), formatTime(e.RightMod), e.Detail,
			})
		}
		w.Flush()
		dialog.ShowInformation("Exported", fmt.Sprintf("%d rows saved.", len(p.filtered)), p.win)
	}, p.win)
	d.SetFileName("folder-compare.csv")
	d.Show()
}
