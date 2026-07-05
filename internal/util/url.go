package util

import "strings"

// IsExternalURL reports whether rawURL is an HTTP(S) or protocol-relative URL.
func IsExternalURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "http://") ||
		strings.HasPrefix(rawURL, "https://") ||
		strings.HasPrefix(rawURL, "//")
}
