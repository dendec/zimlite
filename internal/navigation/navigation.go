// Package navigation provides an abstract Navigator interface and a simple
// history-stack implementation suitable for Stage 1 MVP.
package navigation

import (
	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
)

// Navigator is the abstract navigation interface.
// Implementations manage history and back navigation.
type Navigator interface {
	Open(id string)
	UpdateCurrentState(state document.ViewState)
	Back() (bool, document.ViewState)
	Current() string
	CurrentState() document.ViewState
}

// HistoryItem stores the document ID and its view state.
type HistoryItem struct {
	ID    string
	State document.ViewState
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

func (n *SimpleNavigator) CurrentState() document.ViewState {
	if n.index < 0 || n.index >= len(n.history) {
		return document.ViewState{ScrollY: 0, SelectedLink: -1}
	}
	return n.history[n.index].State
}

func (n *SimpleNavigator) Open(id string) {
	// Truncate any forward history.
	if n.index+1 < len(n.history) {
		n.history = n.history[:n.index+1]
	}
	n.history = append(n.history, HistoryItem{ID: id, State: document.ViewState{ScrollY: 0, SelectedLink: -1}})
	n.index = len(n.history) - 1
}

func (n *SimpleNavigator) UpdateCurrentState(state document.ViewState) {
	if n.index >= 0 && n.index < len(n.history) {
		n.history[n.index].State = state
	}
}

func (n *SimpleNavigator) Back() (bool, document.ViewState) {
	if n.index <= 0 {
		return false, document.ViewState{ScrollY: 0, SelectedLink: -1}
	}
	n.index--
	return true, n.history[n.index].State
}
