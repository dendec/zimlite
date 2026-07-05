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
