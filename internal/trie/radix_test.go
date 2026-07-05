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
	// Single-child chains are absorbed on manual Expand(), not on tree load.
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

	// Manual expand: drills through single-child chain, absorbing it.
	x.Expand()
	if !x.Expanded() {
		t.Fatal("X should be expanded after manual Expand()")
	}
	// Should have absorbed "XY" and "XYZ " and directly have the 3 leaf children.
	if x.prefix != "XYZ " {
		t.Errorf("expected absorbed prefix %q, got %q", "XYZ ", x.prefix)
	}
	if len(x.children) != 3 {
		t.Fatalf("expected 3 children after absorption, got %d", len(x.children))
	}
	for _, child := range x.children {
		if !child.IsLeaf() {
			t.Errorf("child %q should be leaf", child.Label())
		}
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
	if len(root.children) != 3 {
		t.Fatalf("3 groups, got %d", len(root.children))
	}
	for _, c := range root.children {
		if c.Expanded() {
			t.Errorf("%q should NOT be auto-expanded (3 children at root)", c.Label())
		}
	}
}

func TestMultiBranchAfterDrill(t *testing.T) {
	// Manual Expand() drills single-child chain and absorbs it, stopping at first branching.
	articles := []zim.ArticleEntry{
		{Title: "A X Alpha", Path: "A"},
		{Title: "A X Beta", Path: "B"},
		{Title: "A Y Gamma", Path: "C"},
	}
	root := NewTree(articles)
	a := root.children[0]
	if a.Expanded() {
		t.Fatal("A should NOT be expanded on load")
	}

	// Manually expand — drills through single-child "A " to branching, absorbing "A ".
	a.Expand()
	if !a.Expanded() {
		t.Fatal("A drilled")
	}
	if a.prefix != "A " {
		t.Errorf("expected absorbed prefix %q, got %q", "A ", a.prefix)
	}
	if len(a.children) != 2 {
		t.Fatalf("A should have 2 children (X, Y) after absorbing, got %d", len(a.children))
	}

	// X group has 2 children → stop (not expanded, not leaf)
	xGrp := a.children[0]
	if xGrp.IsLeaf() {
		t.Error("A X should NOT be leaf")
	}
	if xGrp.Expanded() {
		t.Error("A X should NOT be expanded (2 children, stop)")
	}
	if xGrp.prefix != "A X" {
		t.Errorf("expected X prefix %q, got %q", "A X", xGrp.prefix)
	}

	// Y group has 1 child which is leaf -> since it's a leaf, it was NOT expanded/absorbed as a parent, but it itself is a leaf
	yLeaf := a.children[1]
	if !yLeaf.IsLeaf() {
		t.Error("A Y Gamma should be leaf")
	}
}

func TestFlatNavigation(t *testing.T) {
	articles := []zim.ArticleEntry{
		{Title: "Apple", Path: "A/Apple"},
		{Title: "Apricot", Path: "A/Apricot"},
		{Title: "Banana", Path: "A/Banana"},
	}
	root := NewTree(articles)
	// Root has "A" (group with 2 articles) and "Banana" (leaf)
	ns := NewNavState(root)

	if ns.Cursor.Label() != "A" {
		t.Fatalf("expected initial cursor to be 'A', got %q", ns.Cursor.Label())
	}

	// Move down -> should go to Banana (since A is collapsed)
	ns.MoveDown()
	if ns.Cursor.Label() != "Banana" {
		t.Fatalf("expected cursor to go to 'Banana', got %q", ns.Cursor.Label())
	}

	// Go back up to A and expand it
	ns.MoveUp()
	ns.ActionRight() // expands A and moves to first child "Apple"
	if ns.Cursor.Label() != "Apple" {
		t.Fatalf("expected expanded cursor on 'Apple', got %q", ns.Cursor.Label())
	}

	// Move down -> should go to Apricot (sibling)
	ns.MoveDown()
	if ns.Cursor.Label() != "Apricot" {
		t.Fatalf("expected cursor on 'Apricot', got %q", ns.Cursor.Label())
	}

	// Move down again -> since A is expanded, from last child of A ("Apricot") we should go to "Banana"
	ns.MoveDown()
	if ns.Cursor.Label() != "Banana" {
		t.Fatalf("expected flat navigation down to 'Banana', got %q", ns.Cursor.Label())
	}
}

func TestActionLeftRight(t *testing.T) {
	articles := []zim.ArticleEntry{
		{Title: "Apple", Path: "A/Apple"},
		{Title: "Apricot", Path: "A/Apricot"},
		{Title: "Banana", Path: "A/Banana"},
	}
	root := NewTree(articles)
	ns := NewNavState(root)

	// 1. Right on collapsed node -> expands and enters
	ns.ActionRight()
	if !ns.Root.children[0].Expanded() {
		t.Error("expected 'A' to be expanded")
	}
	if ns.Cursor.Label() != "Apple" {
		t.Errorf("expected cursor to enter 'Apple', got %q", ns.Cursor.Label())
	}

	// 2. Left on leaf -> goes to parent 'Ap' (originally 'A', but absorbed to 'Ap' during expansion)
	ns.ActionLeft()
	if ns.Cursor.Label() != "Ap" {
		t.Errorf("expected left on leaf to go to parent 'Ap', got %q", ns.Cursor.Label())
	}

	// 3. Left on expanded node 'Ap' -> collapses it
	ns.ActionLeft()
	if ns.Root.children[0].Expanded() {
		t.Error("expected left on expanded 'Ap' to collapse it")
	}
	if ns.Cursor.Label() != "Ap" {
		t.Errorf("expected cursor to remain on 'Ap', got %q", ns.Cursor.Label())
	}
}
