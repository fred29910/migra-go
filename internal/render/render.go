package render

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

// SQLEngine defines the interface for SQL rendering.
type SQLEngine interface {
	RenderAll(ops []diff.Operation) string
}

// Compile-time check: Renderer must satisfy SQLEngine.
var _ SQLEngine = (*Renderer)(nil)

// Renderer converts diff operations into SQL statements
type Renderer struct {
	useIfExists bool
}

// NewRenderer creates a new Renderer
func NewRenderer() *Renderer {
	return &Renderer{
		useIfExists: true,
	}
}

// RenderOutput renders operations to the specified format
func (r *Renderer) RenderOutput(ops []diff.Operation, format string) (string, error) {
	switch format {
	case "sql":
		return r.RenderAll(ops), nil
	case "json":
		return RenderJSON(ops)
	default:
		return "", fmt.Errorf("unsupported format: %s (allowed: sql, json)", format)
	}
}

// RenderAll converts all operations to SQL
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

// Render converts a single operation to SQL
func (r *Renderer) Render(op diff.Operation) string {
	switch v := op.(type) {
	case *diff.AddTableOp:
		return r.renderAddTable(v)
	case *diff.DropTableOp:
		return r.renderDropTable(v)
	case *diff.AddColumnOp:
		return r.renderAddColumn(v)
	case *diff.AlterColumnTypeOp:
		return r.renderAlterColumnType(v)
	case *diff.SetNotNullOp:
		return r.renderSetNotNull(v)
	case *diff.DropNotNullOp:
		return r.renderDropNotNull(v)
	case *diff.CreateIndexOp:
		return r.renderCreateIndex(v)
	case *diff.DropIndexOp:
		return r.renderDropIndex(v)
	case *diff.AddConstraintOp:
		definition := renderConstraintDefinition(v.Constraint)
		if definition == "" {
			return ""
		}
		return fmt.Sprintf("-- op: add_constraint risk:medium\nALTER TABLE %s ADD CONSTRAINT %s %s;",
			quoteQualifiedIdentifier(v.Schema, v.Table),
			quoteIdentifier(v.Constraint.Name),
			definition,
		)
	case *diff.DropConstraintOp:
		return fmt.Sprintf("-- op: drop_constraint risk:medium\nALTER TABLE %s DROP CONSTRAINT %s%s;",
			quoteQualifiedIdentifier(v.Schema, v.Table),
			ifExistsPrefix(r.useIfExists),
			quoteIdentifier(v.Name),
		)
	case *diff.AddEnumTypeOp:
		return r.renderAddEnumType(v)
	case *diff.DropEnumTypeOp:
		return r.renderDropEnumType(v)
	case *diff.AddEnumLabelOp:
		return fmt.Sprintf("-- op: add_enum_label risk:low\nALTER TYPE %s ADD VALUE %s;",
			quoteQualifiedIdentifier(v.Schema, v.Type),
			quoteString(v.Label),
		)
	case *diff.SetDefaultOp:
		return fmt.Sprintf("-- op: set_default risk:low\nALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;",
			quoteQualifiedIdentifier(v.Schema, v.Table),
			quoteIdentifier(v.Column),
			v.DefaultExpr,
		)
	case *diff.DropDefaultOp:
		return fmt.Sprintf("-- op: drop_default risk:low\nALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;",
			quoteQualifiedIdentifier(v.Schema, v.Table),
			quoteIdentifier(v.Column),
		)
	case *diff.AlterColumnCollationOp:
		return r.renderAlterColumnCollation(v)
	case *diff.CreateSchemaOp:
		return r.renderCreateSchema(v)
	case *diff.DropSchemaOp:
		return r.renderDropSchema(v)
	case *diff.AddIdentityOp:
		return fmt.Sprintf("-- op: add_identity risk:low\nALTER TABLE %s ALTER COLUMN %s ADD GENERATED %s AS IDENTITY;",
			quoteQualifiedIdentifier(v.Schema, v.Table),
			quoteIdentifier(v.Column),
			v.IdentityKind,
		)
	case *diff.SetIdentityOp:
		return fmt.Sprintf("-- op: set_identity risk:low\nALTER TABLE %s ALTER COLUMN %s SET GENERATED %s;",
			quoteQualifiedIdentifier(v.Schema, v.Table),
			quoteIdentifier(v.Column),
			v.IdentityKind,
		)
	case *diff.DropIdentityOp:
		return fmt.Sprintf("-- op: drop_identity risk:medium\nALTER TABLE %s ALTER COLUMN %s DROP IDENTITY;",
			quoteQualifiedIdentifier(v.Schema, v.Table),
			quoteIdentifier(v.Column),
		)
	case *diff.RenameColumnOp:
		return fmt.Sprintf("-- op: rename_column risk:low\nALTER TABLE %s RENAME COLUMN %s TO %s;",
			quoteQualifiedIdentifier(v.Schema, v.Table),
			quoteIdentifier(v.OldName),
			quoteIdentifier(v.NewName),
		)
	case *diff.DropColumnOp:
		return fmt.Sprintf("-- op: drop_column risk:high\nALTER TABLE %s DROP COLUMN IF EXISTS %s;",
			quoteQualifiedIdentifier(v.Schema, v.Table),
			quoteIdentifier(v.Column),
		)
	case *diff.CreateViewOp:
		return r.renderCreateView(v)
	case *diff.DropViewOp:
		return r.renderDropView(v)
	case *diff.ReplaceViewOp:
		return r.renderReplaceView(v)
	case *diff.CreateSequenceOp:
		return r.renderCreateSequence(v)
	case *diff.DropSequenceOp:
		return r.renderDropSequence(v)
	case *diff.AlterSequenceOp:
		return r.renderAlterSequence(v)
	case *diff.CreateExtensionOp:
		return r.renderCreateExtension(v)
	case *diff.DropExtensionOp:
		return r.renderDropExtension(v)
	case *diff.AlterExtensionUpdateOp:
		return r.renderAlterExtensionUpdate(v)
	default:
		return fmt.Sprintf("-- Unknown operation: %T", op)
	}
}

