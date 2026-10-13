package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func testPanel(t *testing.T) *fileComparePanel {
	t.Helper()
	a := test.NewApp()
	w := a.NewWindow("test")
	p := newFileComparePanel(w)
	w.SetContent(p.build())
	p.setPaths(`testdata\old_sample.py`, `testdata\new_sample.py`)
	return p
}

func TestDetailStartsCollapsed(t *testing.T) {
	p := testPanel(t)
	if p.detailBox.Visible() {
		t.Fatal("detail pane should start collapsed")
	}
	if p.count() == 0 {
		t.Fatal("expected diff rows")
	}
}

func TestSummaryColors(t *testing.T) {
	p := testPanel(t) // sample files: 19 added, 9 removed
	var added, removed *widget.TextSegment
	for _, s := range p.summary.Segments {
		ts, ok := s.(*widget.TextSegment)
		if !ok {
			continue
		}
		switch ts.Style.ColorName {
		case theme.ColorNameSuccess:
			added = ts
		case theme.ColorNameError:
			removed = ts
		}
	}
	if added == nil || !strings.Contains(added.Textual(), "added (+)") {
		t.Fatalf("green added segment missing: %+v", p.summary.Segments)
	}
	if removed == nil || !strings.Contains(removed.Textual(), "removed (−)") {
		t.Fatalf("red removed segment missing: %+v", p.summary.Segments)
	}
}

func TestNextPrevNavigation(t *testing.T) {
	p := testPanel(t)
	// Auto-select highlights the first diff (row 0) without expanding.
	if p.curIdx != 0 {
		t.Fatalf("auto-select should highlight row 0, got %d", p.curIdx)
	}
	if p.detailBox.Visible() {
		t.Fatal("auto-select must not expand the detail pane")
	}
	// Sample files: diffs at rows 0 (docstring), 3 (import json), 6, ...
	p.stepDiff(1)
	if !p.detailBox.Visible() {
		t.Fatal("Next should expand the detail pane")
	}
	if p.curIdx != 3 || p.detailID != 3 {
		t.Fatalf("expected row 3, got cur=%d detail=%d", p.curIdx, p.detailID)
	}
	p.stepDiff(1)
	if p.curIdx != 6 {
		t.Fatalf("expected row 6, got %d", p.curIdx)
	}
	p.stepDiff(-1)
	if p.curIdx != 3 {
		t.Fatalf("expected row 3 after Prev, got %d", p.curIdx)
	}
	// Wrap: Prev from first diff goes to the last diff row.
	p.gotoRow(0)
	p.stepDiff(-1)
	last := -1
	for i := p.count() - 1; i >= 0; i-- {
		if p.isDiffRow(i) {
			last = i
			break
		}
	}
	if p.curIdx != last {
		t.Fatalf("expected wrap to row %d, got %d", last, p.curIdx)
	}
}

func TestReclickCollapse(t *testing.T) {
	p := testPanel(t)
	p.onRowClick(3) // expands
	if !p.detailBox.Visible() || p.detailID != 3 {
		t.Fatal("click should expand row 3")
	}
	// Clicking the same row collapses; clicking another expands.
	p.onRowClick(3)
	if p.detailBox.Visible() {
		t.Fatal("reclick should collapse")
	}
	p.onRowClick(0)
	if !p.detailBox.Visible() || p.detailID != 0 {
		t.Fatal("click should expand row 0")
	}
}
