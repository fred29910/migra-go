package render

import (
	"context"
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/diff"
)

// SQLEngine defines the interface for SQL rendering.
// Implementations can render diff operations into SQL statements.
type SQLEngine interface {
	RenderAll(ctx context.Context, ops []diff.Operation) (string, error)
}

var _ SQLEngine = (*Renderer)(nil)

// Renderer provides SQL rendering capabilities.
type Renderer struct {
	useIfExists bool
}

// NewRenderer creates a new Renderer with default settings.
func NewRenderer() *Renderer {
	return &Renderer{
		useIfExists: true,
	}
}

// UseIfExists implements diff.RenderContext.
func (r *Renderer) UseIfExists() bool {
	return r.useIfExists
}

// RenderOutput renders all operations in the specified format ("sql" or "json").
func (r *Renderer) RenderOutput(ctx context.Context, ops []diff.Operation, format string) (string, error) {
	switch format {
	case "sql":
		return r.RenderAll(ctx, ops)
	case "json":
		return r.renderJSON(ctx, ops)
	default:
		return "", fmt.Errorf("unsupported format: %s (allowed: sql, json)", format)
	}
}

// RenderAll renders all operations as a single SQL string.
func (r *Renderer) RenderAll(ctx context.Context, ops []diff.Operation) (string, error) {
	var b strings.Builder
	for _, op := range ops {
		if err := ctx.Err(); err != nil {
			return "", err
		}
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
		return "-- No changes detected", nil
	}
	b.WriteString("\n-- End Diff")
	return b.String(), nil
}

// Render renders a single operation as SQL by delegating to the operation's RenderString method.
func (r *Renderer) Render(op diff.Operation) string {
	return op.RenderString(r)
}
