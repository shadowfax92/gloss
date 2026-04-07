package highlights

import (
	"os"
	"strings"
	"time"
)

type Highlight struct {
	ID        string    `json:"id"`
	AbsPath   string    `json:"abs_path"`
	LineStart int       `json:"line_start"`
	LineEnd   int       `json:"line_end"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	Note      string    `json:"note,omitempty"`
}

// IsStale re-reads the source file and returns true when the stored line range
// no longer contains the stored text. Treats unreadable files as stale.
func (h *Highlight) IsStale() bool {
	data, err := os.ReadFile(h.AbsPath)
	if err != nil {
		return true
	}
	lines := strings.Split(string(data), "\n")
	if h.LineStart < 1 || h.LineEnd > len(lines) || h.LineStart > h.LineEnd {
		return true
	}
	chunk := strings.Join(lines[h.LineStart-1:h.LineEnd], "\n")
	return strings.TrimSpace(chunk) != strings.TrimSpace(h.Text)
}
