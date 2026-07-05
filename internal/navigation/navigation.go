// Package navigation provides an abstract Navigator interface and a simple
// history-stack implementation suitable for Stage 1 MVP.
package navigation

// Navigator is the abstract navigation interface.
// Implementations manage history, back/forward, and document opening.
type Navigator interface {
	Open(id string)
	UpdateCurrentState(scrollY int32, linkIdx int)
	Back() (bool, int32, int)    // returns ok, scrollY, linkIdx
	Forward() (bool, int32, int) // returns ok, scrollY, linkIdx
	Current() string             // returns current document ID
	CurrentState() (int32, int)  // returns scrollY, linkIdx
}

// HistoryItem stores the document ID, its scroll position, and selected link.
type HistoryItem struct {
	ID           string
	ScrollY      int32
	SelectedLink int
}

// SimpleNavigator is a basic stack-based navigator for MVP.
type SimpleNavigator struct {
	history []HistoryItem
	index   int // position in history; -1 means empty
}

// NewSimpleNavigator creates an empty navigator.
func NewSimpleNavigator() *SimpleNavigator {
	return &SimpleNavigator{
		history: nil,
		index:   -1,
	}
}

func (n *SimpleNavigator) Current() string {
	if n.index < 0 || n.index >= len(n.history) {
		return ""
	}
	return n.history[n.index].ID
}

func (n *SimpleNavigator) CurrentState() (int32, int) {
	if n.index < 0 || n.index >= len(n.history) {
		return 0, -1
	}
	return n.history[n.index].ScrollY, n.history[n.index].SelectedLink
}

func (n *SimpleNavigator) Open(id string) {
	// Truncate any forward history.
	if n.index+1 < len(n.history) {
		n.history = n.history[:n.index+1]
	}
	n.history = append(n.history, HistoryItem{ID: id, ScrollY: 0, SelectedLink: -1})
	n.index = len(n.history) - 1
}

func (n *SimpleNavigator) UpdateCurrentState(scrollY int32, linkIdx int) {
	if n.index >= 0 && n.index < len(n.history) {
		n.history[n.index].ScrollY = scrollY
		n.history[n.index].SelectedLink = linkIdx
	}
}

func (n *SimpleNavigator) Back() (bool, int32, int) {
	if n.index <= 0 {
		return false, 0, -1
	}
	n.index--
	return true, n.history[n.index].ScrollY, n.history[n.index].SelectedLink
}

func (n *SimpleNavigator) Forward() (bool, int32, int) {
	if n.index+1 >= len(n.history) {
		return false, 0, -1
	}
	n.index++
	return true, n.history[n.index].ScrollY, n.history[n.index].SelectedLink
}
