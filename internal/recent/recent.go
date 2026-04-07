package recent

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gloss/internal/paths"
)

type Entry struct {
	Path string    `json:"path"`
	Time time.Time `json:"time"`
}

type Reader struct {
	tsvPath string
	ttl     time.Duration

	mu      sync.Mutex
	cacheAt time.Time
	cached  []Entry
}

func New() *Reader {
	return &Reader{
		tsvPath: paths.RecentFilesTSV(),
		ttl:     5 * time.Second,
	}
}

func (r *Reader) ListMarkdown(days, max int) ([]Entry, error) {
	all, err := r.allCached()
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	out := make([]Entry, 0, len(all))
	for _, e := range all {
		if !isMarkdown(e.Path) {
			continue
		}
		if e.Time.Before(cutoff) {
			continue
		}
		out = append(out, e)
	}
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return out, nil
}

func (r *Reader) allCached() ([]Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if time.Since(r.cacheAt) < r.ttl && r.cached != nil {
		return r.cached, nil
	}

	data, err := os.ReadFile(r.tsvPath)
	if err != nil {
		if os.IsNotExist(err) {
			r.cached = nil
			r.cacheAt = time.Now()
			return nil, nil
		}
		return nil, err
	}

	seen := map[string]int64{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		ts, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}
		p := parts[1]
		if cur, ok := seen[p]; !ok || ts > cur {
			seen[p] = ts
		}
	}

	out := make([]Entry, 0, len(seen))
	for p, ts := range seen {
		out = append(out, Entry{Path: p, Time: time.Unix(ts, 0)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })

	r.cached = out
	r.cacheAt = time.Now()
	return out, nil
}

func isMarkdown(p string) bool {
	low := strings.ToLower(p)
	return strings.HasSuffix(low, ".md") || strings.HasSuffix(low, ".markdown")
}
