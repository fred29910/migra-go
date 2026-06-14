package render

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/diff"
)

type SQLEngine interface {
	RenderAll(ops []diff.Operation) string
}

var _ SQLEngine = (*Renderer)(nil)

// Renderer provides SQL rendering capabilities.
type Renderer struct {
	useIfExists bool
}

func NewRenderer() *Renderer {
	return &Renderer{
		useIfExists: true,
	}
}

// UseIfExists implements diff.RenderContext.
func (r *Renderer) UseIfExists() bool {
	return r.useIfExists
}

func (r *Renderer) RenderOutput(ops []diff.Operation, format string) (string, error) {
	switch format {
	case "sql":
		return r.RenderAll(ops), nil
	case "json":
		return renderJSON(ops)
	default:
		return "", fmt.Errorf("unsupported format: %s (allowed: sql, json)", format)
	}
}

func (r *Renderer) RenderAll(ops []diff.Operation) string {
	var b strings.Builder
	for _, op := range ops {
		sql := r.Render(op)
		if sql == "" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("-- Begin Diff\n")
		} else {
			b.WriteByte('\n')
		}
		b.WriteString(sql)
	}

	if b.Len() == 0 {
		return "-- No changes detected"
	}
	b.WriteString("\n-- End Diff")
	return b.String()
}

// Render delegates rendering to the operation's own RenderString method.
func (r *Renderer) Render(op diff.Operation) string {
	return op.RenderString(r)
}
