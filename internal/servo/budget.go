package servo

import (
	"fmt"
	"time"

	"github.com/dendec/zimlite/internal/memory"
)

const memoryPollInterval = 50 * time.Millisecond
const firstFrameTimeout = 20 * time.Second
const minimumHelperKiB int64 = 192 * 1024
const systemReserveKiB int64 = 50 * 1024

type memoryBudget struct {
	reserveKiB, helperKiB, helperSwapKiB int64
}

func budgetFor(system memory.System) memoryBudget {
	// Use all available physical RAM except the reserve needed to stop the
	// helper and display fallback. Reclaimable cache counts; swap does not.
	return memoryBudget{
		reserveKiB:    systemReserveKiB,
		helperKiB:     max(int64(0), system.AvailableKiB-systemReserveKiB),
		helperSwapKiB: 16 * 1024,
	}
}

// Once the helper is resident, MemAvailable already excludes its allocations.
// Add its resident footprint back when estimating the total allowance so the
// limit responds to other consumers without charging Servo twice. Swap is not
// available capacity. There is no additional fixed process cap or fractional
// allowance: the budget follows the RAM remaining after other consumers.
func budgetDuringSession(system memory.System, helper memory.Process) memoryBudget {
	system.AvailableKiB = min(system.TotalKiB, system.AvailableKiB+helper.RSSKiB)
	return budgetFor(system)
}

func (b memoryBudget) check(current memory.System, helper memory.Process, previous memory.System, elapsed time.Duration) error {
	if current.AvailableKiB < b.reserveKiB {
		return fmt.Errorf("system memory reserve reached (%d KiB available; reserve %d KiB)", current.AvailableKiB, b.reserveKiB)
	}
	if current.AvailableKiB < 2*b.reserveKiB && helper.SwapKiB > b.helperSwapKiB {
		return fmt.Errorf("servo swap pressure with low RAM (%d KiB swap; %d KiB available)", helper.SwapKiB, current.AvailableKiB)
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
	if b.helperKiB > 0 && helper.RSSKiB > b.helperKiB {
		return fmt.Errorf("servo physical memory allowance reached (%d KiB RSS; allowance %d KiB)", helper.RSSKiB, b.helperKiB)
	}
	return nil
}

// Existing inactive swap is not evidence of current pressure. Stop for rapid
// main-process swap growth only when physical headroom is already low.
func (b memoryBudget) checkViewer(current memory.System, viewer, previous memory.Process, elapsed time.Duration) error {
	growth := viewer.SwapKiB - previous.SwapKiB
	if current.AvailableKiB < 2*b.reserveKiB && elapsed > 0 && elapsed <= 250*time.Millisecond && growth >= 4*1024 {
		return fmt.Errorf("application swap growing with low RAM (%d KiB new swap; %d KiB available)", growth, current.AvailableKiB)
	}
	return nil
}
