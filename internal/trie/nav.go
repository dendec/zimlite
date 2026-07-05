package trie

// NavState tracks the user's position in the tree and provides navigation.
type NavState struct {
	Root   *RadixNode
	Cursor *RadixNode
	scroll int // line offset for rendering
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

// CursorLabel returns the display label of the current cursor.
func (ns *NavState) CursorLabel() string {
	if ns.Cursor == nil {
		return ""
	}
	return ns.Cursor.Label()
}

// CursorSuffix returns the article count suffix.
func (ns *NavState) CursorSuffix() string {
	if ns.Cursor == nil {
		return ""
	}
	return ns.Cursor.Suffix()
}

// CursorIsLeaf returns true if cursor is on an article.
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

// MoveDown moves cursor to next visible sibling.
func (ns *NavState) MoveDown() {
	if ns.Cursor == nil {
		return
	}
	parent := ns.Cursor.parent
	if parent == nil {
		return // at root
	}
	for i, c := range parent.children {
		if c == ns.Cursor && i+1 < len(parent.children) {
			ns.Cursor = parent.children[i+1]
			return
		}
	}
	// Wrap: stay at last.
}

// MoveUp moves cursor to previous visible sibling.
func (ns *NavState) MoveUp() {
	if ns.Cursor == nil {
		return
	}
	parent := ns.Cursor.parent
	if parent == nil {
		return
	}
	for i, c := range parent.children {
		if c == ns.Cursor && i > 0 {
			ns.Cursor = parent.children[i-1]
			return
		}
	}
}

// ExpandCurrent expands the cursor node, building children if needed.
func (ns *NavState) ExpandCurrent() {
	if ns.Cursor == nil {
		return
	}
	if ns.Cursor.IsLeaf() {
		return
	}
	ns.Cursor.Expand()
	// Move cursor into first child if available.
	if len(ns.Cursor.children) > 0 {
		ns.Cursor = ns.Cursor.children[0]
	}
}

// CollapseCurrent collapses cursor's parent and moves cursor up.
func (ns *NavState) CollapseCurrent() {
	if ns.Cursor == nil {
		return
	}
	parent := ns.Cursor.parent
	if parent == nil || parent == ns.Root {
		return
	}
	parent.Collapse()
	ns.Cursor = parent
}

// VisibleNodes flattens the currently visible tree into a displayable list.
// Returns a slice of (indent, label, suffix, isLeaf, isExpanded, isCursor) tuples.
type VisLine struct {
	Indent     int
	Label      string
	Suffix     string
	IsLeaf     bool
	IsExpanded bool
	IsCursor   bool
}

func (ns *NavState) VisibleNodes() []VisLine {
	var lines []VisLine
	ns.walk(ns.Root, 0, &lines)
	return lines
}

func (ns *NavState) walk(node *RadixNode, depth int, lines *[]VisLine) {
	if node == nil {
		return
	}
	// Skip root.
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
