// Package diff implements line-based file comparison.
// It uses Myers diff (via sergi/go-diff) on normalized lines, then
// post-processes into human friendly opcodes.
package diff

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// LineOp describes one row of a unified diff.
type LineOp struct {
	Kind    OpKind // Same, Delete, Insert
	OldLine int    // 1-based old line number, 0 if N/A
	NewLine int    // 1-based new line number, 0 if N/A
	Text    string
}

// OpKind is the kind of a diff row.
type OpKind int

const (
	Same OpKind = iota
	Delete
	Insert
)

// SideRow is one row of a side-by-side view.
type SideRow struct {
	LeftNo    int
	LeftText  string
	LeftKind  OpKind
	RightNo   int
	RightText string
	RightKind OpKind
	// Aligned means both sides represent the same logical row
	// (same / changed pair). False for pure add/remove pairing gaps.
}

// Options controls normalization before comparison.
type Options struct {
	IgnoreWhitespace bool
	IgnoreCase       bool
	IgnoreBlankLines bool
}

// Stats summarizes a diff.
type Stats struct {
	Added   int
	Removed int
	Same    int
}

// NormalizeLines splits text into lines and applies options.
// It returns display lines (original) and compare keys (normalized).
func NormalizeLines(text string, opt Options) (display []string, keys []string) {
	// Normalize line endings first.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	raw := strings.Split(text, "\n")
	// strings.Split("") returns [""], treat empty file as zero lines.
	if len(raw) == 1 && raw[0] == "" {
		return nil, nil
	}
	for _, ln := range raw {
		if opt.IgnoreBlankLines && strings.TrimSpace(ln) == "" {
			continue
		}
		display = append(display, ln)
		k := ln
		if opt.IgnoreWhitespace {
			// Collapse all whitespace runs and trim.
			k = strings.Join(strings.Fields(k), " ")
			k = strings.TrimSpace(k)
		}
		if opt.IgnoreCase {
			k = strings.ToLower(k)
		}
		keys = append(keys, k)
	}
	return display, keys
}

// Compare computes a unified op list for oldText vs newText.
func Compare(oldText, newText string, opt Options) []LineOp {
	oldDisp, oldKeys := NormalizeLines(oldText, opt)
	newDisp, newKeys := NormalizeLines(newText, opt)

	if len(oldKeys) == 0 && len(newKeys) == 0 {
		return nil
	}
	// One side empty: no need to run Myers at all.
	if len(oldKeys) == 0 {
		ops := make([]LineOp, 0, len(newDisp))
		for i, ln := range newDisp {
			ops = append(ops, LineOp{Kind: Insert, NewLine: i + 1, Text: ln})
		}
		return ops
	}
	if len(newKeys) == 0 {
		ops := make([]LineOp, 0, len(oldDisp))
		for i, ln := range oldDisp {
			ops = append(ops, LineOp{Kind: Delete, OldLine: i + 1, Text: ln})
		}
		return ops
	}

	dmp := diffmatchpatch.New()
	// Line-mode diff: encode each unique line as one rune so the
	// Myers diff operates on whole lines, then decode back.
	// Trailing "\n" on both sides keeps the final line encoded
	// identically to a mid-text line (line mode is newline-sensitive).
	a, b, lineArray := dmp.DiffLinesToChars(
		strings.Join(oldKeys, "\n")+"\n", strings.Join(newKeys, "\n")+"\n")
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	return lineOpsFromDiffs(diffs, oldDisp, newDisp)
}

