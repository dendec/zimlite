package renderer

import (
	"testing"
)

func TestFindAnchorY(t *testing.T) {
	tests := []struct {
		name   string
		layout PageLayout
		anchor string
		wantY  int32
		wantOk bool
	}{
		{
			name: "anchorPositions lookup",
			layout: PageLayout{
				anchorPositions: map[string]int32{"Introduction": 300},
			},
			anchor: "Introduction",
			wantY:  300,
			wantOk: true,
		},
		{
			name: "text fallback H1",
			layout: PageLayout{
				anchorPositions: map[string]int32{"introduction": 150},
			},
			anchor: "Introduction",
			wantY:  150,
			wantOk: true,
		},
		{
			name: "text fallback H5",
			layout: PageLayout{
				anchorPositions: map[string]int32{"appendix": 400},
			},
			anchor: "Appendix",
			wantY:  400,
			wantOk: true,
		},
		{
			name: "text fallback H6",
			layout: PageLayout{
				anchorPositions: map[string]int32{"notes": 500},
			},
			anchor: "Notes",
			wantY:  500,
			wantOk: true,
		},
		{
			name: "underscore to space in text fallback",
			layout: PageLayout{
				anchorPositions: map[string]int32{"early life": 200},
			},
			anchor: "Early_Life",
			wantY:  200,
			wantOk: true,
		},
		{
			name: "cite_note to cite_ref",
			layout: PageLayout{
				anchorPositions: map[string]int32{"#cite-ref-42": 600},
			},
			anchor: "cite_note-42",
			wantY:  600,
			wantOk: true,
		},
		{
			name: "cite_ref to cite_note",
			layout: PageLayout{
				anchorPositions: map[string]int32{"#cite-note-42": 700},
			},
			anchor: "cite_ref-42",
			wantY:  700,
			wantOk: true,
		},
		{
			name: "cite_ref-N to cite_note strips suffix",
			layout: PageLayout{
				anchorPositions: map[string]int32{"#cite-note-42": 800},
			},
			anchor: "cite_ref-42-1",
			wantY:  800,
			wantOk: true,
		},
		{
			name: "not found",
			layout: PageLayout{
				anchorPositions: map[string]int32{},
			},
			anchor: "nonexistent",
			wantY:  0,
			wantOk: false,
		},
		{
			name: "cite_note partial match with prefix",
			layout: PageLayout{
				anchorPositions: map[string]int32{"#cite-ref-42-1": 900},
			},
			anchor: "cite_note-42",
			wantY:  900,
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Renderer{
				layout: tt.layout,
			}
			gotY, gotOk := r.FindAnchorY(tt.anchor)
			if gotOk != tt.wantOk {
				t.Errorf("FindAnchorY(%q) ok=%v, want %v", tt.anchor, gotOk, tt.wantOk)
			}
			if gotY != tt.wantY {
				t.Errorf("FindAnchorY(%q) y=%d, want %d", tt.anchor, gotY, tt.wantY)
			}
		})
	}
}
