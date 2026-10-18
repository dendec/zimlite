package article

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSourceConfinesResourcesAndBoundsSize(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "index.html")
	if err := os.WriteFile(filename, []byte("<p>article</p>"), 0600); err != nil {
		t.Fatal(err)
	}
	s, _, err := OpenFileSource(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.ReadResource("index.html"); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.html")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.html")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{outside, "../outside.html", "escape.html"} {
		if _, err := s.ReadResource(name); err == nil {
			t.Fatalf("allowed resource outside article root: %s", name)
		}
	}
	f, err := os.Create(filepath.Join(root, "large.html"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxHTMLBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadResource("large.html"); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("oversized HTML error: %v", err)
	}
}
