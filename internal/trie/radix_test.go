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
	// Single-child chains auto-drill. XYZ Alpha/Beta/Gamma all share prefix.
	articles := []zim.ArticleEntry{
		{Title: "XYZ Alpha", Path: "A/Alpha"},
		{Title: "XYZ Beta", Path: "A/Beta"},
		{Title: "XYZ Gamma", Path: "A/Gamma"},
	}

	root := NewTree(articles)
	x := root.children[0]
	if !x.Expanded() { t.Fatal("X auto-drilled") }
	if len(x.children) != 1 { t.Fatalf("X→1 child, got %d", len(x.children)) }

	xy := x.children[0]
	if !xy.Expanded() { t.Fatal("XY auto-drilled") }
	if len(xy.children) != 1 { t.Fatalf("XY→1 child, got %d", len(xy.children)) }

	xyz := xy.children[0]
	if !xyz.Expanded() { t.Fatal("XYZ auto-drilled") }
	if len(xyz.children) != 1 { t.Fatalf("XYZ→1 child, got %d", len(xyz.children)) }

	spaceGrp := xyz.children[0]
	if !spaceGrp.Expanded() { t.Fatal("'XYZ ' auto-drilled") }
	if len(spaceGrp.children) != 3 { t.Fatalf("space→3 leaves, got %d", len(spaceGrp.children)) }
	for _, c := range spaceGrp.children {
		if !c.IsLeaf() { t.Errorf("expected leaf, got %q", c.Label()) }
	}
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
	// Drills single-child chains, stops at first branching.
	articles := []zim.ArticleEntry{
		{Title: "A X Alpha", Path: "A"},
		{Title: "A X Beta", Path: "B"},
		{Title: "A Y Gamma", Path: "C"},
	}
	root := NewTree(articles)
	// Root child "A" → 1 child "A " (space group)
	a := root.children[0]
	if !a.Expanded() { t.Fatal("A auto-drilled") }
	if len(a.children) != 1 { t.Fatalf("A→1 child, got %d", len(a.children)) }

	// "A " → 2 children (branching: X and Y)
	sp := a.children[0]
	if !sp.Expanded() { t.Fatal("'A ' auto-drilled") }
	if len(sp.children) != 2 { t.Fatalf("'A '→2 children, got %d", len(sp.children)) }

	xGrp := sp.children[0] // "A X": 2 articles
	if xGrp.IsLeaf() { t.Error("A X should NOT be leaf (2 articles)") }
	if xGrp.Expanded() { t.Error("A X should NOT be expanded (2 children → stop)") }

	yLeaf := sp.children[1] // "A Y Gamma": 1 article → leaf
	if !yLeaf.IsLeaf() { t.Error("A Y Gamma should be leaf") }
}
