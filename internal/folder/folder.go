// Package folder implements recursive folder comparison.
package folder

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Status is the comparison result for one relative path.
type Status int

const (
	Same Status = iota
	Different
	LeftOnly
	RightOnly
	Error
)

func (s Status) String() string {
	switch s {
	case Same:
		return "Same"
	case Different:
		return "Different"
	case LeftOnly:
		return "Left only"
	case RightOnly:
		return "Right only"
	default:
		return "Error"
	}
}

// Entry is one compared relative path.
type Entry struct {
	RelPath   string
	IsDir     bool
	Status    Status
	LeftSize  int64
	RightSize int64
	LeftMod   time.Time
	RightMod  time.Time
	Detail    string // error text or "sizes differ" etc.
}

// Options controls folder comparison.
type Options struct {
	Recursive      bool
	CompareContent bool // if false, compare size+modtime only (faster)
	IgnoreCase     bool // case-insensitive name matching (useful on Windows)
	IncludeHidden  bool
}

// Summary counts entries by status.
type Summary struct {
	Same      int
	Different int
	LeftOnly  int
	RightOnly int
	Errors    int
	Total     int
}

func (s Summary) String() string {
	return fmt.Sprintf("%d same, %d different, %d left-only, %d right-only (%d total)",
		s.Same, s.Different, s.LeftOnly, s.RightOnly, s.Total)
}

// fileMeta is a snapshot of one side.
type fileMeta struct {
	path  string
	isDir bool
	size  int64
	mod   time.Time
	err   error
}

// Compare walks left and right folders and matches relative paths.
func Compare(left, right string, opt Options) ([]Entry, Summary, error) {
	leftMap, err := snapshot(left, opt)
	if err != nil {
		return nil, Summary{}, fmt.Errorf("left folder: %w", err)
	}
	rightMap, err := snapshot(right, opt)
	if err != nil {
		return nil, Summary{}, fmt.Errorf("right folder: %w", err)
	}

	// Union of keys.
	keys := make(map[string]struct{}, len(leftMap)+len(rightMap))
	for k := range leftMap {
		keys[k] = struct{}{}
	}
	for k := range rightMap {
		keys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	var out []Entry
	var sum Summary
	for _, rel := range sorted {
		l, lok := leftMap[rel]
		r, rok := rightMap[rel]
		e := Entry{RelPath: rel}
		switch {
		case lok && !rok:
			e.Status = LeftOnly
			e.IsDir = l.isDir
			e.LeftSize = l.size
			e.LeftMod = l.mod
			sum.LeftOnly++
		case !lok && rok:
			e.Status = RightOnly
			e.IsDir = r.isDir
			e.RightSize = r.size
			e.RightMod = r.mod
			sum.RightOnly++
		default:
			e.IsDir = l.isDir || r.isDir
			e.LeftSize, e.RightSize = l.size, r.size
			e.LeftMod, e.RightMod = l.mod, r.mod
			if l.err != nil || r.err != nil {
				e.Status = Error
				e.Detail = errText(l.err, r.err)
				sum.Errors++
				break
			}
			if l.isDir || r.isDir {
				if l.isDir != r.isDir {
					e.Status = Different
					e.Detail = "type differs (file vs folder)"
					sum.Different++
				} else {
					e.Status = Same
					sum.Same++
				}
				break
			}
			st, detail := compareFiles(l.path, r.path, opt)
			e.Status = st
			e.Detail = detail
			switch st {
			case Same:
				sum.Same++
			case Different:
				sum.Different++
			default:
				e.Status = Error
				sum.Errors++
			}
		}
		sum.Total++
		out = append(out, e)
	}
	return out, sum, nil
}

func errText(a, b error) string {
	if a != nil && b != nil {
		return a.Error() + " | " + b.Error()
	}
	if a != nil {
		return a.Error()
	}
	return b.Error()
}

func compareFiles(lp, rp string, opt Options) (Status, string) {
	li, err1 := os.Stat(lp)
	ri, err2 := os.Stat(rp)
	if err1 != nil || err2 != nil {
		return Error, errText(err1, err2)
	}
	if li.Size() != ri.Size() {
		return Different, "sizes differ"
	}
	if !opt.CompareContent {
		if li.ModTime().Equal(ri.ModTime()) {
			return Same, ""
		}
		// Same size, different mtime: assume different to be safe? No —
		// fall through to content compare for correctness.
	}
	same, err := sameContent(lp, rp)
	if err != nil {
		return Error, err.Error()
	}
	if same {
		return Same, ""
	}
	return Different, "content differs"
}

func sameContent(a, b string) (bool, error) {
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()
	ha := sha256.New()
	hb := sha256.New()
	if _, err := io.Copy(ha, fa); err != nil {
		return false, err
	}
	if _, err := io.Copy(hb, fb); err != nil {
		return false, err
	}
	return ha.Sum(nil)[0] == hb.Sum(nil)[0] && string(ha.Sum(nil)) == string(hb.Sum(nil)), nil
}

func snapshot(root string, opt Options) (map[string]fileMeta, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a folder", root)
	}
	out := make(map[string]fileMeta)
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			rel, _ := filepath.Rel(root, path)
			out[displayRel(rel, opt)] = fileMeta{path: path, err: err}
			return nil
		}
		if path == root {
			return nil
		}
		name := d.Name()
		if !opt.IncludeHidden && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		key := rel
		if opt.IgnoreCase {
			key = strings.ToLower(rel)
		}
		fi, err := d.Info()
		if err != nil {
			out[key] = fileMeta{path: path, err: err}
			return nil
		}
		out[key] = fileMeta{path: path, isDir: d.IsDir(), size: fi.Size(), mod: fi.ModTime()}
		if d.IsDir() && !opt.Recursive {
			return filepath.SkipDir
		}
		return nil
	}
	if err := filepath.WalkDir(root, walkFn); err != nil {
		return nil, err
	}
	return out, nil
}

func displayRel(rel string, opt Options) string {
	rel = filepath.ToSlash(rel)
	if opt.IgnoreCase {
		return strings.ToLower(rel)
	}
	return rel
}