// RenderSingle renders a single operation to SQL string
// Used by push command for interactive confirmation
func (r *Renderer) RenderSingle(op diff.Operation) string {
	return r.Render(op)
}

func (r *Renderer) renderAddTable(op *diff.AddTableOp) string {
	table := op.Table
	lines := make([]string, 0)

	for _, col := range table.Columns {
		colDef := fmt.Sprintf("    %s %s", quoteIdentifier(col.Name), col.DataType)
		if col.IsIdentity {
			switch col.IdentityKind {
			case "ALWAYS":
				colDef += " GENERATED ALWAYS AS IDENTITY"
			case "BY DEFAULT":
				colDef += " GENERATED BY DEFAULT AS IDENTITY"
			}
		}
		if col.Collation != "" {
			colDef += fmt.Sprintf(" COLLATE %s", quoteIdentifier(col.Collation))
		}
		if !col.IsNullable {
			colDef += " NOT NULL"
		}
		if col.DefaultExpr != nil {
			colDef += fmt.Sprintf(" DEFAULT %s", *col.DefaultExpr)
		}
		lines = append(lines, colDef)
	}

	if table.PrimaryKey != nil {
		pk := &model.Constraint{
			Name:    table.PrimaryKey.Name,
			Type:    "primary_key",
			Columns: table.PrimaryKey.Columns,
		}
		if rendered := renderConstraint(pk); rendered != "" {
			lines = append(lines, "    "+rendered)
		}
	}

	if len(table.Constraints) > 0 {
		names := make([]string, 0, len(table.Constraints))
		for name := range table.Constraints {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			c := table.Constraints[name]
			if table.PrimaryKey != nil && c.Name == table.PrimaryKey.Name {
				continue
			}
			if rendered := renderConstraint(c); rendered != "" {
				lines = append(lines, "    "+rendered)
			}
		}
	}

	sql := fmt.Sprintf("-- op: add_table risk:low\nCREATE TABLE %s (\n%s\n);",
		quoteQualifiedIdentifier(table.Schema, table.Name),
		strings.Join(lines, ",\n"))
	return sql
}

func (r *Renderer) renderDropTable(op *diff.DropTableOp) string {
	ifExists := ""
	if r.useIfExists {
		ifExists = "IF EXISTS "
	}
	return fmt.Sprintf("-- op: drop_table risk:high\nDROP TABLE %s%s;", ifExists, quoteQualifiedIdentifier(op.Schema, op.Name))
}

func (r *Renderer) renderAddColumn(op *diff.AddColumnOp) string {
	col := op.Column
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
		quoteQualifiedIdentifier(op.Schema, op.Table), quoteIdentifier(col.Name), col.DataType)

	if col.IsIdentity {
		switch col.IdentityKind {
		case "ALWAYS":
			sql += " GENERATED ALWAYS AS IDENTITY"
		case "BY DEFAULT":
			sql += " GENERATED BY DEFAULT AS IDENTITY"
		}
	}

	if col.Collation != "" {
		sql += fmt.Sprintf(" COLLATE %s", quoteIdentifier(col.Collation))
	}
	if !col.IsNullable {
		sql += " NOT NULL"
	}
	if col.DefaultExpr != nil {
		sql += fmt.Sprintf(" DEFAULT %s", *col.DefaultExpr)
	}

	return fmt.Sprintf("-- op: add_column risk:low\n%s;", sql)
}

