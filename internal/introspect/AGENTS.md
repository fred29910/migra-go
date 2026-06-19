# INTROSPECT — Live Database Introspection

**Part of migra-go** — PostgreSQL schema diff tool.

## OVERVIEW

Reads live PostgreSQL schema via `pg_catalog` queries and builds a `SchemaModel`. 8 source files.

## FILES

| File | Purpose |
|------|---------|
| `introspect.go` | Entry point, schema loading orchestration |
| `tables.go` | Table/column metadata (pg_catalog.pg_class, pg_attribute) |
| `constraints.go` | PK, FK, unique, check constraints |
| `indexes.go` | Index definitions |
| `enums.go` | Enum types |
| `extensions.go` | Extension listing |
| `sequences.go` | Sequence metadata |
| `views.go` | View + materialized view definitions |

## CONVENTIONS

- All queries via `pg_catalog` (not information_schema) for full PG-specific features
- Uses `pgx v5` for DB connectivity
- Schema filter applied to all queries
- Query order respects dependency order (tables → columns → constraints → indexes)
- Each file's queries are independent → parallel-safe

## ANTI-PATTERNS

- Never use information_schema for features only in pg_catalog (identity, collations)
- Never load without schema filter (can OOM on large DBs)
