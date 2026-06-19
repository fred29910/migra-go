# PROJECT KNOWLEDGE BASE

**Generated:** 2026-06-19T09:22:59Z
**Commit:** `3bb0062`
**Branch:** `main`

## OVERVIEW

PostgreSQL Schema diff tool in Go — compares database schemas from SQL files, directories, or live PostgreSQL instances. Inspired by Python [migra](https://github.com/djrobstep/migra). Stack: Go 1.26, Cobra+Viper CLI, pgx v5 driver, pg_query_go AST parsing.

## STRUCTURE

```
./
├── cmd/migra/          # CLI entry point (Cobra + Viper)
├── internal/
│   ├── app/            # DI orchestration, push service
│   ├── model/          # Central data model (Schema, Table, Column, etc.)
│   ├── source/         # Schema loading strategy (file/dir/DB)
│   ├── parser/         # SQL DDL parser (pg_query_go + OCP handlers)
│   │   └── parserutil/ # Parser helpers
│   ├── introspect/     # Database introspection (pg_catalog)
│   ├── normalize/      # Semantic type normalization (int4→integer)
│   ├── diff/           # Diff engine: 34 Operation types
│   ├── plan/           # DAG topological sort + 3-phase execution plan
│   ├── render/         # SQL / JSON output renderer
│   ├── errors/         # Structured error types
│   ├── indexdef/       # Index definition parsing
│   ├── util/           # QuoteIdentifier, helpers
│   └── version/        # Build version info
├── testdata/           # Test SQL fixtures (v1-v4, edge cases)
├── docs/               # Architecture, configuration, DDL matrix
├── examples/           # Config samples, .env template
└── scripts/            # setup.sh, release.sh
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| CLI commands/flags | `cmd/migra/` | Cobra commands setup |
| Schema comparison | `internal/diff/` | Core diff engine |
| SQL parsing | `internal/parser/` | DDL → internal model |
| DB introspection | `internal/introspect/` | Live DB schema reading |
| Schema model types | `internal/model/` | Cross-cutting data structures |
| Execution plan | `internal/plan/` | DAG topological ordering |
| SQL output | `internal/render/` | SQL/JSON rendering |
| Schema loading | `internal/source/` | Loader strategy pattern |
| Push/apply changes | `internal/app/push/` | Interactive DB migration |
| Test fixtures | `testdata/diff/` | SQL test cases by scenario |

## CONVENTIONS

- **Package layout**: Standard Go `internal/` monorepo, each package has one responsibility
- **Error handling**: Structured error types in `internal/errors/`; no panics in production paths
- **Testing**: testify suite; table-driven tests common; separate integration test fixtures
- **Parser extensibility**: HandlerRegistry + Mutation pattern for OCP; new DDL = new Handler + Mutation + registry entry
- **Diff operations**: Each operation type implements `DiffOperation` interface with `RenderString`, `IsDestructive`, `DependsOn`
- **Formatting**: `gofmt` enforced in CI; no `.golangci.yml` (relies on go vet + golangci-lint defaults)

## ANTI-PATTERNS (THIS PROJECT)

- **No direct AST type routing**: Always use `HandlerRegistry` with `reflect.Type` — no if/else chains on AST node types
- **No hardcoded type mappings**: Use `normalize` package for type synonyms; `int4` and `integer` must be treated identically
- **No DROP without warning**: All destructive operations require explicit `--unsafe-drop` flag
- **No concurrent DDL**: Push uses a single transaction; `CREATE INDEX CONCURRENTLY` blocked and flagged as non-transactional
- **No skipping handler registration**: Every new handler must be registered in `registry.go` or it won't be found

## COMMANDS

```bash
make build        # Build migra binary
make test         # Run all tests (-v)
make test-coverage # Coverage report
make lint         # golangci-lint
make fmt          # gofmt + go fmt
make vet          # go vet
make ci           # fmt → vet → lint → test
make release-snapshot # goreleaser --snapshot
```

## NOTES

- `pg_query_go` wraps libpg_query C library; cross-compilation requires platform-specific C headers (CI uses `goreleaser-cross` containers)
- Tests use `-short` flag to skip DB-dependent integration tests in CI
- `jj` (Jujutsu) version control in use alongside git
- Testing identity/collation features requires dedicated testdata fixtures under `testdata/diff/`

---
