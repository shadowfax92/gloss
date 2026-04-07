package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"gloss/internal/config"
	"gloss/internal/highlights"
	"gloss/internal/paths"
	"gloss/internal/recent"
	"gloss/internal/render"
	"gloss/internal/watch"
)

const daemonProtocolVersion = 1

type Daemon struct {
	cfg      *config.Config
	token    string
	server   *http.Server
	port     int
	hl       *highlights.Store
	rec      *recent.Reader
	renderer *render.Renderer
	sse      *sseHub

	mu      sync.RWMutex
	folders map[string]*Folder // by folderID

	stopOnce sync.Once
	stopCh   chan struct{}
}

type DaemonInfo struct {
	Version   int    `json:"version"`
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	StartedAt string `json:"started_at"`
	Token     string `json:"token"`
}

// RunDetached is the entry point for the long-lived background process. It
// binds the listener, writes server.json, installs signal handlers, and blocks
// until shutdown.
func RunDetached(cfg *config.Config) error {
	if err := paths.EnsureAll(); err != nil {
		return err
	}
	hl, err := highlights.New(cfg.Highlights.Path)
	if err != nil {
		return err
	}
	d := &Daemon{
		cfg:      cfg,
		token:    newToken(),
		hl:       hl,
		rec:      recent.New(),
		renderer: render.New(),
		sse:      newSSEHub(),
		folders:  map[string]*Folder{},
		stopCh:   make(chan struct{}),
	}

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	d.port = ln.Addr().(*net.TCPAddr).Port

	if err := writeServerJSON(DaemonInfo{
		Version:   daemonProtocolVersion,
		PID:       os.Getpid(),
		Port:      d.port,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Token:     d.token,
	}); err != nil {
		ln.Close()
		return err
	}
	defer os.Remove(paths.ServerJSON())

	d.server = &http.Server{
		Handler:           d.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigs
		d.shutdown()
	}()

	fmt.Fprintf(os.Stderr, "gloss daemon listening on http://127.0.0.1:%d\n", d.port)
	err = d.server.Serve(ln)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (d *Daemon) shutdown() {
	d.stopOnce.Do(func() {
		close(d.stopCh)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.server.Shutdown(ctx)
		d.mu.Lock()
		for _, f := range d.folders {
			if f.Watcher != nil {
				f.Watcher.Close()
			}
		}
		d.mu.Unlock()
	})
}

// addOrGetFolder ensures the daemon has an open folder rooted at absPath. The
// caller must pass an absolute path; symlinks are resolved.
func (d *Daemon) addOrGetFolder(absPath string) (*Folder, error) {
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		resolved = absPath
	}
	id := folderID(resolved)
	d.mu.Lock()
	defer d.mu.Unlock()
	if f, ok := d.folders[id]; ok {
		return f, nil
	}
	w, err := watch.New(resolved, d.cfg.Ignore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "watch: %s: %v\n", resolved, err)
	}
	f := newFolder(resolved, w)
	d.folders[id] = f
	if w != nil {
		go d.consumeWatcher(f)
	}
	return f, nil
}

func (d *Daemon) consumeWatcher(f *Folder) {
	for {
		select {
		case <-d.stopCh:
			return
		case ev, ok := <-f.Watcher.Events():
			if !ok {
				return
			}
			rel, err := filepath.Rel(f.AbsPath, ev.Path)
			if err != nil {
				continue
			}
			f.InvalidateTree()
			d.sse.broadcast("file-changed", map[string]any{
				"folder_id": f.ID,
				"path":      filepath.ToSlash(rel),
			})
			d.sse.broadcast("tree-changed", map[string]any{
				"folder_id": f.ID,
			})
		}
	}
}

func writeServerJSON(info DaemonInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.ServerJSON()), 0755); err != nil {
		return err
	}
	tmp := paths.ServerJSON() + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, paths.ServerJSON())
}
