package memory

import "testing"

func TestPhysicalMemoryParsing(t *testing.T) {
	snapshot, err := parseSystem([]byte("MemTotal: 997816 kB\nMemAvailable: 600000 kB\nSwapTotal: 498904 kB\nSwapFree: 400000 kB\n"))
	if err != nil || snapshot.TotalKiB != 997816 || snapshot.AvailableKiB != 600000 || snapshot.SwapFreeKiB != 400000 {
		t.Fatalf("snapshot=%+v, err=%v", snapshot, err)
	}
	for _, data := range []string{"MemTotal: 1000 kB", "MemTotal: 1000 kB\nMemAvailable: -1 kB", "MemTotal: 1000 kB\nMemAvailable: 1001 kB"} {
		if _, err := parseSystem([]byte(data)); err == nil {
			t.Fatalf("accepted invalid memory data: %s", data)
		}
	}
}
