# MODEL — Central Data Structures

**Part of migra-go** — PostgreSQL schema diff tool.

## OVERVIEW

Core types (Schema, Table, Column, View, Sequence, etc.) used by every package in the pipeline. Cross-cutting data model.

## TYPES

| Type | File | Key Fields |
|------|------|------------|
| `Schema` | `schema.go` | Tables, Views, Sequences, Enums, Extensions, Schemas maps |
| `Table` | `table.go` | Name, Columns, PrimaryKey, Indexes, Constraints |
| `Column` | `column.go` | Name, Type, NotNull, Default, Identity, Collation |
| `View` | `view.go` | Name, Definition, IsMaterialized |
| `Sequence` | `sequence.go` | Name, Start, Increment, Min/Max, Cycle |
| `Extension` | `extension.go` | Name, Version |
| `ObjectKey` | `object_key.go` | SchemaName + ObjectName; implements Comparable |
| `IndexElem` | `index_elem.go` | Column, Expression, Order |
| `FKAction` | `fk_action.go` | OnDelete, OnUpdate enums |
| `CloneResult` | `clone.go` | Deep-copy helpers for safe mutation |

## CONVENTIONS

- All types: plain structs (no methods beyond Clone, Comparable)
- `ObjectKey` = map key for cross-schema references
- `Column.Type` stores *normalized* type name (post-normalize pass)
- `Schema` = root container holding all object maps
- Views + materialized views share same `View` struct (flag: `IsMaterialized`)

## ANTI-PATTERNS

- Never store raw/non-normalized types in `Column.Type`
- Never use string concat for qualified names — use `ObjectKey`
- Never mutate `Schema` in-place without `Clone()`

## WHERE TO LOOK

| Task | File |
|------|------|
| Schema structure | `schema.go` |
| Column definition | `column.go` |
| Cross-schema keys | `object_key.go` |
| Deep copy | `clone.go` |