func (r *Renderer) renderAlterColumnType(op *diff.AlterColumnTypeOp) string {
	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s",
		quoteQualifiedIdentifier(op.Schema, op.Table), quoteIdentifier(op.Column), op.ToType)
	if op.UsingExpr != "" {
		sql += " USING " + op.UsingExpr
	}
	return fmt.Sprintf("-- op: alter_column_type risk:high\n%s;", sql)
}

func (r *Renderer) renderSetNotNull(op *diff.SetNotNullOp) string {
	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL",
		quoteQualifiedIdentifier(op.Schema, op.Table), quoteIdentifier(op.Column))
	return fmt.Sprintf("-- op: set_not_null risk:low\n%s;", sql)
}

func (r *Renderer) renderDropNotNull(op *diff.DropNotNullOp) string {
	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL",
		quoteQualifiedIdentifier(op.Schema, op.Table), quoteIdentifier(op.Column))
	return fmt.Sprintf("-- op: drop_not_null risk:low\n%s;", sql)
}

func (r *Renderer) renderCreateIndex(op *diff.CreateIndexOp) string {
	idx := op.Index
	unique := ""
	if idx.Unique {
		unique = "UNIQUE "
	}
	concurrently := ""
	if idx.Concurrent {
		concurrently = "CONCURRENTLY "
	}
	ifNotExists := ""
	if idx.IfNotExists {
		ifNotExists = "IF NOT EXISTS "
	}
	method := ""
	if idx.Method != "" {
		method = " USING " + idx.Method
	}

	quotedItems := make([]string, 0, len(idx.Elements))
	if len(idx.Elements) > 0 {
		for _, elem := range idx.Elements {
			if s := renderIndexElem(elem); s != "" {
				quotedItems = append(quotedItems, s)
			}
		}
	} else if len(idx.Columns) > 0 {
		for _, c := range idx.Columns {
			quotedItems = append(quotedItems, quoteIdentifier(c))
		}
	}
	items := strings.Join(quotedItems, ", ")

	sql := fmt.Sprintf("CREATE %s%s%sINDEX %s ON %s%s (%s)",
		unique, concurrently, ifNotExists, quoteIdentifier(idx.Name),
		quoteQualifiedIdentifier(op.Schema, idx.Table), method, items)

	if idx.WhereClause != "" {
		sql += " WHERE " + idx.WhereClause
	}
	return fmt.Sprintf("-- op: add_index risk:low\n%s;", sql)
}

func renderIndexElem(elem model.IndexElem) string {
	item := ""
	if elem.Name != "" {
		item = quoteIdentifier(elem.Name)
	} else if elem.Expr != "" {
		item = "(" + elem.Expr + ")"
	}
	if elem.Collation != "" {
		item += " COLLATE " + quoteIdentifier(elem.Collation)
	}
	if elem.Opclass != "" {
		item += " " + elem.Opclass
	}
	if elem.Ordering == "ASC" || elem.Ordering == "DESC" {
		item += " " + elem.Ordering
	}
	if elem.NullsOrdering == "FIRST" || elem.NullsOrdering == "LAST" {
		item += " NULLS " + elem.NullsOrdering
	}
	return item
}

func (r *Renderer) renderDropIndex(op *diff.DropIndexOp) string {
	return fmt.Sprintf("-- op: drop_index risk:medium\nDROP INDEX %s%s;",
		ifExistsPrefix(r.useIfExists), quoteQualifiedIdentifier(op.Schema, op.Name))
}

func (r *Renderer) renderAddEnumType(op *diff.AddEnumTypeOp) string {
	labels := make([]string, len(op.Type.Labels))
	for i, label := range op.Type.Labels {
		labels[i] = quoteString(label)
	}
	sql := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s)",
		quoteQualifiedIdentifier(op.Schema, op.Type.Name), strings.Join(labels, ", "))
	return fmt.Sprintf("-- op: add_enum_type risk:low\n%s;", sql)
}

func (r *Renderer) renderCreateSchema(op *diff.CreateSchemaOp) string {
	return fmt.Sprintf("-- op: create_schema risk:low\nCREATE SCHEMA IF NOT EXISTS %s;",
		quoteIdentifier(op.Schema))
}

