package convert

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectionFor(t *testing.T) {
	tests := []struct {
		path string
		want Direction
	}{
		{"a.smd", ToMd},
		{"a.SMD", ToMd},
		{"a.md", ToSmd},
		{"a.mdx", ToSmd},
		{"noext", ToSmd},
		{filepath.Join("dir", "b.markdown"), ToSmd},
	}
	for _, tt := range tests {
		if got := DirectionFor(tt.path); got != tt.want {
			t.Errorf("DirectionFor(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestFileWritesSibling(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "post.md")
	if err := os.WriteFile(src, []byte("# Hi\n\n![a](b.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := ToSmd.File(src)
	if err != nil {
		t.Fatalf("File() error = %v", err)
	}
	if want := filepath.Join(dir, "post.smd"); out != want {
		t.Errorf("output path = %q, want %q", out, want)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := "# Hi\n\n[a]($image.asset(\"b.png\"))\n"; string(data) != want {
		t.Errorf("content = %q, want %q", data, want)
	}
}

func TestFileAppendsWhenExtensionMatches(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "already.smd")
	if err := os.WriteFile(src, []byte("[t]($link.page(\"a\"))\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := ToMd.File(src)
	if err != nil {
		t.Fatalf("File() error = %v", err)
	}
	if want := filepath.Join(dir, "already.md"); out != want {
		t.Errorf("output path = %q, want %q", out, want)
	}
}

func TestFileMissingInput(t *testing.T) {
	if _, err := ToSmd.File(filepath.Join(t.TempDir(), "nope.md")); err == nil {
		t.Error("expected an error for a missing input file")
	}
}
