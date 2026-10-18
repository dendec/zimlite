package ui

import (
	"testing"
	"time"

	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/trie"
)

func TestAnalogDirection(t *testing.T) {
	tests := []struct {
		name      string
		value     int16
		direction int32
		strength  float64
	}{
		{name: "deadzone", value: analogDeadZone, direction: 0, strength: 0},
		{name: "up", value: -32767, direction: -1, strength: 1},
		{name: "down", value: 32767, direction: 1, strength: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			direction, strength := analogDirection(test.value)
			if direction != test.direction {
				t.Fatalf("direction = %d, want %d", direction, test.direction)
			}
			if strength != test.strength {
				t.Fatalf("strength = %f, want %f", strength, test.strength)
			}
		})
	}
}

type recordingScroller struct {
	Scroller
	delta int32
}

func (s *recordingScroller) ScrollBy(delta int32)                         { s.delta += delta }
func (s *recordingScroller) SetTextLines(_ string, _ []document.TreeItem) {}
func (s *recordingScroller) ScrollToLine(_ int)                           {}

type inputTestViewer struct{ DocViewer }

func (*inputTestViewer) SetStatusOverride(_ string) {}

func TestLongAnalogHoldKeepsScrollingBounded(t *testing.T) {
	scroller := &recordingScroller{}
	app := &App{scroller: scroller}
	app.gamepad.leftYValue = 32767
	controller := &InputController{app: app}
	start := time.Now()
	controller.Update(start)
	controller.Update(start.Add(time.Hour))
	if scroller.delta <= 0 || scroller.delta > 320 {
		t.Fatalf("scroll after a long hold/stall = %d; want 1..320", scroller.delta)
	}
}

func TestLongTreeHoldAndDirectionReversal(t *testing.T) {
	entries := make([]document.ArticleEntry, 100)
	for i := range entries {
		entries[i] = document.ArticleEntry{Title: "Article"}
	}
	root, err := trie.NewIndexedTree(testTitleIndex(entries))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{navState: trie.NewNavState(root), loader: &DocumentLoader{},
		viewer: &inputTestViewer{}, scroller: &recordingScroller{}}
	controller := &InputController{app: app}
	start := time.Now()
	controller.updateTreeHold(1, 1, start)
	controller.updateTreeHold(1, 1, start.Add(time.Hour))
	if app.navState.Cursor != root.Children()[4] {
		t.Fatal("a stalled frame should advance at most four tree rows")
	}
	controller.updateTreeHold(-1, 1, start.Add(time.Hour+time.Second))
	if app.navState.Cursor != root.Children()[4] {
		t.Fatal("reversing direction must restart the hold delay")
	}
}

type testTitleIndex []document.ArticleEntry

func (i testTitleIndex) ArticleCount() int                              { return len(i) }
func (i testTitleIndex) ArticleAt(n int) (document.ArticleEntry, error) { return i[n], nil }

func TestAxisDebouncerRecognizesDirectReversal(t *testing.T) {
	var axis AxisDebouncer
	for _, step := range []struct {
		value int16
		fires bool
	}{{32767, true}, {20000, false}, {-32767, true}, {-20000, false}, {0, false}, {-32767, true}} {
		if got := axis.Update(step.value, analogDeadZone); got != step.fires {
			t.Fatalf("axis %d fired=%v, want %v", step.value, got, step.fires)
		}
	}
}

func TestAnalogDirectionStrengthIncreasesWithDeflection(t *testing.T) {
	_, low := analogDirection(analogDeadZone + 1000)
	_, high := analogDirection(analogDeadZone + 10000)
	if low <= 0 || high <= low || high >= 1 {
		t.Fatalf("strengths = %f, %f; want 0 < low < high < 1", low, high)
	}
}
