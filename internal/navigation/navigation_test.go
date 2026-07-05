package navigation

import "testing"

func TestSimpleNavigator(t *testing.T) {
	n := NewSimpleNavigator()

	if n.Current() != "" {
		t.Error("empty navigator should return empty current")
	}

	if ok, _ := n.Back(); ok {
		t.Error("back on empty navigator should fail")
	}

	if ok, _ := n.Forward(); ok {
		t.Error("forward on empty navigator should fail")
	}

	n.Open("doc1")
	if n.Current() != "doc1" {
		t.Errorf("current: got %q, want doc1", n.Current())
	}

	n.Open("doc2")
	if n.Current() != "doc2" {
		t.Errorf("current: got %q, want doc2", n.Current())
	}

	if ok, _ := n.Back(); !ok {
		t.Error("back should succeed")
	}
	if n.Current() != "doc1" {
		t.Errorf("after back: got %q, want doc1", n.Current())
	}

	if ok, _ := n.Forward(); !ok {
		t.Error("forward should succeed")
	}
	if n.Current() != "doc2" {
		t.Errorf("after forward: got %q, want doc2", n.Current())
	}

	// Back twice should fail.
	if ok, _ := n.Back(); !ok {
		t.Error("first back should succeed")
	}
	if ok, _ := n.Back(); ok {
		t.Error("second back should fail")
	}

	// Open a new doc should truncate forward history.
	n.Open("doc3")
	if ok, _ := n.Forward(); ok {
		t.Error("forward after new open should fail (history truncated)")
	}
	if n.Current() != "doc3" {
		t.Errorf("current: got %q, want doc3", n.Current())
	}
}
