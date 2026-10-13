package main

import (
	"fmt"
	"image/color"
	"net/url"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Row tints: alpha ~36% so the red/green wash is unmistakable on dark
// themes yet text stays readable. Gutter colors mark each line's fate
// even where the wash is subtle (e.g. light themes, selected rows).
var (
	bgSame   = color.RGBA{0, 0, 0, 0}
	bgDelete = color.RGBA{248, 81, 73, 92}
	bgInsert = color.RGBA{46, 160, 67, 92}

	fgDelete = color.RGBA{255, 123, 114, 255}
	fgInsert = color.RGBA{126, 231, 135, 255}
	fgMuted  = color.RGBA{139, 148, 158, 255}
)

// sumSeg is one inline chunk of a status summary. An empty ColorName means
// the default foreground color.
func sumSeg(text string) *widget.TextSegment {
	return &widget.TextSegment{Text: text, Style: widget.RichTextStyle{Inline: true}}
}

// sumAdded is an inline summary chunk in the theme's success (green) color.
func sumAdded(text string) *widget.TextSegment {
	return &widget.TextSegment{Text: text, Style: widget.RichTextStyle{Inline: true, ColorName: theme.ColorNameSuccess}}
}

// sumRemoved is an inline summary chunk in the theme's error (red) color.
func sumRemoved(text string) *widget.TextSegment {
	return &widget.TextSegment{Text: text, Style: widget.RichTextStyle{Inline: true, ColorName: theme.ColorNameError}}
}

func uriToPath(u fyne.URI) string {
	if u == nil {
		return ""
	}
	p := u.Path()
	if p == "" {
		// Fallback: parse the URI string.
		s := u.String()
		if parsed, err := url.Parse(s); err == nil {
			p = parsed.Path
		} else {
			return ""
		}
	}
	if dec, err := url.PathUnescape(p); err == nil {
		p = dec
	}
	// file:///C:/x -> /C:/x on Windows; strip the leading slash.
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return p
}

func formatSize(n int64) string {
	if n < 0 {
		return "—"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 3; v /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("2006-01-02 15:04")
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// readTextFile reads up to maxBytes and reports binary detection.
func readTextFile(path string, maxBytes int64) (text string, binary bool, size int64, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", false, 0, err
	}
	size = fi.Size()
	f, err := os.Open(path)
	if err != nil {
		return "", false, size, err
	}
	defer f.Close()
	limit := size
	if limit > maxBytes {
		limit = maxBytes
	}
	buf := make([]byte, limit)
	n := 0
	for n < len(buf) {
		m, rerr := f.Read(buf[n:])
		n += m
		if rerr != nil {
			break
		}
	}
	buf = buf[:n]
	if isBinaryData(buf) {
		return "", true, size, nil
	}
	return string(buf), false, size, nil
}

func isBinaryData(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

func truncateForRow(s string, max int) string {
	s = strings.ReplaceAll(s, "\t", "    ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
