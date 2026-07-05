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

	// Back from first doc should fail.
	if ok, _ := n.Back(); ok {
		t.Error("back from first doc should fail")
	}

	// Open a new doc overwrites history.
	n.Open("doc3")
	if n.Current() != "doc3" {
		t.Errorf("current: got %q, want doc3", n.Current())
	}
}
