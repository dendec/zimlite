package servo

import (
	"fmt"
	"os"

	"github.com/dendec/zimlite/internal/profile"
)

// Prefer killing the disposable helper over the reader or system services if
// kernel OOM occurs between watchdog samples. This is a last resort, not a cap.
func prioritizeHelperOOM(pid int) {
	err := os.WriteFile(fmt.Sprintf("/proc/%d/oom_score_adj", pid), []byte("1000"), 0600)
	fields := map[string]any{"pid": pid, "oom_score_adj": 1000, "success": err == nil}
	if err != nil {
		fields["error"] = err.Error()
	}
	profile.Mark("servo.oom_priority", fields)
}
