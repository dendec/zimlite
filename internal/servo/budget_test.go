package servo

import (
	"testing"
	"time"

	"github.com/dendec/zimlite/internal/memory"
)

func TestHandheldBudgetUsesAvailableRAMWithoutSwap(t *testing.T) {
	system := memory.System{TotalKiB: 997816, AvailableKiB: 641948, SwapFreeKiB: 498904}
	budget := budgetFor(system)
	if budget.reserveKiB < 190*1024 || budget.helperKiB < 270*1024 || budget.helperKiB > 300*1024 {
		t.Fatalf("unexpected handheld budget: %+v", budget)
	}
	system.AvailableKiB = 400 * 1024
	lower := budgetFor(system)
	if lower.helperKiB >= budget.helperKiB || lower.helperKiB >= minimumHelperKiB {
		t.Fatalf("insufficient RAM should decline startup: %+v", lower)
	}
	system.SwapFreeKiB = 0
	if budgetFor(system) != lower {
		t.Fatal("swap must not increase startup budget")
	}
}

func TestMemoryGuardRejectsPressureBeforeExhaustion(t *testing.T) {
	budget := memoryBudget{reserveKiB: 200 * 1024, helperKiB: 300 * 1024, helperSwapKiB: 16 * 1024}
	for _, tc := range []struct {
		name                           string
		available, rss, swap, previous int64
		reject                         bool
	}{
		{"normal", 400 * 1024, 220 * 1024, 0, 450 * 1024, false},
		{"reserve", 199 * 1024, 220 * 1024, 0, 199 * 1024, true},
		{"helper budget", 400 * 1024, 301 * 1024, 0, 400 * 1024, true},
		{"swap", 400 * 1024, 220 * 1024, 17 * 1024, 400 * 1024, true},
		{"forecast", 250 * 1024, 220 * 1024, 0, 300 * 1024, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := budget.check(memory.System{AvailableKiB: tc.available}, memory.Process{RSSKiB: tc.rss, SwapKiB: tc.swap}, memory.System{AvailableKiB: tc.previous}, 50*time.Millisecond)
			if (err != nil) != tc.reject {
				t.Fatalf("guard rejection=%v, want %v (%v)", err != nil, tc.reject, err)
			}
		})
	}
}
