package trie

import (
	"testing"
)

type mockZIM struct {
	titles []string
	paths  []string
}

func (m *mockZIM) ArticleCount() int                            { return len(m.titles) }
func (m *mockZIM) TitleByIndex(idx int) (string, string, error) { return m.titles[idx], m.paths[idx], nil }

func TestRootBuild(t *testing.T) {
	m := &mockZIM{
		titles: []string{"Apple", "Apricot", "Banana", "Cherry"},
		paths:  []string{"A/Apple", "A/Apricot", "A/Banana", "A/Cherry"},
	}

	root := Root(m, len(m.titles))
	if len(root.children) != 3 {
		t.Fatalf("expected 3 first-level groups (A, B, C), got %d", len(root.children))
	}

	// A has 2 articles → group node, not leaf.
	aNode := root.children[0]
	if aNode.Label() != "A" {
		t.Errorf("A label: got %q, want A", aNode.Label())
	}
	if aNode.Suffix() != "2" {
		t.Errorf("A suffix: got %q, want 2", aNode.Suffix())
	}
	if aNode.IsLeaf() {
		t.Error("A should NOT be a leaf (2 articles)")
	}

	// B has 1 article → leaf with full title.
	bNode := root.children[1]
	if bNode.Label() != "Banana" {
		t.Errorf("B label: got %q, want Banana", bNode.Label())
	}
	if !bNode.IsLeaf() {
		t.Error("B should be a leaf (single article)")
	}

	// C has 1 article → leaf.
	cNode := root.children[2]
	if cNode.Label() != "Cherry" {
		t.Errorf("C label: got %q, want Cherry", cNode.Label())
	}

	// Expand A — should build sub-groups.
	aNode.Expand()
	if !aNode.Expanded() {
		t.Fatal("A should be expanded")
	}
	// "Apple" and "Apricot" share "p" prefix → grouped under "p".
	if len(aNode.children) != 1 {
		t.Errorf("A should have 1 child (p group), got %d", len(aNode.children))
	}
	pGroup := aNode.children[0]
	if pGroup.Label() != "p" {
		t.Errorf("p group label: got %q, want p", pGroup.Label())
	}
	// Collapse should clear children.
	aNode.Collapse()
	if aNode.Expanded() {
		t.Error("A should be collapsed")
	}
	if len(aNode.children) != 0 {
		t.Error("A children should be cleared after collapse")
	}
}

func TestNavState(t *testing.T) {
	m := &mockZIM{
		titles: []string{"A", "B", "C"},
		paths:  []string{"A", "B", "C"},
	}

	root := Root(m, len(m.titles))
	ns := NewNavState(root)

	if ns.Cursor.Label() != "A" {
		t.Errorf("initial cursor: got %q, want A", ns.Cursor.Label())
	}

	ns.MoveDown()
	if ns.Cursor.Label() != "B" {
		t.Errorf("after move down: got %q, want B", ns.Cursor.Label())
	}

	ns.MoveDown()
	if ns.Cursor.Label() != "C" {
		t.Errorf("after move down 2: got %q, want C", ns.Cursor.Label())
	}

	// Wrap: stay at last.
	ns.MoveDown()
	if ns.Cursor.Label() != "C" {
		t.Errorf("move down at end: got %q, want C", ns.Cursor.Label())
	}

	ns.MoveUp()
	if ns.Cursor.Label() != "B" {
		t.Errorf("after move up: got %q, want B", ns.Cursor.Label())
	}

	// CollapseCurrent at root level should no-op.
	ns.CollapseCurrent()
	if ns.Cursor.Label() != "B" {
		t.Errorf("collapse at root: cursor should stay at B")
	}
}

func TestBuildLevelWithCyrillic(t *testing.T) {
	m := &mockZIM{
		titles: []string{"Абрикос", "Ананас", "Банан", "Вишня"},
		paths:  []string{"А/Абрикос", "А/Ананас", "А/Банан", "А/Вишня"},
	}

	root := Root(m, len(m.titles))
	if len(root.children) != 3 {
		t.Fatalf("expected 3 first-level groups, got %d", len(root.children))
	}
	// А has 2 articles → group.
	if root.children[0].Label() != "А" {
		t.Errorf("first child: got %q, want А", root.children[0].Label())
	}
	// Б has 1 article → leaf with full title.
	if root.children[1].Label() != "Банан" {
		t.Errorf("second child: got %q, want Банан", root.children[1].Label())
	}
	// В has 1 article → leaf.
	if root.children[2].Label() != "Вишня" {
		t.Errorf("third child: got %q, want Вишня", root.children[2].Label())
	}
}
