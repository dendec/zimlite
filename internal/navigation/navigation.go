// Package navigation provides an abstract Navigator interface and a simple
// history-stack implementation suitable for Stage 1 MVP.
package navigation

// Navigator is the abstract navigation interface.
// Implementations manage history, back/forward, and document opening.
type Navigator interface {
	Open(id string)
	Back() bool  // returns false when no history
	Forward() bool // returns false when no forward history
	Current() string // returns current document ID
}

// SimpleNavigator is a basic stack-based navigator for MVP.
type SimpleNavigator struct {
	history []string
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
	return n.history[n.index]
}

func (n *SimpleNavigator) Open(id string) {
	// Truncate any forward history.
	if n.index+1 < len(n.history) {
		n.history = n.history[:n.index+1]
	}
	n.history = append(n.history, id)
	n.index = len(n.history) - 1
}

func (n *SimpleNavigator) Back() bool {
	if n.index <= 0 {
		return false
	}
	n.index--
	return true
}

func (n *SimpleNavigator) Forward() bool {
	if n.index+1 >= len(n.history) {
		return false
	}
	n.index++
	return true
}