func lineOpsFromDiffs(diffs []diffmatchpatch.Diff, oldDisp, newDisp []string) []LineOp {
	var ops []LineOp
	oldIdx, newIdx := 0, 0

	// In line mode every chunk is a whole number of lines, but a chunk
	// may carry a trailing "\n" from the join. Split and drop the empty
	// tail so each element is exactly one logical line.
	splitLines := func(s string) []string {
		if s == "" {
			return nil
		}
		parts := strings.Split(s, "\n")
		if parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
		return parts
	}

	for _, d := range diffs {
		lines := splitLines(d.Text)
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			for range lines {
				if oldIdx >= len(oldDisp) || newIdx >= len(newDisp) {
					break
				}
				ops = append(ops, LineOp{Kind: Same, OldLine: oldIdx + 1, NewLine: newIdx + 1, Text: oldDisp[oldIdx]})
				oldIdx++
				newIdx++
			}
		case diffmatchpatch.DiffDelete:
			for range lines {
				if oldIdx >= len(oldDisp) {
					break
				}
				ops = append(ops, LineOp{Kind: Delete, OldLine: oldIdx + 1, Text: oldDisp[oldIdx]})
				oldIdx++
			}
		case diffmatchpatch.DiffInsert:
			for range lines {
				if newIdx >= len(newDisp) {
					break
				}
				ops = append(ops, LineOp{Kind: Insert, NewLine: newIdx + 1, Text: newDisp[newIdx]})
				newIdx++
			}
		}
	}
	// Safety net: flush leftovers (e.g. trailing-newline mismatch).
	for oldIdx < len(oldDisp) && newIdx < len(newDisp) {
		if oldDisp[oldIdx] == newDisp[newIdx] {
			ops = append(ops, LineOp{Kind: Same, OldLine: oldIdx + 1, NewLine: newIdx + 1, Text: oldDisp[oldIdx]})
			oldIdx++
			newIdx++
		} else {
			ops = append(ops, LineOp{Kind: Delete, OldLine: oldIdx + 1, Text: oldDisp[oldIdx]})
			oldIdx++
			ops = append(ops, LineOp{Kind: Insert, NewLine: newIdx + 1, Text: newDisp[newIdx]})
			newIdx++
		}
	}
	for oldIdx < len(oldDisp) {
		ops = append(ops, LineOp{Kind: Delete, OldLine: oldIdx + 1, Text: oldDisp[oldIdx]})
		oldIdx++
	}
	for newIdx < len(newDisp) {
		ops = append(ops, LineOp{Kind: Insert, NewLine: newIdx + 1, Text: newDisp[newIdx]})
		newIdx++
	}
	return ops
}

// ToSideRows aligns unified ops into side-by-side rows.
// Consecutive Delete+Insert blocks are paired as changed rows.
func ToSideRows(ops []LineOp) []SideRow {
	var rows []SideRow
	i := 0
	for i < len(ops) {
		if ops[i].Kind == Same {
			o := ops[i]
			rows = append(rows, SideRow{
				LeftNo: o.OldLine, LeftText: o.Text, LeftKind: Same,
				RightNo: o.NewLine, RightText: o.Text, RightKind: Same,
			})
			i++
			continue
		}
		// Collect a change block.
		var dels, inss []LineOp
		for i < len(ops) && ops[i].Kind != Same {
			if ops[i].Kind == Delete {
				dels = append(dels, ops[i])
			} else {
				inss = append(inss, ops[i])
			}
			i++
		}
		n := len(dels)
		if len(inss) > n {
			n = len(inss)
		}
		for k := 0; k < n; k++ {
			r := SideRow{}
			if k < len(dels) {
				r.LeftNo = dels[k].OldLine
				r.LeftText = dels[k].Text
				r.LeftKind = Delete
			}
			if k < len(inss) {
				r.RightNo = inss[k].NewLine
				r.RightText = inss[k].Text
				r.RightKind = Insert
			}
			rows = append(rows, r)
		}
	}
	return rows
}

// Summarize counts ops.
func Summarize(ops []LineOp) Stats {
	var s Stats
	for _, o := range ops {
		switch o.Kind {
		case Same:
			s.Same++
		case Delete:
			s.Removed++
		case Insert:
			s.Added++
		}
	}
	return s
}

// IsBinary sniffs content for binary data.
func IsBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// NUL byte is the strongest signal.
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	// Check UTF-8 validity on first 8KB.
	sample := data
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	return !isValidUTF8(sample)
}

func isValidUTF8(b []byte) bool {
	i := 0
	for i < len(b) {
		c := b[i]
		if c < 0x80 {
			// Allow common control chars; reject other C0/C1 except \t\n\r.
			if c < 0x20 && c != '\n' && c != '\r' && c != '\t' {
				return false
			}
			i++
			continue
		}
		var need int
		switch {
		case c>>5 == 0x6:
			need = 1
		case c>>4 == 0xE:
			need = 2
		case c>>3 == 0x1E:
			need = 3
		default:
			return false
		}
		if i+need >= len(b) {
			return false
		}
		for j := 1; j <= need; j++ {
			if b[i+j]>>6 != 0x2 {
				return false
			}
		}
		i += need + 1
	}
	return true
}
