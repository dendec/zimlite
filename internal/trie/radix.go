// Package trie implements a lazy Radix Tree for ZIM article navigation.
// First level (A-Z, А-Я, 0-9) built eagerly; deeper levels expanded on demand.
package trie

import (
	"sort"
	"unicode/utf8"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
)

// LeafInfo holds metadata for a terminal (article) node.
type LeafInfo struct {
	Title string
	Path  string
}

// RadixNode is a node in the compressed prefix tree.
type RadixNode struct {
	prefix     string
	origPrefix string // saved before auto-drill mutates prefix
	leaf       *LeafInfo
	children   []*RadixNode
	parent     *RadixNode

	expanded bool
	articles []document.ArticleEntry // lazy: only populated on Expand for non-root
	built    bool
}

// NewTree builds a tree from a pre-fetched article list.
// Builds first level eagerly (grouping by first rune).
func NewTree(articles []document.ArticleEntry) *RadixNode {
	root := &RadixNode{prefix: "", expanded: true}
	if len(articles) == 0 {
		return root
	}

	// Group by first rune.
	type group struct {
		r    rune
		arts []document.ArticleEntry
	}
	var groups []group
	for _, a := range articles {
		r, _ := utf8.DecodeRuneInString(a.Title)
		if r == utf8.RuneError {
			continue
		}
		label := labelForRune(r)
		if len(groups) == 0 || groups[len(groups)-1].r != label {
			groups = append(groups, group{r: label})
		}
		groups[len(groups)-1].arts = append(groups[len(groups)-1].arts, a)
	}

	for _, g := range groups {
		node := &RadixNode{
			prefix:   string(g.r),
			parent:   root,
			articles: g.arts,
		}
		if len(g.arts) == 1 {
			a := g.arts[0]
			node.leaf = &LeafInfo{Title: a.Title, Path: a.Path}
		}
		root.children = append(root.children, node)
	}

	return root
}

// Expand builds children for this node from its article slice.
func (n *RadixNode) Expand() {
	if n.leaf != nil || n.expanded {
		return
	}
	n.expanded = true
	if n.built || len(n.articles) <= 1 {
		return
	}
	n.built = true

	// Group articles by second-level prefix (after first char removed).
	prefixLen := len([]rune(n.prefix))
	type group struct {
		key  string
		arts []document.ArticleEntry
	}
	groups := make(map[string]*group)
	var order []string

	for _, a := range n.articles {
		runes := []rune(a.Title)
		var key string
		if len(runes) > prefixLen {
			key = string(runes[prefixLen])
		} else {
			key = ""
		}
		if key == "" {
			if n.leaf == nil {
				n.leaf = &LeafInfo{Title: a.Title, Path: a.Path}
			}
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &group{key: key}
			groups[key] = g
			order = append(order, key)
		}
		g.arts = append(g.arts, a)
	}

	for _, key := range order {
		g := groups[key]
		child := &RadixNode{
			prefix:   n.prefix + key,
			parent:   n,
			articles: g.arts,
		}
		if len(g.arts) == 1 {
			a := g.arts[0]
			child.leaf = &LeafInfo{Title: a.Title, Path: a.Path}
		}
		n.children = append(n.children, child)
	}

	sort.Slice(n.children, func(i, j int) bool {
		return n.children[i].prefix < n.children[j].prefix
	})

	// Save original prefix before auto-drill mutates it.
	n.origPrefix = n.prefix

	// Auto-drill and absorb single-child chains.
	for len(n.children) == 1 && !n.children[0].IsLeaf() {
		only := n.children[0]
		only.Expand()
		n.prefix = only.prefix
		n.articles = only.articles
		n.children = only.children
		n.leaf = only.leaf
		for _, c := range n.children {
			c.parent = n
		}
	}

}

// Collapse clears children to free memory.
func (n *RadixNode) Collapse() {
	if n.leaf != nil {
		return
	}
	// Restore original prefix before auto-drill so re-expand
	// produces the same tree structure.
	if n.origPrefix != "" {
		n.prefix = n.origPrefix
		n.origPrefix = ""
	}
	n.children = nil
	n.expanded = false
	n.built = false
}

// IsLeaf returns true if this node is a direct article.
func (n *RadixNode) IsLeaf() bool { return n.leaf != nil }

// Label returns the display label.
func (n *RadixNode) Label() string {
	if n.parent == nil {
		return "/"
	}
	if n.leaf != nil {
		return n.leaf.Title
	}
	return n.prefix
}

// Suffix returns the article count or empty.
func (n *RadixNode) Suffix() string {
	if n.leaf != nil {
		return ""
	}
	count := len(n.articles)
	if count > 0 {
		return itoa(count)
	}
	return ""
}

// FullPath returns the ZIM path for leaf nodes.
func (n *RadixNode) FullPath() string {
	if n.leaf != nil {
		return n.leaf.Path
	}
	return ""
}

// Parent returns the parent node.
func (n *RadixNode) Parent() *RadixNode { return n.parent }

// Children returns child nodes.
func (n *RadixNode) Children() []*RadixNode { return n.children }

// Expanded returns whether children are loaded.
func (n *RadixNode) Expanded() bool { return n.expanded }

func labelForRune(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r
	}
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	if r >= 'А' && r <= 'Я' {
		return r
	}
	if r >= 'а' && r <= 'я' {
		return r - 'а' + 'А'
	}
	return r
}

func itoa(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, 0, 16)
	nn := n
	for nn > 0 {
		buf = append(buf, byte('0'+nn%10))
		nn /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
