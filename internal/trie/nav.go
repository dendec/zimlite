package trie

// NavState tracks the user's position in the tree and provides navigation.
type NavState struct {
	Root   *RadixNode
	Cursor *RadixNode
}

// NewNavState creates navigation state rooted at the given node.
func NewNavState(root *RadixNode) *NavState {
	ns := &NavState{Root: root}
	if len(root.children) > 0 {
		ns.Cursor = root.children[0]
	} else {
		ns.Cursor = root
	}
	return ns
}

// CursorIsLeaf returns true if cursor is on a leaf article.
func (ns *NavState) CursorIsLeaf() bool {
	return ns.Cursor != nil && ns.Cursor.IsLeaf()
}

// CursorPath returns the ZIM path of the leaf article, or "".
func (ns *NavState) CursorPath() string {
	if ns.Cursor == nil {
		return ""
	}
	return ns.Cursor.FullPath()
}

// flatVisible collects all visible nodes in the tree in a flat list.
func (ns *NavState) flatVisible() []*RadixNode {
	var nodes []*RadixNode
	ns.collectVisible(ns.Root, &nodes)
	return nodes
}

func (ns *NavState) collectVisible(node *RadixNode, out *[]*RadixNode) {
	if node.parent != nil {
		*out = append(*out, node)
	}
	if node.Expanded() {
		for _, c := range node.children {
			ns.collectVisible(c, out)
		}
	}
}

// MoveDown moves cursor to the next flat visible node.
func (ns *NavState) MoveDown() {
	nodes := ns.flatVisible()
	for i, n := range nodes {
		if n == ns.Cursor && i+1 < len(nodes) {
			ns.Cursor = nodes[i+1]
			return
		}
	}
}

// MoveTo moves the cursor to the visible node at index idx.
func (ns *NavState) MoveTo(idx int) {
	nodes := ns.flatVisible()
	if idx >= 0 && idx < len(nodes) {
		ns.Cursor = nodes[idx]
	}
}

// MoveUp moves cursor to the previous flat visible node.
func (ns *NavState) MoveUp() {
	nodes := ns.flatVisible()
	for i, n := range nodes {
		if n == ns.Cursor && i > 0 {
			ns.Cursor = nodes[i-1]
			return
		}
	}
}

// ActionRight implements the Right-key action:
// - On a collapsed node: expands it and selects the first child (if any).
// - On an expanded node: moves down to its first child.
// - On a leaf: does nothing.
func (ns *NavState) ActionRight() {
	if ns.Cursor == nil {
		return
	}
	if ns.Cursor.IsLeaf() {
		return
	}
	if !ns.Cursor.Expanded() {
		ns.Cursor.Expand()
	}
	if len(ns.Cursor.children) > 0 {
		ns.Cursor = ns.Cursor.children[0]
	}
}

// ActionLeft implements the Left-key action:
// - On an expanded node: collapses it.
// - On a collapsed node or leaf: moves cursor to the parent node.
func (ns *NavState) ActionLeft() {
	if ns.Cursor == nil {
		return
	}
	if ns.Cursor.Expanded() && len(ns.Cursor.children) > 0 {
		ns.Cursor.Collapse()
		return
	}
	if ns.Cursor.parent != nil && ns.Cursor.parent != ns.Root {
		ns.Cursor = ns.Cursor.parent
	}
}

// VisLine describes one line in the tree display.
type VisLine struct {
	Indent     int
	Label      string
	Suffix     string
	IsLeaf     bool
	IsExpanded bool
	IsCursor   bool
}

// VisibleNodes flattens the currently visible tree for display.
func (ns *NavState) VisibleNodes() []VisLine {
	var lines []VisLine
	ns.walk(ns.Root, 0, &lines)
	return lines
}

func (ns *NavState) walk(node *RadixNode, depth int, lines *[]VisLine) {
	if node == nil {
		return
	}
	if node.parent != nil {
		*lines = append(*lines, VisLine{
			Indent:     depth - 1,
			Label:      node.Label(),
			Suffix:     node.Suffix(),
			IsLeaf:     node.IsLeaf(),
			IsExpanded: node.Expanded() && len(node.children) > 0,
			IsCursor:   node == ns.Cursor,
		})
	}
	if node.Expanded() {
		for _, c := range node.children {
			ns.walk(c, depth+1, lines)
		}
	}
}
