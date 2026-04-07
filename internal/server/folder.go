package server

import (
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"
	"sync"

	"gloss/internal/walk"
	"gloss/internal/watch"
)

type Folder struct {
	ID      string
	AbsPath string
	Name    string
	Watcher *watch.Watcher

	mu   sync.Mutex
	tree *walk.Result
}

func folderID(absPath string) string {
	sum := sha1.Sum([]byte(absPath))
	return hex.EncodeToString(sum[:])[:12]
}

func newFolder(absPath string, w *watch.Watcher) *Folder {
	return &Folder{
		ID:      folderID(absPath),
		AbsPath: absPath,
		Name:    filepath.Base(absPath),
		Watcher: w,
	}
}

func (f *Folder) Tree(ignore []string) (*walk.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tree != nil {
		return f.tree, nil
	}
	t, err := walk.Markdown(f.AbsPath, ignore)
	if err != nil {
		return nil, err
	}
	f.tree = t
	return t, nil
}

func (f *Folder) InvalidateTree() {
	f.mu.Lock()
	f.tree = nil
	f.mu.Unlock()
}
