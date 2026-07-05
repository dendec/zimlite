//go:build cgo

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

	data, mime, err := r.MainPage()
	if err != nil {
		t.Fatalf("MainPage: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty document data")
	}
	if mime == "" {
		t.Error("main page has no mime type")
	}
}
