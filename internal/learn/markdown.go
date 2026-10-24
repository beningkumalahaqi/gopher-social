package learn

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// RepoURL is used when a lesson links to a file that has no learning page
// (solutions and raw examples are read on the repository host).
const RepoURL = "https://github.com/haqi/gophersocial"

var numberPrefix = regexp.MustCompile(`^[0-9]{2}\s*[.\x{2014}-]\s*`)

// Renderer parses Markdown once and returns sanitized HTML plus extracted
// metadata. It is safe for concurrent use.
type Renderer struct {
	md     goldmark.Markdown
	policy *bluemonday.Policy
}

// NewRenderer wires Goldmark (GFM + Chroma highlighting) and a sanitizer that
// allows the markup the curriculum actually uses, nothing more.
func NewRenderer() *Renderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(linkTransformer{}, 100)),
		),
	)
	return &Renderer{md: md, policy: sanitizer()}
}

func sanitizer() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowElements("details", "summary", "kbd", "mark", "figure", "figcaption", "abbr")
	p.AllowAttrs("id").Matching(regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)).OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^[a-zA-Z0-9 _:-]+$`)).
		OnElements("pre", "code", "span", "div", "table", "thead", "tbody", "tr", "th", "td")
	p.AllowAttrs("type", "checked", "disabled").OnElements("input")
	p.AllowAttrs("start", "reversed").OnElements("ol")
	p.AllowAttrs("align").OnElements("th", "td")
	p.AllowAttrs("target").Matching(regexp.MustCompile(`^_blank$`)).OnElements("a")
	p.AllowAttrs("rel").Matching(regexp.MustCompile(`^[a-z ]+$`)).OnElements("a")
	p.AllowAttrs("aria-label").OnElements("a", "span", "div")
	return p
}

// Parsed is the result of rendering one Markdown document.
type Parsed struct {
	Title   string
	HTML    template.HTML
	TOC     []Heading
	Text    string
	Code    []string
	Words   int
	Summary string
}

// Render parses and sanitizes a document. Malformed Markdown cannot panic the
// server: Goldmark is tolerant, and anything the sanitizer rejects is dropped.
func (r *Renderer) Render(source []byte) (Parsed, error) {
	doc := r.md.Parser().Parse(text.NewReader(source))
	meta := extract(doc, source)
	var buf bytes.Buffer
	if err := r.md.Renderer().Render(&buf, source, doc); err != nil {
		return Parsed{}, fmt.Errorf("render markdown: %w", err)
	}
	safe := r.policy.SanitizeBytes(buf.Bytes())
	meta.HTML = template.HTML(safe) // #nosec G203 -- sanitized above.
	meta.Words = len(strings.Fields(meta.Text))
	if meta.Words > 0 && meta.Words < 40 {
		meta.Words = 40
	}
	meta.Summary = summarize(meta.Text)
	return meta, nil
}

func extract(doc ast.Node, source []byte) Parsed {
	var out Parsed
	var plain strings.Builder
	firstParagraph := ""
	sawTitle := false

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Heading:
			txt := strings.TrimSpace(nodeText(node, source))
			if node.Level == 1 && !sawTitle {
				out.Title = stripNumberPrefix(txt)
				sawTitle = true
				return ast.WalkContinue, nil
			}
			if node.Level == 2 || node.Level == 3 {
				id := ""
				if attr, ok := node.AttributeString("id"); ok {
					if raw, ok := attr.([]byte); ok {
						id = string(raw)
					}
				}
				if id != "" {
					out.TOC = append(out.TOC, Heading{Level: node.Level, ID: id, Text: txt})
				}
			}
			plain.WriteString(txt)
			plain.WriteByte(' ')
		case *ast.FencedCodeBlock:
			lines := node.Lines()
			var code strings.Builder
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				code.Write(seg.Value(source))
			}
			out.Code = append(out.Code, code.String())
			plain.WriteString(code.String())
			plain.WriteByte(' ')
		case *ast.CodeBlock:
			lines := node.Lines()
			var code strings.Builder
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				code.Write(seg.Value(source))
			}
			out.Code = append(out.Code, code.String())
			plain.WriteString(code.String())
		case *ast.Paragraph:
			if firstParagraph == "" {
				txt := strings.TrimSpace(nodeText(node, source))
				if len([]rune(txt)) >= 40 {
					firstParagraph = txt
				}
			}
		case *ast.Text:
			plain.WriteString(string(node.Segment.Value(source)))
			if node.SoftLineBreak() || node.HardLineBreak() {
				plain.WriteByte('\n')
			}
		case *ast.String:
			plain.Write(node.Value)
		}
		return ast.WalkContinue, nil
	})
	out.Text = strings.Join(strings.Fields(plain.String()), " ")
	if firstParagraph != "" {
		out.Text = firstParagraph + " " + out.Text
	}
	return out
}

func nodeText(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := child.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(source))
		case *ast.String:
			b.Write(t.Value)
		case *ast.CodeSpan:
			b.WriteString(strings.TrimSpace(string(t.Text(source))))
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

func stripNumberPrefix(title string) string {
	trimmed := strings.TrimSpace(numberPrefix.ReplaceAllString(title, ""))
	if trimmed == "" {
		return title
	}
	return trimmed
}

func summarize(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= 180 {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:177])) + "..."
}

// linkTransformer rewrites relative Markdown links into learning-platform URLs
// so a lesson can link a sibling file without knowing the web routes.
type linkTransformer struct{}

func (linkTransformer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		dest := string(link.Destination)
		if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") || strings.HasPrefix(dest, "mailto:") {
			link.SetAttributeString("target", []byte("_blank"))
			link.SetAttributeString("rel", []byte("noopener noreferrer"))
			return ast.WalkContinue, nil
		}
		if rewritten, changed := RewriteLink(dest); changed {
			link.Destination = []byte(rewritten)
		}
		return ast.WalkContinue, nil
	})
}

// RewriteLink maps a curriculum-relative link to a route. It is exported so
// tests and the content loader can assert the same behaviour.
func RewriteLink(dest string) (string, bool) {
	if dest == "" || strings.HasPrefix(dest, "#") || strings.HasPrefix(dest, "/") {
		return dest, false
	}
	pathPart, fragment, hasFragment := strings.Cut(dest, "#")
	pathPart = strings.TrimPrefix(pathPart, "./")

	var base string
	switch {
	case pathPart == "README.md":
		base = "/learn"
	case pathPart == "glossary.md":
		base = "/learn/glossary"
	case strings.HasPrefix(pathPart, "exercises/"):
		base = "/learn/exercises/" + strings.TrimSuffix(strings.TrimPrefix(pathPart, "exercises/"), ".md")
	case strings.HasPrefix(pathPart, "solutions/"):
		base = RepoURL + "/blob/main/docs-learn/" + pathPart
	case strings.HasPrefix(pathPart, "examples/"):
		base = RepoURL + "/tree/main/docs-learn/" + pathPart
	case strings.HasPrefix(pathPart, "../"):
		base = RepoURL + "/blob/main/" + strings.TrimPrefix(pathPart, "../")
	case strings.HasSuffix(pathPart, ".md") && regexp.MustCompile(`^[0-9]{2}-`).MatchString(pathPart):
		base = "/learn/lessons/" + strings.TrimSuffix(pathPart, ".md")
	default:
		return dest, false
	}
	if hasFragment && fragment != "" {
		base += "#" + fragment
	}
	return base, true
}
