//go:build !cgo

package article

import "errors"

// OpenZIMSource requires libzim; HTML probes can run without cgo.
func OpenZIMSource(string) (Source, string, error) {
	return nil, "", errors.New("ZIM source requires cgo and libzim")
}
