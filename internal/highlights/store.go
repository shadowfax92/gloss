package highlights

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/oklog/ulid/v2"
)

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating highlights dir: %w", err)
	}
	return &Store{path: path}, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Add(h Highlight) (Highlight, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if h.ID == "" {
		h.ID = newULID()
	}
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}

	line, err := json.Marshal(h)
	if err != nil {
		return Highlight{}, err
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return Highlight{}, err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return Highlight{}, err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	if _, err := fmt.Fprintln(f, string(line)); err != nil {
		return Highlight{}, err
	}
	return h, nil
}

func (s *Store) List() ([]Highlight, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readAll()
}

func (s *Store) Get(id string) (*Highlight, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == id {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("highlight not found: %s", id)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.readAll()
	if err != nil {
		return err
	}
	kept := all[:0]
	found := false
	for _, h := range all {
		if h.ID == id {
			found = true
			continue
		}
		kept = append(kept, h)
	}
	if !found {
		return fmt.Errorf("highlight not found: %s", id)
	}
	return s.rewrite(kept)
}

func (s *Store) readAll() ([]Highlight, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return nil, err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	var out []Highlight
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var h Highlight
		if err := json.Unmarshal([]byte(line), &h); err != nil {
			continue
		}
		out = append(out, h)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) rewrite(items []Highlight) error {
	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return err
	}
	for _, h := range items {
		line, err := json.Marshal(h)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := fmt.Fprintln(f, string(line)); err != nil {
			f.Close()
			return err
		}
	}
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Export(w io.Writer) error {
	all, err := s.List()
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	groups := map[string][]Highlight{}
	for _, h := range all {
		groups[h.AbsPath] = append(groups[h.AbsPath], h)
	}
	paths := make([]string, 0, len(groups))
	for p := range groups {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintf(bw, "## %s\n\n", p)
		for _, h := range groups[p] {
			fmt.Fprintf(bw, "**%s:%d-%d** · %s\n\n", p, h.LineStart, h.LineEnd, h.CreatedAt.Format(time.RFC3339))
			for line := range strings.SplitSeq(h.Text, "\n") {
				fmt.Fprintf(bw, "> %s\n", line)
			}
			if h.Note != "" {
				fmt.Fprintf(bw, "\n_note:_ %s\n", h.Note)
			}
			fmt.Fprintln(bw)
		}
	}
	return nil
}

var (
	ulidEntropy ulid.MonotonicReader
	ulidMu      sync.Mutex
)

func newULID() string {
	ulidMu.Lock()
	defer ulidMu.Unlock()
	if ulidEntropy == nil {
		ulidEntropy = ulid.Monotonic(rand.Reader, 0)
	}
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulidEntropy).String()
}
