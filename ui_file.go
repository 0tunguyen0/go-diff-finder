package main

import (
	"fmt"
	"image/color"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"difffinder/internal/diff"
)

const maxTextBytes = 8 << 20 // 8 MB cap for in-memory text diff

type fileComparePanel struct {
	win fyne.Window

	leftEntry  *widget.Entry
	rightEntry *widget.Entry
	summary    *widget.RichText
	detail     *widget.Entry
	detailBox  *fyne.Container
	split      *container.Split
	list       *widget.List
	listBox    *fyne.Container

	// detailID is the row shown in the detail pane (-1 = none).
	// curIdx tracks the current row for Prev/Next navigation.
	// silentSel marks programmatic selection (highlight only, no expand).
	detailID  int
	curIdx    int
	silentSel bool

	ignoreWS    *widget.Check
	ignoreCase  *widget.Check
	ignoreBlank *widget.Check
	viewSelect  *widget.Select

	ops  []diff.LineOp
	rows []diff.SideRow
	info string // non-diff message (binary / error / hint)
}

func newFileComparePanel(w fyne.Window) *fileComparePanel {
	return &fileComparePanel{win: w, detailID: -1, curIdx: -1}
}

func (p *fileComparePanel) viewMode() string {
	if p.viewSelect == nil {
		return "Side by side"
	}
	return p.viewSelect.Selected
}

func (p *fileComparePanel) build() fyne.CanvasObject {
	p.leftEntry = widget.NewEntry()
	p.leftEntry.SetPlaceHolder("Left file — type, Browse, or drop a file here")
	p.rightEntry = widget.NewEntry()
	p.rightEntry.SetPlaceHolder("Right file — type, Browse, or drop a file here")
	p.leftEntry.OnChanged = func(string) { p.compare() }
	p.rightEntry.OnChanged = func(string) { p.compare() }

	browseL := widget.NewButton("Browse…", func() { p.browseInto(p.leftEntry) })
	browseR := widget.NewButton("Browse…", func() { p.browseInto(p.rightEntry) })
	swap := widget.NewButton("⇄ Swap", func() {
		l, r := p.leftEntry.Text, p.rightEntry.Text
		p.leftEntry.SetText(r)
		p.rightEntry.SetText(l)
	})
	clear := widget.NewButton("Clear", func() {
		p.leftEntry.SetText("")
		p.rightEntry.SetText("")
	})

	leftRow := container.NewBorder(nil, nil, nil, browseL, p.leftEntry)
	rightRow := container.NewBorder(nil, nil, nil, browseR, p.rightEntry)
	row1 := container.NewGridWithColumns(2, leftRow, rightRow)

	p.ignoreWS = widget.NewCheck("Ignore whitespace", func(bool) { p.compare() })
	p.ignoreCase = widget.NewCheck("Ignore case", func(bool) { p.compare() })
	p.ignoreBlank = widget.NewCheck("Skip blank lines", func(bool) { p.compare() })
	p.viewSelect = widget.NewSelect([]string{"Side by side", "Unified"}, func(string) { p.refresh() })
	p.viewSelect.SetSelected("Side by side")

	compareBtn := widget.NewButton("Compare", func() { p.compare() })
	compareBtn.Importance = widget.HighImportance
	copyBtn := widget.NewButton("Copy diff", func() { p.copyDiff() })
	exportBtn := widget.NewButton("Export…", func() { p.export() })
	row2 := container.NewHBox(compareBtn, swap, clear, widget.NewSeparator(),
		p.ignoreWS, p.ignoreCase, p.ignoreBlank, widget.NewSeparator(),
		widget.NewLabel("View:"), p.viewSelect, copyBtn, exportBtn)

	swatch := func(c color.Color) *canvas.Rectangle {
		r := canvas.NewRectangle(c)
		r.SetMinSize(fyne.NewSize(14, 14))
		return r
	}
	p.summary = widget.NewRichText(sumSeg("Pick two files to compare. You can also drop files anywhere on the window."))
	p.summary.Wrapping = fyne.TextWrapWord
	statusBar := container.NewHBox(p.summary, layout.NewSpacer(),
		swatch(fgDelete), widget.NewLabel("removed"),
		swatch(fgInsert), widget.NewLabel("added"),
		swatch(fgMuted), widget.NewLabel("unchanged"))

	p.list = widget.NewList(p.count, p.createRow, p.updateRow)
	p.list.OnSelected = func(id widget.ListItemID) { p.onRowClick(int(id)) }
	p.listBox = container.NewMax(p.list)

	p.detail = widget.NewEntry()
	p.detail.MultiLine = true
	p.detail.Wrapping = fyne.TextWrapWord
	p.detail.SetPlaceHolder("Full line text.")
	detailHead := container.NewBorder(nil, nil, nil,
		container.NewHBox(
			widget.NewButton("▲ Prev", func() { p.stepDiff(-1) }),
			widget.NewButton("▼ Next", func() { p.stepDiff(1) })),
		widget.NewLabel("Full line text"))
	p.detailBox = container.NewBorder(detailHead, nil, nil, nil,
		container.NewVScroll(p.detail))
	p.detailBox.Hide()
	p.split = container.NewVSplit(p.listBox, p.detailBox)
	p.split.Offset = 0.72

	top := container.NewVBox(row1, row2, widget.NewSeparator())
	return container.NewBorder(top, statusBar, nil, nil, p.split)
}

