package trie

import (
	"testing"

	"github.com/kiwix-sdl/zimlite/internal/document"
)

func TestNewTree(t *testing.T) {
	articles := []document.ArticleEntry{
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
	articles := []document.ArticleEntry{
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
	articles := []document.ArticleEntry{
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
	articles := []document.ArticleEntry{
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
	articles := []document.ArticleEntry{
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
	articles := []document.ArticleEntry{
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
	articles := []document.ArticleEntry{
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

	// Move down -> should go to Banana (sibling at root level)
	ns.MoveDown()
	if ns.Cursor.Label() != "Banana" {
		t.Fatalf("expected cursor to go to 'Banana', got %q", ns.Cursor.Label())
	}

	// Move up -> back to A
	ns.MoveUp()
	if ns.Cursor.Label() != "A" {
		t.Fatalf("expected cursor to go back to 'A', got %q", ns.Cursor.Label())
	}

	// Expand A and enter
	ns.ActionRight()
	if ns.Cursor.Label() != "Apple" {
		t.Fatalf("expected expanded cursor on 'Apple', got %q", ns.Cursor.Label())
	}

	// Move down within A's children -> should go to Apricot (sibling)
	ns.MoveDown()
	if ns.Cursor.Label() != "Apricot" {
		t.Fatalf("expected cursor on 'Apricot', got %q", ns.Cursor.Label())
	}

	// Move down again -> should stay at Apricot (last sibling)
	ns.MoveDown()
	if ns.Cursor.Label() != "Apricot" {
		t.Fatalf("expected cursor to stay on 'Apricot', got %q", ns.Cursor.Label())
	}

	// Move up within A's children -> back to Apple
	ns.MoveUp()
	if ns.Cursor.Label() != "Apple" {
		t.Fatalf("expected cursor on 'Apple', got %q", ns.Cursor.Label())
	}

	// Move up again -> should stay at Apple (first sibling)
	ns.MoveUp()
	if ns.Cursor.Label() != "Apple" {
		t.Fatalf("expected cursor to stay on 'Apple', got %q", ns.Cursor.Label())
	}
}

func TestActionLeftRight(t *testing.T) {
	articles := []document.ArticleEntry{
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

	// 2. Left from child -> move to parent, collapse parent's branch
	ns.ActionLeft()
	if ns.Root.children[0].Expanded() {
		t.Error("expected 'A' to be collapsed after left")
	}
	if ns.Cursor.Label() != "A" {
		t.Errorf("expected cursor on 'A', got %q", ns.Cursor.Label())
	}

	// 3. Left at first level -> collapses all branches, cursor stays
	ns.ActionLeft()
	if ns.Root.children[0].Expanded() {
		t.Error("expected 'A' to remain collapsed")
	}
	if ns.Cursor.Label() != "A" {
		t.Errorf("expected cursor to stay on 'A', got %q", ns.Cursor.Label())
	}
}

func TestCollapseReExpand(t *testing.T) {
	articles := []document.ArticleEntry{
		{Title: "Apple", Path: "A/Apple"},
		{Title: "Apricot", Path: "A/Apricot"},
		{Title: "Banana", Path: "A/Banana"},
	}
	root := NewTree(articles)
	ns := NewNavState(root)

	// Expand first-level node A
	ns.ActionRight()
	aNode := ns.Root.children[0]
	if !aNode.Expanded() {
		t.Fatal("A should be expanded")
	}
	childCount := len(aNode.children)
	if childCount == 0 {
		t.Fatal("A should have children after expand")
	}

	// Left from child -> move to parent, collapse it
	ns.ActionLeft()
	if aNode.Expanded() {
		t.Error("A should be collapsed")
	}
	if ns.Cursor.Label() != "A" {
		t.Errorf("expected cursor on 'A', got %q", ns.Cursor.Label())
	}

	// Re-expand
	ns.ActionRight()
	if !aNode.Expanded() {
		t.Error("A should be re-expanded")
	}
	if len(aNode.children) != childCount {
		t.Errorf("A should have same children count after re-expand: got %d, want %d", len(aNode.children), childCount)
	}

	// Cursor should be on first child
	if ns.Cursor.Label() != "Apple" {
		t.Errorf("cursor should be on first child after re-expand, got %q", ns.Cursor.Label())
	}
}

func TestCollapseReExpandPreservesPrefix(t *testing.T) {
	// After auto-drill absorbs a child, the prefix changes (e.g., "A" → "Ap").
	// Collapse should restore the original prefix so re-expand produces the same tree.
	articles := []document.ArticleEntry{
		{Title: "Apple", Path: "A/Apple"},
		{Title: "Apricot", Path: "A/Apricot"},
		{Title: "Banana", Path: "A/Banana"},
	}
	root := NewTree(articles)
	ns := NewNavState(root)

	aNode := ns.Root.children[0]

	// Record prefix before first expand.
	originalPrefix := aNode.prefix

	// Expand: auto-drill absorbs and prefix changes.
	ns.ActionRight()
	drilledPrefix := aNode.prefix
	if drilledPrefix == originalPrefix {
		t.Skip("auto-drill did not change prefix, test N/A")
	}

	// Capture first expansion's children structure.
	firstChildren := make([]string, len(aNode.children))
	for i, c := range aNode.children {
		firstChildren[i] = c.Label()
	}

	// Left from child -> collapse A, prefix restores.
	ns.ActionLeft()
	if aNode.prefix != originalPrefix {
		t.Errorf("after collapse, prefix should be %q, got %q", originalPrefix, aNode.prefix)
	}

	// Re-expand and check same structure.
	ns.ActionRight()
	if aNode.prefix != drilledPrefix {
		t.Errorf("after re-expand, prefix should be %q, got %q", drilledPrefix, aNode.prefix)
	}
	reChildren := make([]string, len(aNode.children))
	for i, c := range aNode.children {
		reChildren[i] = c.Label()
	}
	if len(firstChildren) != len(reChildren) {
		t.Fatalf("children count mismatch: first=%d, re=%d", len(firstChildren), len(reChildren))
	}
	for i := range firstChildren {
		if firstChildren[i] != reChildren[i] {
			t.Errorf("child[%d] mismatch: first=%q, re=%q", i, firstChildren[i], reChildren[i])
		}
	}
}

func TestSiblingNavigationDeep(t *testing.T) {
	// Build tree with multi-level siblings.
	articles := []document.ArticleEntry{
		{Title: "A Alpha", Path: "A/Alpha"},
		{Title: "A Beta", Path: "A/Beta"},
		{Title: "A Gamma", Path: "A/Gamma"},
		{Title: "B Delta", Path: "B/Delta"},
	}
	root := NewTree(articles)
	ns := NewNavState(root)

	// Cursor starts at first root child (A)
	if ns.Cursor.parent != root {
		t.Fatal("cursor should be at root level")
	}

	// Expand A
	ns.ActionRight()
	aNode := root.children[0]
	if !aNode.Expanded() {
		t.Fatal("A should be expanded")
	}
	// Cursor on first child of A
	if ns.Cursor.parent != aNode {
		t.Fatalf("cursor should be inside A, got %q (parent=%q)", ns.Cursor.Label(), ns.Cursor.parent.Label())
	}

	// MoveDown within A: Apple -> Beta
	ns.MoveDown()
	if ns.Cursor.Label() != "A Beta" {
		t.Errorf("expected 'A Beta', got %q", ns.Cursor.Label())
	}

	// MoveDown within A: Beta -> Gamma
	ns.MoveDown()
	if ns.Cursor.Label() != "A Gamma" {
		t.Errorf("expected 'A Gamma', got %q", ns.Cursor.Label())
	}

	// MoveDown on last sibling -> no-op
	ns.MoveDown()
	if ns.Cursor.Label() != "A Gamma" {
		t.Errorf("expected stay on 'A Gamma', got %q", ns.Cursor.Label())
	}

	// MoveUp within A: Gamma -> Beta
	ns.MoveUp()
	if ns.Cursor.Label() != "A Beta" {
		t.Errorf("expected 'A Beta', got %q", ns.Cursor.Label())
	}

	// MoveUp within A: Beta -> Apple
	ns.MoveUp()
	if ns.Cursor.Label() != "A Alpha" {
		t.Errorf("expected 'A Alpha', got %q", ns.Cursor.Label())
	}

	// MoveUp on first sibling -> no-op
	ns.MoveUp()
	if ns.Cursor.Label() != "A Alpha" {
		t.Errorf("expected stay on 'A Alpha', got %q", ns.Cursor.Label())
	}
}

func TestActionLeftCollapseParent(t *testing.T) {
	// Left from a child should move to parent and collapse the parent.
	articles := []document.ArticleEntry{
		{Title: "A Alpha", Path: "A/Alpha"},
		{Title: "A Beta", Path: "A/Beta"},
		{Title: "A Gamma", Path: "A/Gamma"},
		{Title: "B Delta", Path: "B/Delta"},
	}
	root := NewTree(articles)
	ns := NewNavState(root)

	// Expand A
	ns.ActionRight()
	aNode := root.children[0]
	if !aNode.Expanded() {
		t.Fatal("A should be expanded")
	}

	// Move to second child
	ns.MoveDown()
	if ns.Cursor.Label() != "A Beta" {
		t.Fatalf("expected 'A Beta', got %q", ns.Cursor.Label())
	}

	// Left: cursor -> A, A collapsed
	ns.ActionLeft()
	if aNode.Expanded() {
		t.Error("A should be collapsed after left")
	}
	if ns.Cursor.Label() != "A" {
		t.Errorf("expected cursor on 'A', got %q", ns.Cursor.Label())
	}
}

func TestActionLeftAtFirstLevel(t *testing.T) {
	// Left at first level collapses all branches, cursor stays.
	articles := []document.ArticleEntry{
		{Title: "A Alpha", Path: "A/Alpha"},
		{Title: "A Beta", Path: "A/Beta"},
		{Title: "B Gamma", Path: "B/Gamma"},
		{Title: "B Delta", Path: "B/Delta"},
	}
	root := NewTree(articles)
	ns := NewNavState(root)

	// Expand both A and B
	root.children[0].Expand()
	root.children[1].Expand()

	// Verify both expanded
	if !root.children[0].Expanded() || !root.children[1].Expanded() {
		t.Fatal("both A and B should be expanded")
	}

	// Left at first level -> all branches collapse
	ns.ActionLeft()
	if root.children[0].Expanded() {
		t.Error("A should be collapsed after left at first level")
	}
	if root.children[1].Expanded() {
		t.Error("B should be collapsed after left at first level")
	}
	if ns.Cursor.Label() != "A" {
		t.Errorf("cursor should stay on 'A', got %q", ns.Cursor.Label())
	}
}

func TestActionLeftDeepThenRight(t *testing.T) {
	// Deep navigation: Right→Right, then Left→Left, then Right→Right again.
	articles := []document.ArticleEntry{
		{Title: "A Alpha", Path: "A/Alpha"},
		{Title: "A Beta", Path: "A/Beta"},
		{Title: "A Gamma", Path: "A/Gamma"},
		{Title: "B Delta", Path: "B/Delta"},
	}
	root := NewTree(articles)
	ns := NewNavState(root)

	// Right: expand A, enter first child
	ns.ActionRight()
	aNode := root.children[0]
	if ns.Cursor.Label() != "A Alpha" {
		t.Fatalf("expected 'A Alpha', got %q", ns.Cursor.Label())
	}

	// Left: collapse A, cursor -> A
	ns.ActionLeft()
	if aNode.Expanded() {
		t.Error("A should be collapsed")
	}
	if ns.Cursor.Label() != "A" {
		t.Errorf("expected 'A', got %q", ns.Cursor.Label())
	}

	// Right: re-expand A, enter first child
	ns.ActionRight()
	if !aNode.Expanded() {
		t.Error("A should be re-expanded")
	}
	if ns.Cursor.Label() != "A Alpha" {
		t.Errorf("expected 'A Alpha' after re-expand, got %q", ns.Cursor.Label())
	}
}

// TestHybridNodeExpands verifies that a node whose title exactly matches a
// branch prefix (e.g. "Cat" alongside "Category"/"Catalog") behaves as a
// branch: it is Expandable, Right/select expands instead of opening the
// article, and the exact-match article stays reachable as a leaf child.
func TestHybridNodeExpands(t *testing.T) {
	articles := []document.ArticleEntry{
		{Title: "Cat", Path: "A/Cat"},
		{Title: "Catalog", Path: "A/Catalog"},
		{Title: "Category", Path: "A/Category"},
	}
	root := NewTree(articles)
	ns := NewNavState(root)

	// Enter the 'C' first-level group.
	ns.ActionRight()

	// Locate the hybrid node (prefix "Cat").
	var cat *RadixNode
	var findCat func(n *RadixNode)
	findCat = func(n *RadixNode) {
		if n.prefix == "Cat" && n.leaf == nil {
			cat = n
			return
		}
		for _, c := range n.children {
			findCat(c)
		}
	}
	findCat(root)
	if cat == nil {
		t.Fatal("expected a branch node with prefix 'Cat'")
	}

	// The hybrid node must be a branch, not a leaf.
	if cat.IsLeaf() {
		t.Error("hybrid 'Cat' node must not be a leaf")
	}
	if !cat.Expandable() {
		t.Error("hybrid 'Cat' node must be expandable")
	}

	// The exact-match article "Cat" must remain reachable as a leaf child.
	cat.Expand()
	var catArticle *RadixNode
	for _, c := range cat.children {
		if c.IsLeaf() && c.FullPath() == "A/Cat" {
			catArticle = c
		}
	}
	if catArticle == nil {
		t.Error("exact-match article 'Cat' must be reachable as a leaf child")
	}
}

func TestActionLeftCollapseOnlyExpanded(t *testing.T) {
	// Left at first level only collapses expanded branches.
	articles := []document.ArticleEntry{
		{Title: "A Alpha", Path: "A/Alpha"},
		{Title: "A Beta", Path: "A/Beta"},
		{Title: "B Gamma", Path: "B/Gamma"},
	}
	root := NewTree(articles)
	ns := NewNavState(root)

	// Only expand A, leave B collapsed
	aNode := root.children[0]
	bNode := root.children[1]
	aNode.Expand()

	// Left at first level
	ns.ActionLeft()
	if aNode.Expanded() {
		t.Error("A should be collapsed")
	}
	if bNode.Expanded() {
		t.Error("B should remain collapsed (was not expanded)")
	}
}
