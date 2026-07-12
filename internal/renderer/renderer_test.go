package renderer

import (
	"testing"
)

// --- linkHasVisibleRect ---

func TestLinkHasVisibleRect(t *testing.T) {
	tests := []struct {
		name   string
		rects  []sdlRect
		top    int32
		bottom int32
		want   bool
	}{
		{
			name:  "rect fully inside",
			rects: []sdlRect{{X: 0, Y: 50, W: 100, H: 20}},
			top:   30, bottom: 200,
			want: true,
		},
		{
			name:  "rect above viewport",
			rects: []sdlRect{{X: 0, Y: 10, W: 100, H: 20}},
			top:   50, bottom: 200,
			want: false,
		},
		{
			name:  "rect below viewport",
			rects: []sdlRect{{X: 0, Y: 300, W: 100, H: 20}},
			top:   50, bottom: 200,
			want: false,
		},
		{
			name:  "rect straddles top edge",
			rects: []sdlRect{{X: 0, Y: 40, W: 100, H: 30}},
			top:   50, bottom: 200,
			want: true,
		},
		{
			name:  "rect straddles bottom edge",
			rects: []sdlRect{{X: 0, Y: 190, W: 100, H: 30}},
			top:   50, bottom: 200,
			want: true,
		},
		{
			name:  "multi-rect second rect inside",
			rects: []sdlRect{{X: 0, Y: 10, W: 100, H: 20}, {X: 0, Y: 60, W: 50, H: 20}},
			top:   50, bottom: 200,
			want: true,
		},
		{
			name:  "empty rects",
			rects: []sdlRect{},
			top:   0, bottom: 100,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link := linkEntry{rects: tt.rects}
			got := linkHasVisibleRect(link, tt.top, tt.bottom)
			if got != tt.want {
				t.Errorf("linkHasVisibleRect(%+v, %d, %d) = %v, want %v",
					tt.rects, tt.top, tt.bottom, got, tt.want)
			}
		})
	}
}

// --- forwardLinkIdx / backwardLinkIdx ---

func mkLink(ys ...int32) linkEntry {
	var rects []sdlRect
	for _, y := range ys {
		rects = append(rects, sdlRect{X: 0, Y: y, W: 100, H: 20})
	}
	return linkEntry{rects: rects, url: ""}
}

func TestForwardLinkIdx(t *testing.T) {
	// links at Y: 10, 100, 200, 500, 700
	links := []linkEntry{mkLink(10), mkLink(100), mkLink(200), mkLink(500), mkLink(700)}

	tests := []struct {
		name        string
		top, bottom int32
		want        int
	}{
		{
			name: "first visible in viewport",
			top:  50, bottom: 300,
			want: 1, // link[1] at Y=100 is first visible
		},
		{
			name: "multi-rect visible even when first rect above",
			top:  80, bottom: 300,
			want: 1, // link[1] has rect Y=100 >= 80 → visible via any-rect check
		},
		{
			name: "no visible, pick closest below",
			top:  300, bottom: 400,
			want: 3, // link[3] at Y=500 is first below viewport
		},
		{
			name: "all links above viewport",
			top:  800, bottom: 1000,
			want: 4, // fallback to last
		},
		{
			name: "empty viewport above all links",
			top:  0, bottom: 5,
			want: 0, // link[0] at Y=10 has rect.Y=10 >= 0 → first with rect.Y >= top
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := forwardLinkIdx(links, tt.top, tt.bottom)
			if got != tt.want {
				t.Errorf("forwardLinkIdx(top=%d, bottom=%d) = %d, want %d",
					tt.top, tt.bottom, got, tt.want)
			}
		})
	}
}

func TestBackwardLinkIdx(t *testing.T) {
	links := []linkEntry{mkLink(10), mkLink(100), mkLink(200), mkLink(500), mkLink(700)}

	tests := []struct {
		name        string
		top, bottom int32
		want        int
	}{
		{
			name: "last visible in viewport",
			top:  50, bottom: 400,
			want: 2, // link[2] at Y=200 is last visible
		},
		{
			name: "no visible, pick closest above",
			top:  250, bottom: 350,
			want: 2, // link[2] ends at 220 <= 350 → closest above
		},
		{
			name: "rect ending inside viewport is visible (caught by pass 1 not fallback)",
			top:  100, bottom: 120,
			want: 1, // only link[1] Y=100 bottom=120 is visible; pass 1 backward finds it
		},
		{
			name: "all links below viewport",
			top:  0, bottom: 5,
			want: 0, // fallback to first
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backwardLinkIdx(links, tt.top, tt.bottom)
			if got != tt.want {
				t.Errorf("backwardLinkIdx(top=%d, bottom=%d) = %d, want %d",
					tt.top, tt.bottom, got, tt.want)
			}
		})
	}
}

func TestBackwardLinkIdx_multiRect(t *testing.T) {
	// link[1] is multi-rect: first rect above viewport, second rect in viewport
	links := []linkEntry{
		mkLink(10),
		{rects: []sdlRect{
			{X: 0, Y: 130, W: 100, H: 20},
			{X: 0, Y: 160, W: 50, H: 20},
		}},
		mkLink(300),
	}
	got := backwardLinkIdx(links, 140, 250) // second rect of link[1] is visible
	if got != 1 {
		t.Errorf("expected multi-rect visible link (1), got %d", got)
	}
}

// --- moveLink — Renderer integration ---