// ---- paths ----

func (p *fileComparePanel) setPaths(left, right string) {
	// SetText triggers OnChanged -> compare; guard double compare by setting directly.
	p.leftEntry.SetText(left)
	p.rightEntry.SetText(right)
	p.compare()
}

func (p *fileComparePanel) setPathsFromDrop(paths []string) {
	switch {
	case len(paths) >= 2:
		p.setPaths(paths[0], paths[1])
	case len(paths) == 1:
		if strings.TrimSpace(p.leftEntry.Text) == "" {
			p.leftEntry.SetText(paths[0])
		} else {
			p.rightEntry.SetText(paths[0])
		}
		p.compare()
	}
}

func (p *fileComparePanel) browseInto(e *widget.Entry) {
	d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()
		e.SetText(uriToPath(uc.URI()))
	}, p.win)
	d.Show()
}

// ---- compare ----

func (p *fileComparePanel) options() diff.Options {
	return diff.Options{
		IgnoreWhitespace: p.ignoreWS.Checked,
		IgnoreCase:       p.ignoreCase.Checked,
		IgnoreBlankLines: p.ignoreBlank.Checked,
	}
}

func (p *fileComparePanel) compare() {
	if p.leftEntry == nil || p.list == nil {
		return
	}
	l, r := strings.TrimSpace(p.leftEntry.Text), strings.TrimSpace(p.rightEntry.Text)
	p.ops = nil
	p.rows = nil
	p.info = ""
	if l == "" || r == "" {
		p.info = "Pick two files to compare. You can also drop files anywhere on the window."
		p.refresh()
		return
	}
	lt, lb, ls, lerr := readTextFile(l, maxTextBytes)
	rt, rb, rs, rerr := readTextFile(r, maxTextBytes)
	if lerr != nil || rerr != nil {
		msg := ""
		if lerr != nil {
			msg += "Left: " + lerr.Error() + ". "
		}
		if rerr != nil {
			msg += "Right: " + rerr.Error()
		}
		p.info = strings.TrimSpace(msg)
		p.refresh()
		return
	}
	if lb || rb {
		if ls == rs {
			// Same size binaries: hash-compare quickly.
			same := false
			if lb, rb := fileHash(l), fileHash(r); lb != "" && lb == rb {
				same = true
			}
			if same {
				p.info = fmt.Sprintf("Binary files are identical (%s each).", formatSize(ls))
			} else {
				p.info = fmt.Sprintf("Binary files differ (%s vs %s) — no text preview.",
					formatSize(ls), formatSize(rs))
			}
		} else {
			p.info = fmt.Sprintf("Binary files differ (%s vs %s) — no text preview.",
				formatSize(ls), formatSize(rs))
		}
		p.refresh()
		return
	}
	if ls >= maxTextBytes || rs >= maxTextBytes {
		p.info = fmt.Sprintf("File too large for text view (%s vs %s, cap %s).",
			formatSize(ls), formatSize(rs), formatSize(maxTextBytes))
		p.refresh()
		return
	}
	opt := p.options()
	p.ops = diff.Compare(lt, rt, opt)
	p.rows = diff.ToSideRows(p.ops)
	p.refresh()
}

