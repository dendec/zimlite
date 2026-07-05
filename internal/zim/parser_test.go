package zim

import (
	"os"
	"testing"
)

func TestOpenZIM(t *testing.T) {
	path := os.Getenv("KIWIX_ZIM")
	if path == "" {
		t.Skip("KIWIX_ZIM not set, skipping ZIM integration test")
	}

	r, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()

	doc, err := r.MainPage()
	if err != nil {
		t.Fatalf("MainPage: %v", err)
	}
	if doc == nil {
		t.Fatal("nil document")
	}
	if len(doc.Blocks) == 0 {
		t.Error("main page has no blocks")
	}
}