func (r *Renderer) renderDropSchema(op *diff.DropSchemaOp) string {
	return fmt.Sprintf("-- op: drop_schema risk:high\nDROP SCHEMA IF EXISTS %s;",
		quoteIdentifier(op.Schema))
}

func (r *Renderer) renderAlterColumnCollation(op *diff.AlterColumnCollationOp) string {
	// PostgreSQL requires SET DATA TYPE even when only the collation changes —
	// there is no standalone ALTER COLUMN ... COLLATE syntax. The COLLATE clause
	// is an optional modifier within SET DATA TYPE. Using the column's current
	// data type (from the op) means the type itself is unchanged but PostgreSQL
	// will still validate the column data against it.
	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DATA TYPE %s",
		quoteQualifiedIdentifier(op.Schema, op.Table),
		quoteIdentifier(op.Column),
		op.DataType,
	)
	if op.ToCollation != "" {
		sql += fmt.Sprintf(" COLLATE %s", quoteIdentifier(op.ToCollation))
	}
	return fmt.Sprintf("-- op: alter_column_collation risk:low\n%s;", sql)
}

func (r *Renderer) renderDropEnumType(op *diff.DropEnumTypeOp) string {
	return fmt.Sprintf("-- op: drop_enum_type risk:high\nDROP TYPE %s%s;",
		ifExistsPrefix(r.useIfExists), quoteQualifiedIdentifier(op.Schema, op.Name))
}

func (r *Renderer) renderCreateView(op *diff.CreateViewOp) string {
	return fmt.Sprintf("-- op: create_view risk:low\nCREATE VIEW %s AS %s;",
		quoteQualifiedIdentifier(op.Schema, op.View.Name), op.View.Definition)
}

func (r *Renderer) renderReplaceView(op *diff.ReplaceViewOp) string {
	return fmt.Sprintf("-- op: replace_view risk:medium\nCREATE OR REPLACE VIEW %s AS %s;",
		quoteQualifiedIdentifier(op.Schema, op.View.Name), op.View.Definition)
}

func (r *Renderer) renderDropView(op *diff.DropViewOp) string {
	return fmt.Sprintf("-- op: drop_view risk:high\nDROP VIEW %s%s;",
		ifExistsPrefix(r.useIfExists), quoteQualifiedIdentifier(op.Schema, op.Name))
}

func (r *Renderer) renderCreateSequence(op *diff.CreateSequenceOp) string {
	seq := op.Sequence
	parts := []string{"CREATE SEQUENCE " + quoteQualifiedIdentifier(op.Schema, seq.Name)}
	if seq.DataType != "" {
		parts = append(parts, "AS "+seq.DataType)
	}
	if seq.StartValue != 0 {
		parts = append(parts, fmt.Sprintf("START WITH %d", seq.StartValue))
	}
	if seq.IncrementBy != 0 {
		parts = append(parts, fmt.Sprintf("INCREMENT BY %d", seq.IncrementBy))
	}
	if seq.MinValue != 0 {
		parts = append(parts, fmt.Sprintf("MINVALUE %d", seq.MinValue))
	}
	if seq.MaxValue != 0 {
		parts = append(parts, fmt.Sprintf("MAXVALUE %d", seq.MaxValue))
	}
	if seq.CacheSize != 0 && seq.CacheSize != 1 {
		parts = append(parts, fmt.Sprintf("CACHE %d", seq.CacheSize))
	}
	if seq.Cycle {
		parts = append(parts, "CYCLE")
	}
	return "-- op: create_sequence risk:low\n" + strings.Join(parts, " ") + ";"
}

func (r *Renderer) renderDropSequence(op *diff.DropSequenceOp) string {
	return fmt.Sprintf("-- op: drop_sequence risk:high\nDROP SEQUENCE %s%s;",
		ifExistsPrefix(r.useIfExists), quoteQualifiedIdentifier(op.Schema, op.Name))
}

func (r *Renderer) renderAlterSequence(op *diff.AlterSequenceOp) string {
	seq := op.To
	parts := []string{"ALTER SEQUENCE " + quoteQualifiedIdentifier(op.Schema, seq.Name)}
	if seq.DataType != "" && seq.DataType != op.From.DataType {
		parts = append(parts, "AS "+seq.DataType)
	}
	if seq.StartValue != 0 && seq.StartValue != op.From.StartValue {
		parts = append(parts, fmt.Sprintf("START WITH %d", seq.StartValue))
	}
	if seq.IncrementBy != 0 && seq.IncrementBy != op.From.IncrementBy {
		parts = append(parts, fmt.Sprintf("INCREMENT BY %d", seq.IncrementBy))
	}
	if seq.MinValue != 0 && seq.MinValue != op.From.MinValue {
		parts = append(parts, fmt.Sprintf("MINVALUE %d", seq.MinValue))
	}
	if seq.MaxValue != 0 && seq.MaxValue != op.From.MaxValue {
		parts = append(parts, fmt.Sprintf("MAXVALUE %d", seq.MaxValue))
	}
	if seq.CacheSize != 0 && seq.CacheSize != op.From.CacheSize {
		parts = append(parts, fmt.Sprintf("CACHE %d", seq.CacheSize))
	}
	if seq.Cycle != op.From.Cycle {
		parts = append(parts, "CYCLE")
	}
	return "-- op: alter_sequence risk:low\n" + strings.Join(parts, " ") + ";"
}