func fileHash(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// Cheap FNV for the identical-size fast path (folder view uses SHA; fine here).
	h := uint64(14695981039346656037)
	for _, c := range b {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return fmt.Sprintf("%x-%d", h, len(b))
}

// ---- list ----

func (p *fileComparePanel) count() int {
	if p.info != "" || (p.ops == nil && p.rows == nil) {
		return 0
	}
	if p.viewMode() == "Unified" {
		return len(p.ops)
	}
	return len(p.rows)
}

// rowMaxFill caps rendered row text; the full line is always in the detail pane.
const rowMaxFill = 1000

func monoLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Monospace: true}
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

// monoGutter is a line-number label with an explicit color so added/removed
// rows are distinguishable even when the background wash is subtle.
func monoGutter(text string, col color.Color) *canvas.Text {
	t := canvas.NewText(text, col)
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

func setGutter(g *canvas.Text, text string, col color.Color) {
	g.Text = text
	g.Color = col
	g.Refresh()
}

// The row template holds BOTH layouts; updateRow shows the one matching the
// current view mode. Fixed-width gutters (GridWrap) keep line-number columns
// stable and stop long text from stretching the layout.
func (p *fileComparePanel) createRow() fyne.CanvasObject {
	uGutter := monoGutter("", fgMuted)
	uBody := monoLabel("")
	unified := container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(150, 24), uGutter), nil, uBody)

	lNo, lText := monoGutter("", fgMuted), monoLabel("")
	rNo, rText := monoGutter("", fgMuted), monoLabel("")
	left := container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(72, 24), lNo), nil, lText)
	right := container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(72, 24), rNo), nil, rText)
	side := container.NewGridWithColumns(2, left, right)

	return container.NewMax(
		canvas.NewRectangle(bgSame),
		unified,
		side,
	)
}

func rowObjects(obj fyne.CanvasObject) (bg *canvas.Rectangle, unified, side *fyne.Container) {
	// Order-independent: the Max holds one rectangle background, one Border
	// (unified: has a direct Label child) and one 2-column grid (side).
	max := obj.(*fyne.Container)
	for _, o := range max.Objects {
		switch c := o.(type) {
		case *canvas.Rectangle:
			bg = c
		case *fyne.Container:
			directLabel := false
			for _, k := range c.Objects {
				if _, ok := k.(*widget.Label); ok {
					directLabel = true
				}
			}
			if directLabel {
				unified = c
			} else {
				side = c
			}
		}
	}
	return bg, unified, side
}

func borderParts(b *fyne.Container) (gutter *canvas.Text, body *widget.Label) {
	// Order-independent: the Border holds one body Label and one wrap
	// container whose child is the gutter canvas.Text.
	for _, o := range b.Objects {
		switch c := o.(type) {
		case *widget.Label:
			body = c
		case *fyne.Container:
			for _, k := range c.Objects {
				if t, ok := k.(*canvas.Text); ok {
					gutter = t
				}
			}
		}
	}
	return gutter, body
}

