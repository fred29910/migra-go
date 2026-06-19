# PARSER — SQL DDL to SchemaModel

**Part of migra-go** — PostgreSQL schema diff tool.

## OVERVIEW

Converts PostgreSQL DDL statements (via `pg_query_go` AST) into SchemaModel mutations. HandlerRegistry + OCP pattern.

## FILES

| File | Purpose |
|------|---------|
| `parser.go` | `Parse()`, `ParseDDL()` entry points |
| `registry.go` | HandlerRegistry — `reflect.Type` → Handler |
| `mutation.go` | Mutation interface + MutationApplier |
| `applier.go` | Applies mutations to SchemaModel |
| `create_table_handler.go` | CREATE TABLE → CreateTableMutation |
| `alter_table_handler.go` | ALTER TABLE (ADD/DROP columns, constraints) |
| `index_handler.go` | CREATE/DROP INDEX |
| `index_mutation.go` | Index creation mutation |
| `enum_handler.go` | CREATE TYPE AS ENUM |
| `schema_handler.go` | CREATE SCHEMA |
| `view_handler.go` | CREATE/DROP VIEW |
| `sequence_handler.go` | CREATE/DROP/ALTER SEQUENCE |
| `extension_handler.go` | CREATE/DROP/ALTER EXTENSION |
| `rename_column_mutation.go` | Rename column mutation |
| `rename_stmt_handler.go` | ALTER TABLE RENAME handler |
| `parserutil/` | Shared helpers |

## CONVENTIONS

- AST type routing via `HandlerRegistry` + `reflect.Type` — no if/else chains
- Each DDL type: one Handler implementing `Handler` interface
- Each Handler produces one typed Mutation
- All handlers registered in `registry.go` — MUST register or won't be found
- Sequence options: parsed by shared `applySequenceOptions()`

## ANTI-PATTERNS

- No direct type switches on AST nodes — always use HandlerRegistry
- No skipping handler registration (runtime won't find it)
- No hardcoded type names — normalize in `internal/normalize/`

## WHERE TO LOOK

| Task | File |
|------|------|
| Add new DDL support | New Handler + Mutation + `registry.go` entry |
| Parser entry | `parser.go` |
| Handler registration | `registry.go` |
| Mutation types | `mutation.go` |
