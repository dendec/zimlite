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

func TestAutoExpand(t *testing.T) {
	// 3 articles sharing "XYZ " prefix — auto-expand drills through to leaves.
	// Structure: X → XY → XYZ → "XYZ " → [A:leaf, B:leaf, G:leaf]
	articles := []zim.ArticleEntry{
		{Title: "XYZ Alpha", Path: "A/Alpha"},
		{Title: "XYZ Beta", Path: "A/Beta"},
		{Title: "XYZ Gamma", Path: "A/Gamma"},
	}

	root := NewTree(articles)
	x := root.children[0]
	if !x.Expanded() { t.Fatal("X should be auto-expanded") }
	if len(x.children) != 1 { t.Fatalf("X→1 child, got %d", len(x.children)) }

	xy := x.children[0]
	if !xy.Expanded() { t.Fatal("XY auto-expanded") }
	if len(xy.children) != 1 { t.Fatalf("XY→1 child, got %d", len(xy.children)) }

	xyz := xy.children[0]
	if !xyz.Expanded() { t.Fatal("XYZ auto-expanded") }
	// Next char after XYZ is space for all three → grouped under "XYZ "
	if len(xyz.children) != 1 { t.Fatalf("XYZ→1 child (space group), got %d", len(xyz.children)) }

	spaceGroup := xyz.children[0]
	if !spaceGroup.Expanded() { t.Fatal("space group auto-expanded") }
	if len(spaceGroup.children) != 3 { t.Fatalf("space→3 leaves, got %d", len(spaceGroup.children)) }
	for _, c := range spaceGroup.children {
		if !c.IsLeaf() { t.Errorf("expected leaf, got %q", c.Label()) }
	}
}

func TestNoAutoExpand(t *testing.T) {
	// Large group (>20) should NOT auto-expand.
	articles := make([]zim.ArticleEntry, 25)
	for i := 0; i < 25; i++ {
		articles[i] = zim.ArticleEntry{
			Title: "A" + string(rune('A'+i)) + "title",
			Path:  "A/title",
		}
	}

	root := NewTree(articles)
	aNode := root.children[0]
	if aNode.Expanded() {
		t.Error("A node with 25 articles should NOT auto-expand")
	}
}
