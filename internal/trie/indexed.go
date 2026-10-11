package trie

import (
	"fmt"

	"github.com/dendec/zimlite/internal/document"
)

// TitleIndex exposes random access to the archive's existing title index.
// The tree never owns a complete article list or reads article bodies.
type TitleIndex interface {
	ArticleCount() int
	ArticleAt(int) (document.ArticleEntry, error)
}

const indexedFanout = 32
const indexedLeafSize = 128

type indexedRange struct {
	source     TitleIndex
	start, end int // Half-open interval in the title index.
}

// NewIndexedTree creates bounded title ranges; all articles remain reachable.
// Only one branch per level stays expanded, bounding retained tree metadata.
func NewIndexedTree(source TitleIndex) (*RadixNode, error) {
	root := &RadixNode{index: &indexedRange{source: source, end: source.ArticleCount()}}
	root.expandIndexed()
	return root, root.loadErr
}

func (n *RadixNode) LoadError() error { return n.loadErr }

func (n *RadixNode) expandIndexed() {
	n.loadErr = nil
	r := n.index
	count := r.end - r.start
	var children []*RadixNode
	if count <= indexedLeafSize {
		for i := r.start; i < r.end; i++ {
			entry, err := r.source.ArticleAt(i)
			if err != nil {
				n.loadErr = err
				return
			}
			children = append(children, &RadixNode{parent: n, prefix: entry.Title,
				leaf: &LeafInfo{Title: entry.Title, Path: entry.Path}})
		}
	} else {
		step := (count + indexedFanout - 1) / indexedFanout
		for start := r.start; start < r.end; start += step {
			end := min(start+step, r.end)
			first, err := r.source.ArticleAt(start)
			if err != nil {
				n.loadErr = err
				return
			}
			last, err := r.source.ArticleAt(end - 1)
			if err != nil {
				n.loadErr = err
				return
			}
			children = append(children, &RadixNode{parent: n,
				prefix: fmt.Sprintf("%s … %s", first.Title, last.Title),
				index:  &indexedRange{source: r.source, start: start, end: end}})
		}
	}
	// Collapse siblings only after a successful read; errors can be retried.
	if n.parent != nil {
		for _, sibling := range n.parent.children {
			if sibling != n && sibling.index != nil && sibling.expanded {
				sibling.Collapse()
			}
		}
	}
	n.children = children
	n.expanded = true
	n.built = true
}
