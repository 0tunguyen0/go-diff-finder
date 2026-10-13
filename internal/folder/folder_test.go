package folder

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompareBasic(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	writeFile(t, filepath.Join(left, "same.txt"), "hello")
	writeFile(t, filepath.Join(right, "same.txt"), "hello")
	writeFile(t, filepath.Join(left, "diff.txt"), "aaa")
	writeFile(t, filepath.Join(right, "diff.txt"), "bbb")
	writeFile(t, filepath.Join(left, "only-left.txt"), "x")
	writeFile(t, filepath.Join(right, "only-right.txt"), "y")

	entries, sum, err := Compare(left, right, Options{Recursive: true, CompareContent: true})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Same != 1 || sum.Different != 1 || sum.LeftOnly != 1 || sum.RightOnly != 1 {
		t.Fatalf("unexpected summary %+v", sum)
	}
	byName := map[string]Status{}
	for _, e := range entries {
		byName[e.RelPath] = e.Status
	}
	if byName["same.txt"] != Same || byName["diff.txt"] != Different ||
		byName["only-left.txt"] != LeftOnly || byName["only-right.txt"] != RightOnly {
		t.Fatalf("wrong statuses: %v", byName)
	}
}

func TestCompareRecursive(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	writeFile(t, filepath.Join(left, "sub", "a.txt"), "1")
	writeFile(t, filepath.Join(right, "sub", "a.txt"), "1")

	_, sum, err := Compare(left, right, Options{Recursive: false, CompareContent: true})
	if err != nil {
		t.Fatal(err)
	}
	// Non-recursive still lists the subdir itself, but not its contents.
	if sum.Total != 1 {
		t.Fatalf("non-recursive should only list the subdir, got %+v", sum)
	}
	_, sum, err = Compare(left, right, Options{Recursive: true, CompareContent: true})
	if err != nil {
		t.Fatal(err)
	}
	// Recursive finds the subdir itself plus sub/a.txt.
	if sum.Same != 2 || sum.Total != 2 {
		t.Fatalf("recursive should find sub/ and sub/a.txt, got %+v", sum)
	}
}

func TestNotAFolder(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "x")
	f.Close()
	if _, _, err := Compare(f.Name(), t.TempDir(), Options{}); err == nil {
		t.Fatal("expected error for file as folder")
	}
}
