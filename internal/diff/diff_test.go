package diff

import "testing"

func TestIdentical(t *testing.T) {
	ops := Compare("a\nb\nc", "a\nb\nc", Options{})
	s := Summarize(ops)
	if s.Added != 0 || s.Removed != 0 || s.Same != 3 {
		t.Fatalf("got %+v ops=%v", s, ops)
	}
}

func TestAddRemove(t *testing.T) {
	ops := Compare("a\nb\nc", "a\nx\nc\nd", Options{})
	s := Summarize(ops)
	if s.Same != 2 {
		t.Fatalf("same: %+v", s)
	}
	if s.Added != 2 || s.Removed != 1 {
		t.Fatalf("added/removed: %+v", s)
	}
	rows := ToSideRows(ops)
	if len(rows) == 0 {
		t.Fatal("no side rows")
	}
}

func TestEmpty(t *testing.T) {
	if ops := Compare("", "", Options{}); len(ops) != 0 {
		t.Fatalf("expected no ops, got %v", ops)
	}
	ops := Compare("", "a\nb", Options{})
	if Summarize(ops).Added != 2 {
		t.Fatalf("got %+v", Summarize(ops))
	}
}

func TestIgnoreCase(t *testing.T) {
	ops := Compare("Hello", "hello", Options{})
	s := Summarize(ops)
	if s.Same != 0 || s.Added != 1 || s.Removed != 1 {
		t.Fatalf("case sensitive should differ: %+v %v", s, ops)
	}
	// normalized keys equal, but display keeps original left text
	ops = Compare("Hello", "hello", Options{IgnoreCase: true})
	if Summarize(ops).Same != 1 {
		t.Fatalf("ignore case should match: %v", ops)
	}
}

func TestIgnoreWhitespace(t *testing.T) {
	ops := Compare("a  b", "a b", Options{IgnoreWhitespace: true})
	if Summarize(ops).Same != 1 {
		t.Fatalf("whitespace: %v", ops)
	}
}

func TestBinarySniff(t *testing.T) {
	if IsBinary([]byte("hello\nworld")) {
		t.Fatal("text detected as binary")
	}
	if !IsBinary([]byte{'h', 0, 'i'}) {
		t.Fatal("NUL not detected")
	}
}
