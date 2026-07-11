package ui

import (
	"testing"

	"github.com/dendec/zimlite/internal/zim"
)

// TestInterfaceCompliance verifies that concrete types satisfy the injected interfaces.
func TestInterfaceCompliance(t *testing.T) {
	var z *zim.Reader
	_ = ZimReader(z) // *zim.Reader satisfies ZimReader
}
