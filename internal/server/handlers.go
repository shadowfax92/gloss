package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gloss/internal/highlights"
	"gloss/internal/reltime"
	"gloss/web"
)

const maxRenderBytes = 5 * 1024 * 1024

func (d *Daemon) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", d.healthz)
	mux.Handle("/", d.tokenGate(http.HandlerFunc(d.indexHandler)))
	mux.Handle("/static/", http.StripPrefix("/static/", staticHandler()))

	mux.Handle("/api/open", d.tokenGate(http.HandlerFunc(d.openHandler)))
	mux.Handle("/api/folders", d.tokenGate(http.HandlerFunc(d.foldersHandler)))
	mux.Handle("/api/folders/", d.tokenGate(http.HandlerFunc(d.folderRoutes)))
	mux.Handle("/api/recent", d.tokenGate(http.HandlerFunc(d.recentHandler)))
	mux.Handle("/api/highlights", d.tokenGate(http.HandlerFunc(d.highlightsHandler)))
	mux.Handle("/api/highlights/", d.tokenGate(http.HandlerFunc(d.highlightItemHandler)))
	mux.Handle("/api/events", d.tokenGate(http.HandlerFunc(d.sseStream)))
	mux.Handle("/api/shutdown", d.tokenGate(http.HandlerFunc(d.shutdownHandler)))
	return mux
}

func staticHandler() http.Handler {
	sub, err := fs.Sub(web.Static, ".")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}

func (d *Daemon) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"version": daemonProtocolVersion,
		"port":    d.port,
		"ok":      true,
	})
}

func (d *Daemon) indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := web.Static.ReadFile("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

type openRequest struct {
	Folder string `json:"folder"`
	File   string `json:"file"`
}

type openResponse struct {
	FolderID string `json:"folder_id"`
	FileRel  string `json:"file_rel,omitempty"`
}

func (d *Daemon) openHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req openRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Folder == "" {
		http.Error(w, "folder required", http.StatusBadRequest)
		return
	}

	abs, err := filepath.Abs(req.Folder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	f, err := d.addOrGetFolder(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := openResponse{FolderID: f.ID}
	if req.File != "" {
		fileAbs, _ := filepath.Abs(req.File)
		if rel, err := filepath.Rel(f.AbsPath, fileAbs); err == nil && !strings.HasPrefix(rel, "..") {
			resp.FileRel = filepath.ToSlash(rel)
		}
	}
	writeJSON(w, resp)
}

type folderSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Root string `json:"root"`
}

func (d *Daemon) foldersHandler(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]folderSummary, 0, len(d.folders))
	for _, f := range d.folders {
		out = append(out, folderSummary{ID: f.ID, Name: f.Name, Root: f.AbsPath})
	}
	writeJSON(w, map[string]any{"folders": out})
}

func (d *Daemon) folderRoutes(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/folders/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	id, action := parts[0], parts[1]
	d.mu.RLock()
	f := d.folders[id]
	d.mu.RUnlock()
	if f == nil {
		http.Error(w, "folder not found", http.StatusNotFound)
		return
	}
	switch action {
	case "tree":
		d.serveTree(w, f)
	case "file":
		d.serveFile(w, r, f)
	case "asset":
		d.serveAsset(w, r, f)
	default:
		http.NotFound(w, r)
	}
}

func (d *Daemon) serveTree(w http.ResponseWriter, f *Folder) {
	tree, err := f.Tree(d.cfg.Ignore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"id":        f.ID,
		"name":      f.Name,
		"root":      f.AbsPath,
		"tree":      tree.Tree,
		"truncated": tree.Truncated,
	})
}

type fileResponse struct {
	Path      string `json:"path"`
	AbsPath   string `json:"abs_path"`
	Home      string `json:"home"`
	HTML      string `json:"html"`
	LineCount int    `json:"line_count"`
	CopyStyle string `json:"copy_style"`
	TooLarge  bool   `json:"too_large,omitempty"`
}

func (d *Daemon) serveFile(w http.ResponseWriter, r *http.Request, f *Folder) {
	rel := r.URL.Query().Get("path")
	if rel == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	abs := filepath.Join(f.AbsPath, filepath.FromSlash(rel))
	if !strings.HasPrefix(abs, f.AbsPath) {
		http.Error(w, "path escapes folder", http.StatusBadRequest)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	home, _ := os.UserHomeDir()
	resp := fileResponse{
		Path:      rel,
		AbsPath:   abs,
		Home:      home,
		CopyStyle: d.cfg.CopyPathStyle,
	}
	if info.Size() > maxRenderBytes {
		resp.TooLarge = true
		resp.HTML = `<div class="banner">File too large to preview — open in your editor.</div>`
		writeJSON(w, resp)
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out, err := d.renderer.Render(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp.HTML = out.HTML
	resp.LineCount = out.LineCount
	writeJSON(w, resp)
}

func (d *Daemon) serveAsset(w http.ResponseWriter, r *http.Request, f *Folder) {
	rel := r.URL.Query().Get("path")
	if rel == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	abs := filepath.Join(f.AbsPath, filepath.FromSlash(rel))
	if !strings.HasPrefix(abs, f.AbsPath) {
		http.Error(w, "path escapes folder", http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, abs)
}

func (d *Daemon) recentHandler(w http.ResponseWriter, r *http.Request) {
	entries, err := d.rec.ListMarkdown(d.cfg.Recent.Days, d.cfg.Recent.Max)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]any{
			"path":     e.Path,
			"time":     e.Time.UTC().Format(time.RFC3339),
			"relative": reltime.Format(e.Time),
		})
	}
	writeJSON(w, map[string]any{"entries": out})
}

func (d *Daemon) highlightsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		all, err := d.hl.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"highlights": all})
	case http.MethodPost:
		var h highlights.Highlight
		if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if h.AbsPath == "" || h.LineStart < 1 || h.LineEnd < h.LineStart {
			http.Error(w, "invalid highlight", http.StatusBadRequest)
			return
		}
		saved, err := d.hl.Add(h)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, saved)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (d *Daemon) highlightItemHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/highlights/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h, err := d.hl.Get(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, h)
	case http.MethodDelete:
		if err := d.hl.Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (d *Daemon) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
	go func() {
		time.Sleep(50 * time.Millisecond)
		d.shutdown()
	}()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "encode error:", err)
	}
}

