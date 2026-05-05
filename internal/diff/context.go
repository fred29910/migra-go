package diff

import (
	"fmt"
)

// diffContext holds the local state during a single diff operation
type diffContext struct {
	ops      []Operation
	warnings []string
}

func (c *diffContext) addOp(op Operation) {
	c.ops = append(c.ops, op)
}

func (c *diffContext) warnf(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}