// updateRow handles both modes.
func (p *fileComparePanel) updateRow(id widget.ListItemID, obj fyne.CanvasObject) {
	bg, unified, side := rowObjects(obj)
	if p.viewMode() == "Unified" {
		unified.Show()
		side.Hide()
		if int(id) >= len(p.ops) {
			return
		}
		o := p.ops[id]
		gutter, body := borderParts(unified)
		switch o.Kind {
		case diff.Same:
			bg.FillColor = bgSame
			setGutter(gutter, fmt.Sprintf("%5d %5d  ", o.OldLine, o.NewLine), fgMuted)
			body.SetText("  " + truncateForRow(o.Text, rowMaxFill))
		case diff.Delete:
			bg.FillColor = bgDelete
			setGutter(gutter, fmt.Sprintf("%5d %5s −", o.OldLine, ""), fgDelete)
			body.SetText("− " + truncateForRow(o.Text, rowMaxFill))
		case diff.Insert:
			bg.FillColor = bgInsert
			setGutter(gutter, fmt.Sprintf("%5s %5d +", "", o.NewLine), fgInsert)
			body.SetText("+ " + truncateForRow(o.Text, rowMaxFill))
		}
		bg.Refresh()
		return
	}
	unified.Hide()
	side.Show()
	if int(id) >= len(p.rows) {
		return
	}
	r := p.rows[id]
	leftCell := side.Objects[0].(*fyne.Container)
	rightCell := side.Objects[1].(*fyne.Container)
	lNo, lText := borderParts(leftCell)
	rNo, rText := borderParts(rightCell)

	bg.FillColor = bgSame
	if r.LeftKind == diff.Same && r.RightKind == diff.Same {
		setGutter(lNo, fmt.Sprintf("%d", r.LeftNo), fgMuted)
		lText.SetText(truncateForRow(r.LeftText, rowMaxFill))
		setGutter(rNo, fmt.Sprintf("%d", r.RightNo), fgMuted)
		rText.SetText(truncateForRow(r.RightText, rowMaxFill))
	} else {
		if r.LeftNo > 0 {
			setGutter(lNo, fmt.Sprintf("%d −", r.LeftNo), fgDelete)
			lText.SetText(truncateForRow(r.LeftText, rowMaxFill))
			bg.FillColor = bgDelete
		} else {
			// One-sided row: clear or recycled widgets show ghost text.
			setGutter(lNo, "", fgDelete)
			lText.SetText("")
		}
		if r.RightNo > 0 {
			setGutter(rNo, fmt.Sprintf("%d +", r.RightNo), fgInsert)
			rText.SetText(truncateForRow(r.RightText, rowMaxFill))
			if bg.FillColor == bgSame {
				bg.FillColor = bgInsert
			}
		} else {
			setGutter(rNo, "", fgInsert)
			rText.SetText("")
		}
	}
	bg.Refresh()
}

// onRowClick expands the detail pane for the clicked row; clicking the
// same row again collapses it.
func (p *fileComparePanel) onRowClick(id int) {
	if p.silentSel {
		p.detailID = id
		p.curIdx = id
		p.showDetail(id)
		return
	}
	p.curIdx = id
	if p.detailBox.Visible() && p.detailID == id {
		p.setDetailShown(false)
		return
	}
	p.detailID = id
	p.showDetail(id)
	p.setDetailShown(true)
}

func (p *fileComparePanel) setDetailShown(show bool) {
	if show {
		p.detailBox.Show()
	} else {
		p.detailBox.Hide()
		p.detailID = -1
	}
}

// isDiffRow reports whether row i is an actual change (not context).
func (p *fileComparePanel) isDiffRow(i int) bool {
	if p.viewMode() == "Unified" {
		return i >= 0 && i < len(p.ops) && p.ops[i].Kind != diff.Same
	}
	return i >= 0 && i < len(p.rows) &&
		(p.rows[i].LeftKind != diff.Same || p.rows[i].RightKind != diff.Same)
}

// firstDiffRow returns the first changed row, the first row when the files
// are identical, or -1 when there is nothing to show.
func (p *fileComparePanel) firstDiffRow() int {
	n := p.count()
	if n == 0 {
		return -1
	}
	for i := 0; i < n; i++ {
		if p.isDiffRow(i) {
			return i
		}
	}
	return 0
}

// stepDiff jumps to the next (dir=+1) or previous (dir=-1) changed row,
// wrapping around at the ends.
func (p *fileComparePanel) stepDiff(dir int) {
	n := p.count()
	if n == 0 {
		return
	}
	idx := p.curIdx
	if idx < 0 {
		if dir > 0 {
			idx = n - 1
		} else {
			idx = 0
		}
	}
	for k := 0; k < n; k++ {
		idx = (idx + dir + n) % n
		if p.isDiffRow(idx) {
			p.gotoRow(idx)
			return
		}
	}
}

