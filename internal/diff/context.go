package diff

import (
	"context"
	"fmt"
)

// diffContext holds the local state during a single diff operation
type diffContext struct {
	ctx      context.Context
	ops      []Operation
	warnings []string
}

// newDiffContext creates a new diffContext with the given background context.
func newDiffContext(ctx context.Context) *diffContext {
	return &diffContext{
		ctx:      ctx,
		ops:      make([]Operation, 0, 16),
		warnings: make([]string, 0, 4),
	}
}

// checkCancelled checks if the context has been cancelled.
func (c *diffContext) checkCancelled() error {
	return c.ctx.Err()
}

func (c *diffContext) addOp(op Operation) {
	c.ops = append(c.ops, op)
}

func (c *diffContext) warnf(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}
