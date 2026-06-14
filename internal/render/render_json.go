package render

import (
	"encoding/json"

	"github.com/fred29910/migra-go/internal/diff"
)

type OpInfo struct {
	Kind        string `json:"kind"`
	ObjectKey   string `json:"object_key"`
	Destructive bool   `json:"destructive"`
	SQL         string `json:"sql"`
}

func renderJSON(ops []diff.Operation) (string, error) {
	r := NewRenderer()
	infos := make([]OpInfo, len(ops))
	for i, op := range ops {
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