// gotoRow selects a row, shows its full text and expands the detail pane.
func (p *fileComparePanel) gotoRow(idx int) {
	p.silentSel = true
	p.list.Select(widget.ListItemID(idx))
	p.silentSel = false
	p.detailID = idx
	p.curIdx = idx
	p.showDetail(idx)
	p.setDetailShown(true)
}

func (p *fileComparePanel) showDetail(id int) {
	var s string
	if p.viewMode() == "Unified" {
		if id < 0 || id >= len(p.ops) {
			return
		}
		o := p.ops[id]
		s = fmt.Sprintf("old:%d new:%d\n%s", o.OldLine, o.NewLine, o.Text)
	} else {
		if id < 0 || id >= len(p.rows) {
			return
		}
		r := p.rows[id]
		s = fmt.Sprintf("left:%d\n%s\n\nright:%d\n%s", r.LeftNo, r.LeftText, r.RightNo, r.RightText)
	}
	p.detail.SetText(s)
}

func (p *fileComparePanel) refresh() {
	if p.list == nil {
		return
	}
	p.list.Refresh()
	if p.info != "" {
		p.setSummary(sumSeg(p.info))
		return
	}
	if p.ops == nil {
		p.setSummary(sumSeg("Pick two files to compare."))
		return
	}
	s := diff.Summarize(p.ops)
	if s.Added == 0 && s.Removed == 0 {
		p.setSummary(sumSeg(fmt.Sprintf("Identical — %d lines match.", s.Same)))
	} else {
		p.setSummary(
			sumAdded(fmt.Sprintf("%d added (+)", s.Added)),
			sumSeg("    "),
			sumRemoved(fmt.Sprintf("%d removed (−)", s.Removed)),
			sumSeg(fmt.Sprintf("    %d unchanged   ·   %d rows in %s view",
				s.Same, p.count(), strings.ToLower(p.viewMode()))),
		)
	}
	if p.detailBox.Visible() {
		// Keep the expanded detail in sync; collapse if the row is gone.
		if p.detailID >= 0 && p.detailID < p.count() {
			p.showDetail(p.detailID)
		} else {
			p.setDetailShown(false)
		}
	} else {
		p.autoSelectFirst()
	}
}

// autoSelectFirst highlights the first change (without expanding the
// detail pane) so it is easy to spot.
func (p *fileComparePanel) autoSelectFirst() {
	id := p.firstDiffRow()
	if id < 0 {
		return
	}
	p.silentSel = true
	p.list.Select(widget.ListItemID(id))
	p.silentSel = false
}

// ---- share ----

// setSummary replaces the status bar segments.
func (p *fileComparePanel) setSummary(segs ...widget.RichTextSegment) {
	p.summary.Segments = segs
	p.summary.Refresh()
}

func (p *fileComparePanel) unifiedText() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- %s\n+++ %s\n", p.leftEntry.Text, p.rightEntry.Text))
	if p.info != "" {
		b.WriteString(p.info + "\n")
		return b.String()
	}
	for _, o := range p.ops {
		switch o.Kind {
		case diff.Same:
			b.WriteString(" " + o.Text + "\n")
		case diff.Delete:
			b.WriteString("-" + o.Text + "\n")
		case diff.Insert:
			b.WriteString("+" + o.Text + "\n")
		}
	}
	return b.String()
}

func (p *fileComparePanel) copyDiff() {
	fyne.CurrentApp().Clipboard().SetContent(p.unifiedText())
	p.summary.Segments = append(p.summary.Segments, sumSeg("   (copied to clipboard)"))
	p.summary.Refresh()
}

func (p *fileComparePanel) export() {
	d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()
		_, _ = uc.Write([]byte(p.unifiedText()))
		dialog.ShowInformation("Exported", "Diff saved to "+uriToPath(uc.URI()), p.win)
	}, p.win)
	d.SetFileName("diff.txt")
	d.Show()
}