func (r *Renderer) renderCreateExtension(op *diff.CreateExtensionOp) string {
	sql := "CREATE EXTENSION IF NOT EXISTS " + quoteIdentifier(op.Extension.Name)
	if op.Extension.Version != "" {
		sql += " WITH VERSION " + quoteString(op.Extension.Version)
	}
	return "-- op: create_extension risk:low\n" + sql + ";"
}

func (r *Renderer) renderDropExtension(op *diff.DropExtensionOp) string {
	return fmt.Sprintf("-- op: drop_extension risk:high\nDROP EXTENSION %s%s;",
		ifExistsPrefix(r.useIfExists), quoteQualifiedIdentifier(op.Schema, op.Name))
}

func (r *Renderer) renderAlterExtensionUpdate(op *diff.AlterExtensionUpdateOp) string {
	sql := fmt.Sprintf("ALTER EXTENSION %s UPDATE TO %s",
		quoteIdentifier(op.Extension.Name), quoteString(op.Extension.Version))
	return "-- op: alter_extension_update risk:low\n" + sql + ";"
}

// Helper functions

func quoteIdentifierList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = quoteIdentifier(item)
	}
	return strings.Join(quoted, ", ")
}

func renderConstraint(c *model.Constraint) string {
	if c == nil {
		return ""
	}
	definition := renderConstraintDefinition(c)
	if definition == "" {
		return ""
	}
	return fmt.Sprintf("CONSTRAINT %s %s", quoteIdentifier(c.Name), definition)
}

func renderConstraintDefinition(c *model.Constraint) string {
	if c == nil {
		return ""
	}
	switch c.Type {
	case "primary_key":
		return fmt.Sprintf("PRIMARY KEY (%s)", quoteIdentifierList(c.Columns))
	case "foreign_key":
		sql := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s (%s)",
			quoteIdentifierList(c.Columns),
			quoteQualifiedIdentifier(c.RefSchema, c.RefTable),
			quoteIdentifierList(c.RefColumns),
		)
		if c.OnDelete != "" {
			sql += fmt.Sprintf(" ON DELETE %s", c.OnDelete)
		}
		if c.OnUpdate != "" {
			sql += fmt.Sprintf(" ON UPDATE %s", c.OnUpdate)
		}
		return sql
	case "unique":
		return fmt.Sprintf("UNIQUE (%s)", quoteIdentifierList(c.Columns))
	case "check":
		if c.Expression != "" {
			return fmt.Sprintf("CHECK (%s)", c.Expression)
		}
		return ""
	}
	if c.Definition != "" {
		return c.Definition
	}
	return ""
}

func quoteIdentifier(id string) string {
	// Always quote to be safe with reserved words
	// Escape internal double quotes by doubling them (SQL standard)
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

func quoteQualifiedIdentifier(schema, identifier string) string {
	return fmt.Sprintf(`%s.%s`, quoteIdentifier(schema), quoteIdentifier(identifier))
}

func quoteString(s string) string {
	// Escape internal single quotes by doubling them (SQL standard)
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

func ifExistsPrefix(use bool) string {
	if use {
		return "IF EXISTS "
	}
	return ""
}

// RenderJSON renders operations as JSON (for programmatic use)
func RenderJSON(ops []diff.Operation) (string, error) {
	// Convert operations to a JSON-friendly format
	type OpInfo struct {
		Kind        string `json:"kind"`
		ObjectKey   string `json:"object_key"`
		Destructive bool   `json:"destructive"`
		SQL         string `json:"sql"`
	}

	r := NewRenderer()
	infos := make([]OpInfo, len(ops))
	for i, op := range ops {
		obj := op.ObjectKey()
		infos[i] = OpInfo{
			Kind:        string(op.Kind()),
			ObjectKey:   obj.Schema + "." + obj.Name,
			Destructive: op.IsDestructive(),
			SQL:         r.RenderSingle(op),
		}
	}

	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
