package walk

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxDepth   = 8
	maxEntries = 5000
)

type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"` // relative to walk root, forward slashes
	IsDir    bool    `json:"is_dir"`
	Children []*Node `json:"children,omitempty"`
}

type Result struct {
	Root      string `json:"root"`
	Tree      *Node  `json:"tree"`
	Truncated bool   `json:"truncated"`
}

func Markdown(root string, ignore []string) (*Result, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		// Single-file mode: synthesize a tree containing just that file under
		// its parent directory so the UI still has somewhere to render.
		parent := filepath.Dir(abs)
		base := filepath.Base(abs)
		return &Result{
			Root: parent,
			Tree: &Node{
				Name:  filepath.Base(parent),
				Path:  "",
				IsDir: true,
				Children: []*Node{
					{Name: base, Path: base, IsDir: false},
				},
			},
		}, nil
	}

	ig := buildIgnoreSet(ignore)
	files, truncated, err := collect(abs, ig)
	if err != nil {
		return nil, err
	}
	tree := build(abs, files)
	return &Result{Root: abs, Tree: tree, Truncated: truncated}, nil
}

func collect(root string, ignore map[string]struct{}) ([]string, bool, error) {
	var files []string
	truncated := false
	rootDepth := strings.Count(root, string(os.PathSeparator))

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") && name != "." {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if _, skip := ignore[name]; skip {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		depth := strings.Count(path, string(os.PathSeparator)) - rootDepth
		if depth > maxDepth {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !isMarkdown(name) {
			return nil
		}
		files = append(files, path)
		if len(files) >= maxEntries {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, truncated, err
	}
	return files, truncated, nil
}

func build(root string, files []string) *Node {
	rootNode := &Node{Name: filepath.Base(root), Path: "", IsDir: true}
	dirIndex := map[string]*Node{"": rootNode}
	for _, abs := range files {
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		dirRel := ""
		parent := rootNode
		for i := 0; i < len(parts)-1; i++ {
			seg := parts[i]
			next := dirRel
			if next == "" {
				next = seg
			} else {
				next = next + "/" + seg
			}
			if existing, ok := dirIndex[next]; ok {
				parent = existing
				dirRel = next
				continue
			}
			dn := &Node{Name: seg, Path: next, IsDir: true}
			parent.Children = append(parent.Children, dn)
			dirIndex[next] = dn
			parent = dn
			dirRel = next
		}
		parent.Children = append(parent.Children, &Node{
			Name:  parts[len(parts)-1],
			Path:  rel,
			IsDir: false,
		})
	}
	sortTree(rootNode)
	return rootNode
}

func sortTree(n *Node) {
	if !n.IsDir {
		return
	}
	sort.SliceStable(n.Children, func(i, j int) bool {
		a, b := n.Children[i], n.Children[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	for _, c := range n.Children {
		sortTree(c)
	}
}

func buildIgnoreSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, it := range items {
		if it == "" {
			continue
		}
		out[it] = struct{}{}
	}
	return out
}

func isMarkdown(name string) bool {
	low := strings.ToLower(name)
	return strings.HasSuffix(low, ".md") || strings.HasSuffix(low, ".markdown")
}
