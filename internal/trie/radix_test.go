package trie

import (
	"testing"

	"github.com/kiwix-sdl/kiwix-sdl/internal/zim"
)

func TestNewTree(t *testing.T) {
	articles := []zim.ArticleEntry{
		{Title: "Apple", Path: "A/Apple"},
		{Title: "Apricot", Path: "A/Apricot"},
		{Title: "Banana", Path: "A/Banana"},
		{Title: "Cherry", Path: "A/Cherry"},
	}

	root := NewTree(articles)
	if len(root.children) != 3 {
		t.Fatalf("expected 3 first-level groups (A, B, C), got %d", len(root.children))
	}

	aNode := root.children[0]
	if aNode.Label() != "A" {
		t.Errorf("A label: got %q", aNode.Label())
	}
	if aNode.Suffix() != "2" {
		t.Errorf("A suffix: got %q", aNode.Suffix())
	}

	bNode := root.children[1]
	if bNode.Label() != "Banana" {
		t.Errorf("B label: got %q", bNode.Label())
	}
	if !bNode.IsLeaf() {
		t.Error("B should be leaf")
	}

	// Expand A.
	aNode.Expand()
	if !aNode.Expanded() {
		t.Fatal("A should be expanded")
	}
	// "Apple" and "Apricot" share "p" second letter → grouped under "Ap".
	if len(aNode.children) < 1 {
		t.Errorf("A should have children, got %d", len(aNode.children))
	}

	// Collapse.
	aNode.Collapse()
	if aNode.Expanded() {
		t.Error("A should be collapsed")
	}
}

func TestNavStateNew(t *testing.T) {
	articles := []zim.ArticleEntry{
		{Title: "A", Path: "A"},
		{Title: "B", Path: "B"},
		{Title: "C", Path: "C"},
	}

	root := NewTree(articles)
	ns := NewNavState(root)

	if ns.Cursor.Label() != "A" {
		t.Errorf("initial: got %q", ns.Cursor.Label())
	}
	ns.MoveDown()
	if ns.Cursor.Label() != "B" {
		t.Errorf("down: got %q", ns.Cursor.Label())
	}
	ns.MoveDown()
	if ns.Cursor.Label() != "C" {
		t.Errorf("down2: got %q", ns.Cursor.Label())
	}
	ns.MoveDown() // stay at C
	if ns.Cursor.Label() != "C" {
		t.Errorf("wrap: got %q", ns.Cursor.Label())
	}
	ns.MoveUp()
	if ns.Cursor.Label() != "B" {
		t.Errorf("up: got %q", ns.Cursor.Label())
	}
}

func TestCyrillic(t *testing.T) {
	articles := []zim.ArticleEntry{
		{Title: "Абрикос", Path: "A/Абрикос"},
		{Title: "Ананас", Path: "A/Ананас"},
		{Title: "Банан", Path: "A/Банан"},
	}

	root := NewTree(articles)
	if len(root.children) != 2 {
		t.Fatalf("expected 2 groups (А, Б), got %d", len(root.children))
	}
	if root.children[0].Label() != "А" {
		t.Errorf("first: got %q", root.children[0].Label())
	}
	if root.children[1].Label() != "Банан" {
		t.Errorf("second: got %q", root.children[1].Label())
	}
}

func TestAutoDrill(t *testing.T) {
	// Single-child chains auto-drill on manual Expand(), not on tree load.
	articles := []zim.ArticleEntry{
		{Title: "XYZ Alpha", Path: "A/Alpha"},
		{Title: "XYZ Beta", Path: "A/Beta"},
		{Title: "XYZ Gamma", Path: "A/Gamma"},
	}

	root := NewTree(articles)
	x := root.children[0]
	// Root load: NOT expanded.
	if x.Expanded() {
		t.Fatal("X should NOT be expanded on load")
	}

	// Manual expand: drills through single-child chain to leaves.
	x.Expand()
	if !x.Expanded() { t.Fatal("X should be expanded after manual Expand()") }
	if len(x.children) != 1 { t.Fatalf("X→1 child (XY), got %d", len(x.children)) }
	if !x.children[0].Expanded() { t.Fatal("XY auto-drilled") }
}

func TestMultiBranchStops(t *testing.T) {
	// Multiple children at first level → no auto-drill.
	articles := []zim.ArticleEntry{
		{Title: "Alpha", Path: "A"},
		{Title: "Beta", Path: "B"},
		{Title: "Gamma", Path: "C"},
	}
	root := NewTree(articles)
	if len(root.children) != 3 { t.Fatalf("3 groups, got %d", len(root.children)) }
	for _, c := range root.children {
		if c.Expanded() {
			t.Errorf("%q should NOT be auto-expanded (3 children at root)", c.Label())
		}
	}
}

func TestMultiBranchAfterDrill(t *testing.T) {
	// Manual Expand() drills single-child chain, stops at first branching.
	articles := []zim.ArticleEntry{
		{Title: "A X Alpha", Path: "A"},
		{Title: "A X Beta", Path: "B"},
		{Title: "A Y Gamma", Path: "C"},
	}
	root := NewTree(articles)
	a := root.children[0]
	if a.Expanded() { t.Fatal("A should NOT be expanded on load") }

	// Manually expand — drills through single-child "A " to branching.
	a.Expand()
	if !a.Expanded() { t.Fatal("A drilled") }
	if len(a.children) != 1 { t.Fatalf("A→1 child, got %d", len(a.children)) }

	sp := a.children[0] // "A "
	if !sp.Expanded() { t.Fatal("'A ' drilled") }
	if len(sp.children) != 2 { t.Fatalf("'A '→2 children (X,Y), got %d", len(sp.children)) }

	// X group has 2 children → stop, Y leaf → drilled.
	xGrp := sp.children[0]
	if xGrp.IsLeaf() { t.Error("A X should NOT be leaf") }
	if xGrp.Expanded() { t.Error("A X should NOT be expanded (2 children, stop)") }

	yLeaf := sp.children[1]
	if !yLeaf.IsLeaf() { t.Error("A Y Gamma should be leaf") }
}
