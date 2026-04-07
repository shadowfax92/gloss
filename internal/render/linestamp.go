package render

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// registerLineStamper installs a renderer that wraps every block element with
// data-line-start / data-line-end attributes pointing back to the source
// markdown line numbers. The browser uses these to map text selections back
// to source line ranges for the "Copy with ref" feature.
func registerLineStamper(md goldmark.Markdown) {
	md.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&lineStamper{}, 100),
		),
	)
}

type lineStamper struct{}

func (s *lineStamper) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, s.renderHeading)
	reg.Register(ast.KindParagraph, s.renderParagraph)
	reg.Register(ast.KindCodeBlock, s.renderCodeBlock)
	reg.Register(ast.KindFencedCodeBlock, s.renderFencedCodeBlock)
	reg.Register(ast.KindBlockquote, s.renderBlockquote)
	reg.Register(ast.KindList, s.renderList)
	reg.Register(ast.KindListItem, s.renderListItem)
	reg.Register(ast.KindThematicBreak, s.renderThematicBreak)
}

func (s *lineStamper) renderHeading(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	h := n.(*ast.Heading)
	if entering {
		start, end := lineRange(source, n)
		fmt.Fprintf(w, `<h%d data-line-start="%d" data-line-end="%d">`, h.Level, start, end)
		return ast.WalkContinue, nil
	}
	fmt.Fprintf(w, `</h%d>`+"\n", h.Level)
	return ast.WalkContinue, nil
}

func (s *lineStamper) renderParagraph(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if _, ok := n.Parent().(*ast.ListItem); ok {
			// Tight list items hide their paragraph wrapper to match GFM rendering.
			if list, ok := n.Parent().Parent().(*ast.List); ok && !list.IsTight {
				start, end := lineRange(source, n)
				fmt.Fprintf(w, `<p data-line-start="%d" data-line-end="%d">`, start, end)
				return ast.WalkContinue, nil
			}
			return ast.WalkContinue, nil
		}
		start, end := lineRange(source, n)
		fmt.Fprintf(w, `<p data-line-start="%d" data-line-end="%d">`, start, end)
		return ast.WalkContinue, nil
	}
	if _, ok := n.Parent().(*ast.ListItem); ok {
		if list, ok := n.Parent().Parent().(*ast.List); ok && list.IsTight {
			return ast.WalkContinue, nil
		}
	}
	w.WriteString("</p>\n")
	return ast.WalkContinue, nil
}

func (s *lineStamper) renderBlockquote(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		start, end := lineRange(source, n)
		fmt.Fprintf(w, `<blockquote data-line-start="%d" data-line-end="%d">`+"\n", start, end)
		return ast.WalkContinue, nil
	}
	w.WriteString("</blockquote>\n")
	return ast.WalkContinue, nil
}

func (s *lineStamper) renderList(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	list := n.(*ast.List)
	tag := "ul"
	if list.IsOrdered() {
		tag = "ol"
	}
	if entering {
		start, end := lineRange(source, n)
		startAttr := ""
		if list.IsOrdered() && list.Start != 1 {
			startAttr = fmt.Sprintf(` start="%d"`, list.Start)
		}
		fmt.Fprintf(w, `<%s data-line-start="%d" data-line-end="%d"%s>`+"\n", tag, start, end, startAttr)
		return ast.WalkContinue, nil
	}
	fmt.Fprintf(w, "</%s>\n", tag)
	return ast.WalkContinue, nil
}

func (s *lineStamper) renderListItem(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		start, end := lineRange(source, n)
		fmt.Fprintf(w, `<li data-line-start="%d" data-line-end="%d">`, start, end)
		return ast.WalkContinue, nil
	}
	w.WriteString("</li>\n")
	return ast.WalkContinue, nil
}

func (s *lineStamper) renderCodeBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		w.WriteString("</code></pre>\n")
		return ast.WalkContinue, nil
	}
	start, end := lineRange(source, n)
	fmt.Fprintf(w, `<pre data-line-start="%d" data-line-end="%d"><code>`, start, end)
	writeLines(w, source, n)
	return ast.WalkContinue, nil
}

func (s *lineStamper) renderFencedCodeBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		w.WriteString("</code></pre>\n")
		return ast.WalkContinue, nil
	}
	fcb := n.(*ast.FencedCodeBlock)
	lang := ""
	if fcb.Info != nil {
		lang = string(fcb.Info.Segment.Value(source))
	}
	start, end := lineRange(source, n)
	if lang != "" {
		fmt.Fprintf(w, `<pre data-line-start="%d" data-line-end="%d"><code class="language-%s">`, start, end, escapeAttr(lang))
	} else {
		fmt.Fprintf(w, `<pre data-line-start="%d" data-line-end="%d"><code>`, start, end)
	}
	writeLines(w, source, n)
	return ast.WalkContinue, nil
}

func (s *lineStamper) renderThematicBreak(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	start, end := lineRange(source, n)
	fmt.Fprintf(w, `<hr data-line-start="%d" data-line-end="%d" />`+"\n", start, end)
	return ast.WalkContinue, nil
}

// lineRange returns the 1-based [start, end] source line range for any block
// node by inspecting its first/last segment positions.
func lineRange(source []byte, n ast.Node) (int, int) {
	lines := n.Lines()
	if lines.Len() == 0 {
		// Containers like list / blockquote may have no direct text segments;
		// recurse into children for an outer envelope.
		var first, last text.Segment
		gotFirst := false
		ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering || c == n {
				return ast.WalkContinue, nil
			}
			cl := c.Lines()
			if cl.Len() == 0 {
				return ast.WalkContinue, nil
			}
			if !gotFirst {
				first = cl.At(0)
				gotFirst = true
			}
			last = cl.At(cl.Len() - 1)
			return ast.WalkContinue, nil
		})
		if !gotFirst {
			return 1, 1
		}
		return offsetToLine(source, first.Start), offsetToLine(source, last.Stop-1)
	}
	first := lines.At(0)
	last := lines.At(lines.Len() - 1)
	return offsetToLine(source, first.Start), offsetToLine(source, last.Stop-1)
}

// offsetToLine maps a byte offset in source to a 1-based line number.
func offsetToLine(source []byte, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	return bytes.Count(source[:offset], []byte("\n")) + 1
}

func writeLines(w util.BufWriter, source []byte, n ast.Node) {
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		writeHTML(w, seg.Value(source))
	}
}

func writeHTML(w util.BufWriter, b []byte) {
	for _, c := range b {
		switch c {
		case '&':
			w.WriteString("&amp;")
		case '<':
			w.WriteString("&lt;")
		case '>':
			w.WriteString("&gt;")
		case '"':
			w.WriteString("&quot;")
		default:
			w.WriteByte(c)
		}
	}
}

func escapeAttr(s string) string {
	var b bytes.Buffer
	for _, r := range s {
		switch r {
		case '"', '<', '>', '&':
			fmt.Fprintf(&b, "&#%d;", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
