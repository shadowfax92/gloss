package mdfile

import "strings"

// Is reports whether p looks like a markdown file by extension.
func Is(p string) bool {
	low := strings.ToLower(p)
	return strings.HasSuffix(low, ".md") || strings.HasSuffix(low, ".markdown")
}
