package parser

// Parser 使用 HandlerRegistry 将 AST 节点路由到对应 Handler，
// Handler 返回 SchemaMutation 列表，由 MutationApplier 统一应用到 model.Schema。
// 新增 DDL 类型只需：实现 Handler + Mutation 类型 + 注册到 DefaultRegistry。

import (
	"fmt"
	"runtime"

	"github.com/fred29910/migra-go/internal/errors"
	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// WarningEmitter is a callback for emitting non-fatal warnings during parsing.
type WarningEmitter func(format string, args ...any)

// ParseError represents a parsing error with position information
type ParseError struct {
	Message   string
	Position  int
	Statement string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at position %d: %s", e.Position, e.Message)
}

// Parser parses SQL statements using pg_query_go and builds a Schema model.
// Parser is stateless: all mutable state is held in local variables within ParseSQL.
type Parser struct {
	applier  *MutationApplier
	registry *HandlerRegistry
	warnFn   WarningEmitter
}

// NewParser creates a new SQL parser with default handler registry.
func NewParser() *Parser {
	return &Parser{
		applier:  &MutationApplier{},
		registry: DefaultRegistry(),
	}
}

// NewParserWith creates a Parser with custom dependencies. Primarily for testing.
func NewParserWith(registry *HandlerRegistry, applier *MutationApplier) *Parser {
	if registry == nil {
		registry = DefaultRegistry()
	}
	if applier == nil {
		applier = &MutationApplier{}
	}
	return &Parser{
		applier:  applier,
		registry: registry,
	}
}

// SetWarningEmitter sets the warning emitter callback.
func (p *Parser) SetWarningEmitter(fn WarningEmitter) {
	p.warnFn = fn
}

// ParseSQL parses SQL string and returns the schema.
// All parsing state is local to this call; the Parser is stateless.
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
	// Ensure dependencies are initialized even if created via struct literal
	if p.registry == nil {
		p.registry = DefaultRegistry()
	}
	if p.applier == nil {
		p.applier = &MutationApplier{}
	}

	schema := model.NewSchema()
	var errs []error
	var warnings []string

	tree, err := pg_query.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("parse SQL: %w, %w", errors.ErrParseFailed, err)
	}

	for _, rawStmt := range tree.Stmts {
		if err := p.visitNode(schema, rawStmt.Stmt, int(rawStmt.StmtLocation), sql); err != nil {
			errs = append(errs, err)
		}
	}

	// Emit warnings via callback
	for _, w := range warnings {
		if p.warnFn != nil {
			p.warnFn(w)
		}
	}

	if len(errs) > 0 {
		first := errs[0]
		return schema, fmt.Errorf("parsing completed with %d errors, first: %w", len(errs), first)
	}
	return schema, nil
}

// visitNode dispatches the statement node to the registered handler via HandlerRegistry.
// stmt is the inner statement (e.g., CreateStmt) extracted from a RawStmt wrapper.
// schema is the Schema being built and sql is the original SQL for snippet extraction.
func (p *Parser) visitNode(schema *model.Schema, stmt *pg_query.Node, pos int, sql string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			err = &ParseError{
				Message:  fmt.Sprintf("recovered from panic: %v\nstack trace:\n%s", r, buf[:n]),
				Position: -1,
			}
		}
	}()

	handler, found := p.registry.Dispatch(stmt)
	if found {
		if we, ok := handler.(interface{ SetWarningEmitter(WarningEmitter) }); ok {
			we.SetWarningEmitter(p.warnFn)
		}
	}
	if !found {
		return &ParseError{
			Message:   fmt.Sprintf("unsupported statement type: %T", stmt),
			Position:  pos,
			Statement: getStatementSnippet(sql, pos),
		}
	}

	mutations, err := handler.Handle(stmt)
	if err != nil {
		return &ParseError{
			Message:   err.Error(),
			Position:  pos,
			Statement: getStatementSnippet(sql, pos),
		}
	}

	if err := p.applier.Apply(schema, mutations); err != nil {
		return &ParseError{
			Message:   err.Error(),
			Position:  pos,
			Statement: getStatementSnippet(sql, pos),
		}
	}
	return nil
}

// getStatementSnippet extracts SQL snippet around position
func getStatementSnippet(sql string, pos int) string {
	if pos < 0 || pos >= len(sql) {
		return ""
	}
	end := pos + 100
	if end > len(sql) {
		end = len(sql)
	}
	return sql[pos:end]
}
