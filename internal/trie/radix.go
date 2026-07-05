// Package trie implements a lazy Radix Tree for ZIM article navigation.
package trie

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// ZIMReader abstracts the ZIM index needed for tree building.
type ZIMReader interface {
	ArticleCount() int
	TitleByIndex(idx int) (title string, path string, err error)
}

// LeafInfo holds metadata for a terminal (article) node.
type LeafInfo struct {
	Title string
	Path  string
}

// RadixNode is a node in the compressed prefix tree.
type RadixNode struct {
	prefix   string
	leaf     *LeafInfo
	children []*RadixNode
	parent   *RadixNode

	expanded bool
	zr       ZIMReader
	zimStart int
	zimEnd   int
	zimBuilt bool
}

// Root creates a new tree root. Builds first level eagerly.
func Root(zr ZIMReader, count int) *RadixNode {
	root := &RadixNode{
		prefix:   "",
		zr:       zr,
		zimStart: 0,
		zimEnd:   count,
		expanded: true,
	}
	root.buildLevel()
	return root
}

// buildLevel reads ZIM titles in [start, end) and builds children by first rune.
// Runs in O(n) where n = end - start.
func (n *RadixNode) buildLevel() {
	if n.zimBuilt || n.zr == nil || n.zimStart >= n.zimEnd {
		return
	}
	n.zimBuilt = true

	// Group titles by first rune, storing ZIM index ranges.
	type bucket struct {
		r     rune
		start int
		end   int // exclusive
	}
	var buckets []bucket

	prevRune := rune(-1)
	for i := n.zimStart; i < n.zimEnd; i++ {
		title, _, err := n.zr.TitleByIndex(i)
		if err != nil {
			continue
		}
		r, _ := utf8.DecodeRuneInString(title)
		if r == utf8.RuneError {
			continue
		}
		if r != prevRune {
			if prevRune != -1 {
				buckets[len(buckets)-1].end = i
			}
			buckets = append(buckets, bucket{r: r, start: i})
			prevRune = r
		}
	}
	if len(buckets) > 0 {
		buckets[len(buckets)-1].end = n.zimEnd
	}

	for _, b := range buckets {
		child := &RadixNode{
			prefix:   prefixForRune(b.r),
			parent:   n,
			zr:       n.zr,
			zimStart: b.start,
			zimEnd:   b.end,
		}
		// If bucket has only 1 article, make it a leaf.
		if b.end-b.start == 1 {
			title, path, err := n.zr.TitleByIndex(b.start)
			if err == nil {
				child.leaf = &LeafInfo{Title: title, Path: path}
			}
		}
		n.children = append(n.children, child)
	}
}

// buildSubtree lazily builds the children of this node by reading ZIM titles.
func (n *RadixNode) buildSubtree() {
	if n.zimBuilt || n.leaf != nil || n.zr == nil || n.zimStart >= n.zimEnd {
		return
	}

	// Build a sub-trie limited to this node's index range.
	// Use a map for shared-prefix grouping.
	type slot struct {
		key      string
		children []*RadixNode
		leaf     *LeafInfo // if exact match exists
	}
	groups := make(map[string]*slot)
	var order []string

	for i := n.zimStart; i < n.zimEnd; i++ {
		title, path, err := n.zr.TitleByIndex(i)
		if err != nil {
			continue
		}
		// Title is relative to n.prefix: "Abc" with prefix "A" → key "bc"
		key := title
		if len(n.prefix) > 0 && strings.HasPrefix(title, n.prefix) {
			key = title[len(n.prefix):]
		}
		if key == "" {
			n.leaf = &LeafInfo{Title: title, Path: path}
			continue
		}

		first, _ := utf8.DecodeRuneInString(key)
		firstStr := string(first)

		grp, ok := groups[firstStr]
		if !ok {
			grp = &slot{key: firstStr}
			groups[firstStr] = grp
			order = append(order, firstStr)
		}

		// For simplicity: each article becomes a leaf child with full key.
		child := &RadixNode{
			prefix:   key,
			leaf:     &LeafInfo{Title: title, Path: path},
			parent:   n,
			zr:       n.zr,
			zimBuilt: true,
		}
		grp.children = append(grp.children, child)
	}

	for _, key := range order {
		grp := groups[key]
		if len(grp.children) == 1 && grp.children[0].prefix == grp.key {
			// Single child: just add it directly.
			n.children = append(n.children, grp.children[0])
		} else {
			// Multiple children sharing first char: create grouping node.
			groupNode := &RadixNode{
				prefix:   grp.key,
				parent:   n,
				zr:       n.zr,
				zimBuilt: true,
			}
			groupNode.children = grp.children
			for _, c := range grp.children {
				c.parent = groupNode
			}
			n.children = append(n.children, groupNode)
		}
	}

	// Sort children for deterministic display.
	sort.Slice(n.children, func(i, j int) bool {
		return n.children[i].prefix < n.children[j].prefix
	})

	n.zimBuilt = true
}

// Expand builds children if not yet done.
func (n *RadixNode) Expand() {
	if n.leaf != nil {
		return // leaf, nothing to expand
	}
	if n.expanded {
		return
	}
	n.buildSubtree()
	n.expanded = true
}

// Collapse clears children to free memory (keeps ZIM range for re-expand).
func (n *RadixNode) Collapse() {
	if n.leaf != nil {
		return
	}
	if !n.expanded {
		return
	}
	n.children = nil
	n.zimBuilt = false
	n.expanded = false
}

// IsLeaf returns true if this node represents a single article.
func (n *RadixNode) IsLeaf() bool { return n.leaf != nil }

// ChildCount returns number of children (0 for leaf or unexpanded collapsed node).
func (n *RadixNode) ChildCount() int { return len(n.children) }

// Children returns the child nodes.
func (n *RadixNode) Children() []*RadixNode { return n.children }

// Label returns the display label for this node.
func (n *RadixNode) Label() string {
	if n.parent == nil {
		return "/"
	}
	if n.leaf != nil {
		return n.leaf.Title
	}
	return n.prefix
}

// Suffix returns the display suffix (article count or empty).
func (n *RadixNode) Suffix() string {
	if n.leaf != nil {
		return ""
	}
	count := n.zimEnd - n.zimStart
	if count > 0 && n.zr != nil {
		return itoa(count)
	}
	return ""
}

// FullPath returns the path of the article (or empty if not a leaf).
func (n *RadixNode) FullPath() string {
	if n.leaf != nil {
		return n.leaf.Path
	}
	return ""
}

// Parent returns the parent node.
func (n *RadixNode) Parent() *RadixNode { return n.parent }

// Expanded returns whether children are currently loaded.
func (n *RadixNode) Expanded() bool { return n.expanded }

func prefixForRune(r rune) string {
	if r >= 'A' && r <= 'Z' || r >= 'А' && r <= 'Я' {
		return strings.ToUpper(string(r))
	}
	if r >= 'a' && r <= 'z' || r >= 'а' && r <= 'я' {
		return strings.ToUpper(string(r))
	}
	return string(r)
}

func itoa(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, 0, 16)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
