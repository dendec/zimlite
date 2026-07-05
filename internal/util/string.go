package util

// Truncate safely truncates a string to maxLen characters (runes), appending "..." if it was truncated.
func Truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return s
}
