package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotationPreservesLatestEventsAndStop(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KIWIX_PROFILE_DIR", dir)
	stop := Start()
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			stop()
		}
	})
	for i := 0; i < 600; i++ {
		Mark("sample", map[string]any{"payload": strings.Repeat("x", 8192)})
	}
	Mark("latest", nil)
	stop()
	stopped = true
	previous, err := os.Stat(filepath.Join(dir, "events.previous.jsonl"))
	if err != nil || previous.Size() < 4<<20 || previous.Size() > (4<<20)+(16<<10) {
		t.Fatalf("unexpected rotated log: %v, %v", previous, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) >= 4<<20 || !strings.Contains(string(data), `"event":"latest"`) || !strings.Contains(string(data), `"event":"app.stop"`) {
		t.Fatal("rotation discarded recent events or exceeded the bound")
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatal("rotation produced an incomplete JSON event")
		}
	}
}
