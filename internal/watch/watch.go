package watch

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"gloss/internal/mdfile"
)

type Event struct {
	Path string
}

type Watcher struct {
	root    string
	ignore  map[string]struct{}
	out     chan Event
	stop    chan struct{}
	once    sync.Once
	fsw     *fsnotify.Watcher
	useStat bool
}

func New(root string, ignore []string) (*Watcher, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		root:   abs,
		ignore: setOf(ignore),
		out:    make(chan Event, 16),
		stop:   make(chan struct{}),
	}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		w.useStat = true
		go w.statLoop()
		return w, nil
	}
	w.fsw = fw
	if err := w.addRecursive(abs); err != nil {
		fw.Close()
		w.useStat = true
		go w.statLoop()
		return w, nil
	}
	go w.fsLoop()
	return w, nil
}

func (w *Watcher) Events() <-chan Event { return w.out }

func (w *Watcher) Close() {
	w.once.Do(func() {
		close(w.stop)
		if w.fsw != nil {
			w.fsw.Close()
		}
	})
}

func (w *Watcher) Root() string { return w.root }

func (w *Watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != root && strings.HasPrefix(name, ".") {
			return fs.SkipDir
		}
		if _, skip := w.ignore[name]; skip {
			return fs.SkipDir
		}
		return w.fsw.Add(path)
	})
}

func (w *Watcher) fsLoop() {
	debounce := newDebouncer(100 * time.Millisecond)
	for {
		select {
		case <-w.stop:
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if !relevant(ev.Name) {
				continue
			}
			if ev.Op&fsnotify.Create != 0 {
				if info, err := readDir(ev.Name); err == nil && info {
					_ = w.fsw.Add(ev.Name)
				}
			}
			debounce.fire(ev.Name, func(p string) {
				w.emit(Event{Path: p})
			})
		case <-w.fsw.Errors:
			// swallow; fsnotify can flake on macOS rename storms.
		}
	}
}

func (w *Watcher) statLoop() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	prev := snapshot(w.root, w.ignore)
	for {
		select {
		case <-w.stop:
			return
		case <-tick.C:
			cur := snapshot(w.root, w.ignore)
			for p, mt := range cur {
				if prev[p] != mt {
					w.emit(Event{Path: p})
				}
			}
			prev = cur
		}
	}
}

func (w *Watcher) emit(e Event) {
	select {
	case w.out <- e:
	default:
	}
}

type debouncer struct {
	d     time.Duration
	mu    sync.Mutex
	timer *time.Timer
	last  string
}

func newDebouncer(d time.Duration) *debouncer { return &debouncer{d: d} }

func (b *debouncer) fire(p string, fn func(string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.last = p
	if b.timer != nil {
		b.timer.Stop()
	}
	b.timer = time.AfterFunc(b.d, func() {
		b.mu.Lock()
		latest := b.last
		b.mu.Unlock()
		fn(latest)
	})
}

func snapshot(root string, ignore map[string]struct{}) map[string]int64 {
	out := map[string]int64{}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			if _, skip := ignore[name]; skip {
				return fs.SkipDir
			}
			return nil
		}
		if !relevant(name) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out[path] = info.ModTime().UnixNano()
		return nil
	})
	return out
}

func relevant(p string) bool {
	return mdfile.Is(p)
}

func readDir(path string) (bool, error) {
	info, err := fileInfo(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func setOf(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, it := range items {
		out[it] = struct{}{}
	}
	return out
}