func mkScrollTest(links []linkEntry, scrollY int32) *Renderer {
	return &Renderer{
		layout: PageLayout{
			links:       links,
			totalHeight: 1000,
		},
		scrollY:      scrollY,
		marginY:      10,
		height:       300,
		selectedLink: 0,
	}
}

func TestMoveLink_forward_jump_to_visible(t *testing.T) {
	r := mkScrollTest([]linkEntry{
		mkLink(10),  // 0: above viewport
		mkLink(100), // 1: visible (visibleTop=50, visibleBottom=266)
		mkLink(200), // 2: visible
		mkLink(500), // 3: below
	}, 40 /* scrollY=40 → visibleTop=50 */)
	r.selectedLink = 0 // not visible

	r.moveLink(+1)

	if r.selectedLink != 1 {
		t.Errorf("expected first visible link (1), got %d", r.selectedLink)
	}
}

func TestMoveLink_forward_jump_multirect_partial(t *testing.T) {
	// link[0] is multi-rect: first rect above viewport, second rect inside viewport
	links := []linkEntry{
		mkLink(10), // 0: simple, above
		{rects: []sdlRect{ // 1: multi-rect, first rect above, second inside
			{X: 0, Y: 30, W: 100, H: 20},
			{X: 0, Y: 60, W: 50, H: 20},
		}},
		mkLink(200), // 2: visible but after link[1]
	}
	r := mkScrollTest(links, 40) // visibleTop=50, visibleBottom=266
	r.selectedLink = 0           // link[0] above viewport

	r.moveLink(+1)

	if r.selectedLink != 1 {
		t.Errorf("expected multi-rect partially visible link (1), got %d", r.selectedLink)
	}
}

func TestMoveLink_forward_fallback_below(t *testing.T) {
	links := []linkEntry{mkLink(10), mkLink(30), mkLink(50)}
	r := mkScrollTest(links, 200) // visibleTop=210, visibleBottom=466; all links above
	r.selectedLink = 0

	r.moveLink(+1)

	if r.selectedLink != 2 {
		t.Errorf("expected fallback to last link (2), got %d", r.selectedLink)
	}
}

func TestMoveLink_forward_normal_increment(t *testing.T) {
	r := mkScrollTest([]linkEntry{mkLink(10), mkLink(100), mkLink(200)}, 0)
	r.selectedLink = 0 // visible: Y=10, visibleTop=10

	r.moveLink(+1)

	if r.selectedLink != 1 {
		t.Errorf("expected increment to 1, got %d", r.selectedLink)
	}
}

func TestMoveLink_no_selection(t *testing.T) {
	r := mkScrollTest([]linkEntry{mkLink(10)}, 0)
	r.selectedLink = -1

	r.moveLink(+1)

	if r.selectedLink != 0 {
		t.Errorf("expected first link, got %d", r.selectedLink)
	}
}

func TestMoveLink_no_links(t *testing.T) {
	r := mkScrollTest(nil, 0)
	r.selectedLink = -1

	r.moveLink(+1) // should not panic
}

func TestMoveLink_backward_jump_to_visible(t *testing.T) {
	links := []linkEntry{
		mkLink(10),  // 0: above viewport
		mkLink(100), // 1: visible
		mkLink(200), // 2: visible
		mkLink(500), // 3: below viewport
	}
	r := mkScrollTest(links, 40)
	r.selectedLink = 3 // below viewport

	r.moveLink(-1)

	if r.selectedLink != 2 {
		t.Errorf("expected last visible link (2), got %d", r.selectedLink)
	}
}

func TestMoveLink_backward_fallback_above(t *testing.T) {
	links := []linkEntry{mkLink(10), mkLink(30), mkLink(50)}
	r := mkScrollTest(links, 200) // all links above viewport
	r.selectedLink = 2            // not visible

	r.moveLink(-1)

	if r.selectedLink != 2 { // closest above: link[2] ends at 70 <= 466 → caught by pass 2
		t.Errorf("expected walk-back to last link ending in viewport (2), got %d", r.selectedLink)
	}
}

func TestMoveLink_backward_all_below(t *testing.T) {
	links := []linkEntry{mkLink(500), mkLink(700)}
	r := mkScrollTest(links, 0) // visibleTop=10, visibleBottom=266; all links below
	r.selectedLink = 0          // not visible (Y=500 > visibleBottom)

	r.moveLink(-1)

	// All links below viewport, backward pass 2 finds nothing, falls back to 0
	if r.selectedLink != 0 {
		t.Errorf("expected fallback to first link (0), got %d", r.selectedLink)
	}
}

func TestMoveLink_backward_normal_decrement(t *testing.T) {
	r := mkScrollTest([]linkEntry{mkLink(10), mkLink(100), mkLink(200)}, 0)
	r.selectedLink = 1

	r.moveLink(-1)

	if r.selectedLink != 0 {
		t.Errorf("expected decrement to 0, got %d", r.selectedLink)
	}
}

func TestMoveLink_scrollY_adjusted(t *testing.T) {
	links := []linkEntry{mkLink(10), mkLink(500)}
	r := mkScrollTest(links, 0)
	r.selectedLink = 0

	r.moveLink(+1) // select link[1] at Y=500

	// scrollY should be adjusted to show link[1]
	if r.scrollY <= 0 {
		t.Errorf("expected scrollY > 0 after scrolling to link at Y=500, got %d", r.scrollY)
	}
}

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
