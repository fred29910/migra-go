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

// Parser parses SQL statements using pg_query_go and builds a Schema model
type Parser struct {
	schema   *model.Schema
	errors   []error
	warnings []string
	sql      string // Original SQL for extracting statement snippets
	applier  *MutationApplier
	registry *HandlerRegistry
	warnFn   WarningEmitter
}

// NewParser creates a new SQL parser with default handler registry.
func NewParser() *Parser {
	return &Parser{
		schema:   model.NewSchema(),
		errors:   make([]error, 0),
		warnings: make([]string, 0),
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
		schema:   model.NewSchema(),
		errors:   make([]error, 0),
		warnings: make([]string, 0),
		applier:  applier,
		registry: registry,
	}
}

// SetWarningEmitter sets the warning emitter callback.
func (p *Parser) SetWarningEmitter(fn WarningEmitter) {
	p.warnFn = fn
}

// Warnings returns parsing warnings (defensive copy).
func (p *Parser) Warnings() []string {
	out := make([]string, len(p.warnings))
	copy(out, p.warnings)
	return out
}

// warnf emits a non-fatal warning.
func (p *Parser) warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	p.warnings = append(p.warnings, msg)
	if p.warnFn != nil {
		p.warnFn("%s", msg)
	}
}

// ParseSQL parses SQL string and returns the schema
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
	// Ensure dependencies are initialized even if created via struct literal
	if p.registry == nil {
		p.registry = DefaultRegistry()
	}
	if p.applier == nil {
		p.applier = &MutationApplier{}
	}

	// Reset parser state for each ParseSQL call
	p.schema = model.NewSchema()
	p.errors = p.errors[:0]
	p.warnings = p.warnings[:0]
	p.sql = sql

	tree, err := pg_query.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("parse SQL: %w", errors.ErrParseFailed)
	}

	for _, rawStmt := range tree.Stmts {
		if err := p.visitNode(rawStmt.Stmt, int(rawStmt.StmtLocation)); err != nil {
			p.errors = append(p.errors, err)
		}
	}

	if len(p.errors) > 0 {
		first := p.errors[0]
		return p.schema, fmt.Errorf("parsing completed with %d errors, first: %w", len(p.errors), first)
	}
	return p.schema, nil
}

// visitNode dispatches the statement node to the registered handler via HandlerRegistry.
// stmt is the inner statement (e.g., CreateStmt) extracted from a RawStmt wrapper.
func (p *Parser) visitNode(stmt *pg_query.Node, pos int) (err error) {
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
			Statement: p.getStatementSnippet(pos),
		}
	}

	mutations, err := handler.Handle(stmt)
	if err != nil {
		return &ParseError{
			Message:   err.Error(),
			Position:  pos,
			Statement: p.getStatementSnippet(pos),
		}
	}

	if err := p.applier.Apply(p.schema, mutations); err != nil {
		return &ParseError{
			Message:   err.Error(),
			Position:  pos,
			Statement: p.getStatementSnippet(pos),
		}
	}
	return nil
}

// getStatementSnippet extracts SQL snippet around position
func (p *Parser) getStatementSnippet(pos int) string {
	if pos < 0 || pos >= len(p.sql) {
		return ""
	}
	end := pos + 100
	if end > len(p.sql) {
		end = len(p.sql)
	}
	return p.sql[pos:end]
}

// Errors returns parsing errors (defensive copy)
func (p *Parser) Errors() []error {
	out := make([]error, len(p.errors))
	copy(out, p.errors)
	return out
}
