package trie

import (
	"fmt"
	"testing"

	"github.com/dendec/zimlite/internal/document"
)

type generatedTitleIndex struct {
	count, reads int
	fail         bool
}

func (i *generatedTitleIndex) ArticleCount() int { return i.count }
func (i *generatedTitleIndex) ArticleAt(n int) (document.ArticleEntry, error) {
	i.reads++
	if i.fail || n < 0 || n >= i.count {
		return document.ArticleEntry{}, fmt.Errorf("cannot read entry %d", n)
	}
	return document.ArticleEntry{Title: fmt.Sprintf("Article %08d", n), Path: fmt.Sprint(n)}, nil
}

func TestIndexedTreeReachesMillionsOfArticlesWithBoundedReads(t *testing.T) {
	index := &generatedTitleIndex{count: 10_000_000}
	root, err := NewIndexedTree(index)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []int{0, index.count / 2, index.count - 1} {
		node := root
		for node.index != nil {
			node.Expand()
			if err := node.LoadError(); err != nil {
				t.Fatal(err)
			}
			var next *RadixNode
			for _, child := range node.Children() {
				if child.index != nil && target >= child.index.start && target < child.index.end ||
					child.IsLeaf() && child.FullPath() == fmt.Sprint(target) {
					next = child
					break
				}
			}
			if next == nil {
				t.Fatalf("article %d is unreachable", target)
			}
			node = next
		}
		if node.FullPath() != fmt.Sprint(target) {
			t.Fatalf("opened %s instead of article %d", node.FullPath(), target)
		}
	}
	if index.reads > 1500 {
		t.Fatalf("too many title reads for three branches: %d", index.reads)
	}
}

func TestIndexedTreeFailedReadCanBeRetried(t *testing.T) {
	index := &generatedTitleIndex{count: 1000}
	root, err := NewIndexedTree(index)
	if err != nil {
		t.Fatal(err)
	}
	branch := root.Children()[0]
	index.fail = true
	branch.Expand()
	if branch.LoadError() == nil || branch.Expanded() {
		t.Fatal("failed read must leave the branch collapsed with an error")
	}
	index.fail = false
	branch.Expand()
	if branch.LoadError() != nil || !branch.Expanded() || len(branch.Children()) == 0 {
		t.Fatal("successful retry must populate the branch")
	}
}
