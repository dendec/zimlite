package servo

import (
	"fmt"
	"time"

	"github.com/dendec/zimlite/internal/memory"
)

const memoryPollInterval = 50 * time.Millisecond
const firstFrameTimeout = 20 * time.Second
const minimumHelperKiB int64 = 192 * 1024

type memoryBudget struct {
	reserveKiB, helperKiB, helperSwapKiB int64
}

func budgetFor(system memory.System) memoryBudget {
	// Leave room for SDL fallback, Go/native allocations, and the rest of the
	// handheld. Swap is never counted as available RAM for starting Servo.
	reserve := max(int64(128*1024), min(int64(256*1024), system.TotalKiB/5))
	helper := min(int64(1024*1024), max(int64(0), (system.AvailableKiB-reserve)*2/3))
	swap := int64(128 * 1024)
	if system.TotalKiB <= 2*1024*1024 {
		helper = min(helper, 384*1024)
		swap = 16 * 1024 // Abort before zram/reclaim can make the handheld unresponsive.
	}
	return memoryBudget{reserveKiB: reserve, helperKiB: helper, helperSwapKiB: swap}
}

func (b memoryBudget) check(current memory.System, helper memory.Process, previous memory.System, elapsed time.Duration) error {
	if current.AvailableKiB < b.reserveKiB {
		return fmt.Errorf("system memory reserve reached (%d KiB available; reserve %d KiB)", current.AvailableKiB, b.reserveKiB)
	}
	if err := b.checkHelper(helper); err != nil {
		return err
	}
	// A short forecast supplements the reserve during rapid loading. Ignore
	// distant samples after scheduling stalls and avoid startup noise while
	// available memory is still more than twice the reserve.
	if elapsed > 0 && elapsed <= 250*time.Millisecond && current.AvailableKiB < 2*b.reserveKiB {
		loss := max(int64(0), previous.AvailableKiB-current.AvailableKiB)
		projected := current.AvailableKiB - int64(float64(loss)*float64(200*time.Millisecond)/float64(elapsed))
		if projected < b.reserveKiB {
			return fmt.Errorf("rapid memory growth would cross reserve (%d KiB available; projected %d KiB)", current.AvailableKiB, projected)
		}
	}
	return nil
}

func (b memoryBudget) checkHelper(helper memory.Process) error {
	if helper.RSSKiB+helper.SwapKiB > b.helperKiB {
		return fmt.Errorf("servo resident memory budget reached (%d KiB RSS+swap; budget %d KiB)", helper.RSSKiB+helper.SwapKiB, b.helperKiB)
	}
	if helper.SwapKiB > b.helperSwapKiB {
		return fmt.Errorf("servo swap pressure (%d KiB; limit %d KiB)", helper.SwapKiB, b.helperSwapKiB)
	}
	return nil
}
