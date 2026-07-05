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

// CursorExpandable returns true if the cursor node is a branch that can be
// expanded (as opposed to a terminal article that should be opened).
func (ns *NavState) CursorExpandable() bool {
	return ns.Cursor != nil && ns.Cursor.Expandable()
}

// CursorPath returns the ZIM path of the leaf article, or "".
func (ns *NavState) CursorPath() string {
	if ns.Cursor == nil {
		return ""
	}
	return ns.Cursor.FullPath()
}

// MoveDown moves cursor to the next sibling (same parent).
func (ns *NavState) MoveDown() {
	if ns.Cursor == nil || ns.Cursor.parent == nil {
		return
	}
	siblings := ns.Cursor.parent.children
	for i, s := range siblings {
		if s == ns.Cursor && i+1 < len(siblings) {
			ns.Cursor = siblings[i+1]
			return
		}
	}
}

// MoveTo moves the cursor to the visible node at flat index idx.
func (ns *NavState) MoveTo(idx int) {
	lines := ns.VisibleNodes()
	if idx >= 0 && idx < len(lines) {
		var found int
		var findNode func(node *RadixNode) *RadixNode
		findNode = func(node *RadixNode) *RadixNode {
			if node.parent != nil {
				if found == idx {
					return node
				}
				found++
			}
			if node.Expanded() {
				for _, c := range node.children {
					if r := findNode(c); r != nil {
						return r
					}
				}
			}
			return nil
		}
		if n := findNode(ns.Root); n != nil {
			ns.Cursor = n
		}
	}
}

// MoveUp moves cursor to the previous sibling (same parent).
func (ns *NavState) MoveUp() {
	if ns.Cursor == nil || ns.Cursor.parent == nil {
		return
	}
	siblings := ns.Cursor.parent.children
	for i, s := range siblings {
		if s == ns.Cursor && i > 0 {
			ns.Cursor = siblings[i-1]
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
	if !ns.Cursor.Expandable() {
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
// - At first level (parent == Root): collapses all branches, cursor stays.
// - Otherwise: moves cursor to parent, then collapses the parent's branch.
func (ns *NavState) ActionLeft() {
	if ns.Cursor == nil {
		return
	}
	if ns.Cursor.parent == ns.Root {
		ns.collapseRoot()
		return
	}
	ns.Cursor = ns.Cursor.parent
	if ns.Cursor.Expanded() {
		ns.Cursor.Collapse()
	}
	// When the cursor lands back on the root level, ensure every branch is
	// collapsed so the tree returns to its fully-folded state.
	if ns.Cursor.parent == ns.Root {
		ns.collapseRoot()
	}
}

// collapseRoot collapses all first-level branches.
func (ns *NavState) collapseRoot() {
	for _, child := range ns.Root.children {
		if child.Expanded() {
			child.Collapse()
		}
	}
}

// VisLine describes one line in the tree display.
type VisLine struct {
	TreePrefix string
	Label      string
	Suffix     string
	Path       string
	IsLeaf     bool
	IsExpanded bool
	IsCursor   bool
}

// VisibleNodes flattens the currently visible tree for display.
func (ns *NavState) VisibleNodes() []VisLine {
	var lines []VisLine
	ns.walk(ns.Root, "", &lines, false)
	return lines
}

func (ns *NavState) walk(node *RadixNode, prefix string, lines *[]VisLine, isLast bool) {
	if node == nil {
		return
	}
	if node.parent != nil {
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		*lines = append(*lines, VisLine{
			TreePrefix: prefix + connector,
			Label:      node.Label(),
			Suffix:     node.Suffix(),
			Path:       node.FullPath(),
			IsLeaf:     !node.Expandable(),
			IsExpanded: node.Expanded() && len(node.children) > 0,
			IsCursor:   node == ns.Cursor,
		})
	}
	if node.Expanded() {
		for i, c := range node.children {
			childIsLast := i == len(node.children)-1
			var childPrefix string
			if node.parent != nil {
				if isLast {
					childPrefix = prefix + "    "
				} else {
					childPrefix = prefix + "│   "
				}
			}
			ns.walk(c, childPrefix, lines, childIsLast)
		}
	}
}
