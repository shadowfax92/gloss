package render

import (
	"bytes"
	"fmt"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type Renderer struct {
	md goldmark.Markdown
}

func New() *Renderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			extension.DefinitionList,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(false),
					chromahtml.WithLineNumbers(false),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
	registerLineStamper(md)
	return &Renderer{md: md}
}

type Output struct {
	HTML      string `json:"html"`
	LineCount int    `json:"line_count"`
}

func (r *Renderer) Render(source []byte) (out *Output, err error) {
	// Goldmark is robust but defensive: a malformed input or a bug in the
	// custom block renderer shouldn't crash the daemon. Recover any panic
	// and replace the return with a plaintext fallback render.
	defer func() {
		if rec := recover(); rec != nil {
			out = fallback(source, fmt.Errorf("renderer panic: %v", rec))
			err = nil
		}
	}()

	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := r.md.Convert(source, &buf, parser.WithContext(ctx)); err != nil {
		return fallback(source, err), nil
	}
	return &Output{
		HTML:      buf.String(),
		LineCount: bytes.Count(source, []byte("\n")) + 1,
	}, nil
}

func fallback(source []byte, err error) *Output {
	return &Output{
		HTML: fmt.Sprintf(
			`<div class="render-error"><strong>Render error:</strong> %s</div><pre class="raw-fallback">%s</pre>`,
			htmlEscape(err.Error()),
			htmlEscape(string(source)),
		),
		LineCount: bytes.Count(source, []byte("\n")) + 1,
	}
}

func htmlEscape(s string) string {
	var b bytes.Buffer
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
