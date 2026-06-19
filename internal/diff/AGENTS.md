# DIFF — Schema Comparison Engine

**Part of migra-go** — PostgreSQL schema diff tool.

## OVERVIEW

Compares two `SchemaModel` instances → list of `DiffOperation` objects. Core engine of the tool. 34 operation types across 15 source files.

## FILES

| File | Purpose |
|------|---------|
| `differ.go` | Entry: `Differ.Compare()` |
| `context.go` | `DiffContext` — schema filters, options |
| `diff_tables.go` | Table-level diff (columns, rename candidates) |
| `diff_columns.go` | Column-level diff logic |
| `operation.go` | `DiffOperation` interface, `Kind` constants |
| `operation_table.go` | AddTableOp, DropTableOp, RenameTableOp |
| `operation_column.go` | AddColumnOp, DropColumnOp, AlterColumnTypeOp, SetNotNullOp, DropNotNullOp, SetDefaultOp, DropDefaultOp, RenameColumnOp, AddIdentityOp, SetIdentityOp, DropIdentityOp, AlterColumnCollationOp |
| `operation_index.go` | AddIndexOp, DropIndexOp |
| `operation_constraint.go` | AddConstraintOp, DropConstraintOp |
| `operation_enum.go` | AddEnumTypeOp, DropEnumTypeOp, AddEnumLabelOp |
| `operation_view.go` | CreateViewOp, DropViewOp, ReplaceViewOp, CreateMaterializedViewOp, DropMaterializedViewOp |
| `operation_schema.go` | CreateSchemaOp, DropSchemaOp |
| `operation_sequence.go` | CreateSequenceOp, DropSequenceOp, AlterSequenceOp |
| `operation_extension.go` | CreateExtensionOp, DropExtensionOp, AlterExtensionUpdateOp |
| `rename_column_op.go` | Rename detection via content heuristics |

## CONVENTIONS

- Every op implements `DiffOperation` (`RenderString`, `IsDestructive`, `DependsOn`)
- Register new `Kind` in `operation.go`
- Rename: table diff detects add+drop pairs → heuristics match by type/default/constraint
- Column ops reference parent table + schema for DAG dependencies

## ANTI-PATTERNS

- Never add an op type without registering its `Kind` constant
- Never skip rename-candidate check when table diff finds add+drop pairs

## WHERE TO LOOK

| Task | File |
|------|------|
| Compare two schemas | `differ.go` |
| Column diff logic | `diff_columns.go` |
| New column operation | `operation_column.go` |
| Rename detection | `rename_column_op.go` |
| Operation kind list | `operation.go` |
