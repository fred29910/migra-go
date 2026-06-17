package render

import (
	"context"
	"encoding/json"

	"github.com/fred29910/migra-go/internal/diff"
)

// OpInfo represents a JSON-serializable summary of a diff operation.
type OpInfo struct {
	Kind        string `json:"kind"`
	ObjectKey   string `json:"object_key"`
	Destructive bool   `json:"destructive"`
	SQL         string `json:"sql"`
}

func (r *Renderer) renderJSON(ctx context.Context, ops []diff.Operation) (string, error) {
	infos := make([]OpInfo, len(ops))
	for i, op := range ops {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		obj := op.ObjectKey()
		infos[i] = OpInfo{
			Kind:        string(op.Kind()),
			ObjectKey:   obj.Schema + "." + obj.Name,
			Destructive: op.IsDestructive(),
			SQL:         r.Render(op),
		}
	}

	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
