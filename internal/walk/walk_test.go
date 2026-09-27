package walk

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"a.md", "b.smd", "c.MD", "d.txt", "e.markdown", "f.png", "g.bin",
		"noext", ".hidden.md", "sub/k.md", "sub/deep/l.smd", ".git/config",
	} {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Files(dir)
	if err != nil {
		t.Fatalf("Files() error = %v", err)
	}
	rel := make([]string, 0, len(got))
	for _, f := range got {
		r, err := filepath.Rel(dir, f)
		if err != nil {
			t.Fatal(err)
		}
		rel = append(rel, r)
	}
	sort.Strings(rel)

	want := []string{
		".hidden.md", "a.md", "b.smd", "c.MD", "d.txt", "e.markdown",
		"noext", "sub/deep/l.smd", "sub/k.md",
	}
	if strings.Join(rel, ",") != strings.Join(want, ",") {
		t.Errorf("Files() =\n %v\nwant\n %v", rel, want)
	}
}

func TestFilesSingleFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.md")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Files(src)
	if err != nil {
		t.Fatalf("Files() error = %v", err)
	}
	// Any non-directory path is returned untouched, whatever its extension.
	if len(got) != 1 || got[0] != src {
		t.Errorf("Files(%q) = %v", src, got)
	}
}

func TestFilesMissing(t *testing.T) {
	if _, err := Files(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected an error for a missing path")
	}
}
