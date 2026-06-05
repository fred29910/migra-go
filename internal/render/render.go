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

type Renderer struct {
	useIfExists bool
}

func NewRenderer() *Renderer {
	return &Renderer{
		useIfExists: true,
	}
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

func (r *Renderer) Render(op diff.Operation) string {
	switch v := op.(type) {
	case *diff.AddTableOp:
		return renderAddTable(r, v)
	case *diff.DropTableOp:
		return renderDropTable(r, v)
	case *diff.AddColumnOp:
		return renderAddColumn(r, v)
	case *diff.AlterColumnTypeOp:
		return renderAlterColumnType(r, v)
	case *diff.SetNotNullOp:
		return renderSetNotNull(r, v)
	case *diff.DropNotNullOp:
		return renderDropNotNull(r, v)
	case *diff.CreateIndexOp:
		return renderCreateIndex(r, v)
	case *diff.DropIndexOp:
		return renderDropIndex(r, v)
	case *diff.AddConstraintOp:
		return renderAddConstraint(r, v)
	case *diff.DropConstraintOp:
		return renderDropConstraint(r, v)
	case *diff.AddEnumTypeOp:
		return renderAddEnumType(r, v)
	case *diff.DropEnumTypeOp:
		return renderDropEnumType(r, v)
	case *diff.AddEnumLabelOp:
		return renderAddEnumLabel(r, v)
	case *diff.SetDefaultOp:
		return renderSetDefault(r, v)
	case *diff.DropDefaultOp:
		return renderDropDefault(r, v)
	case *diff.AlterColumnCollationOp:
		return renderAlterColumnCollation(r, v)
	case *diff.CreateSchemaOp:
		return renderCreateSchema(r, v)
	case *diff.DropSchemaOp:
		return renderDropSchema(r, v)
	case *diff.AddIdentityOp:
		return renderAddIdentity(r, v)
	case *diff.SetIdentityOp:
		return renderSetIdentity(r, v)
	case *diff.DropIdentityOp:
		return renderDropIdentity(r, v)
	case *diff.RenameColumnOp:
		return renderRenameColumn(r, v)
	case *diff.DropColumnOp:
		return renderDropColumn(r, v)
	case *diff.CreateViewOp:
		return renderCreateView(r, v)
	case *diff.DropViewOp:
		return renderDropView(r, v)
	case *diff.ReplaceViewOp:
		return renderReplaceView(r, v)
	case *diff.CreateMaterializedViewOp:
		return renderCreateMaterializedView(r, v)
	case *diff.DropMaterializedViewOp:
		return renderDropMaterializedView(r, v)
	case *diff.CreateSequenceOp:
		return renderCreateSequence(r, v)
	case *diff.DropSequenceOp:
		return renderDropSequence(r, v)
	case *diff.AlterSequenceOp:
		return renderAlterSequence(r, v)
	case *diff.CreateExtensionOp:
		return renderCreateExtension(r, v)
	case *diff.DropExtensionOp:
		return renderDropExtension(r, v)
	case *diff.AlterExtensionUpdateOp:
		return renderAlterExtensionUpdate(r, v)
	default:
		return fmt.Sprintf("-- Unknown operation: %T", op)
	}
}

func (r *Renderer) RenderSingle(op diff.Operation) string {
	return r.Render(op)
}
