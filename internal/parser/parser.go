package parser

// Parser 使用 HandlerRegistry 将 AST 节点路由到对应 Handler，
// Handler 返回 SchemaMutation 列表，由 MutationApplier 统一应用到 model.Schema。
// 新增 DDL 类型只需：实现 Handler + Mutation 类型 + 注册到 DefaultRegistry。

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

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
	sql      string // Original SQL for extracting statement snippets
	applier  *MutationApplier
	registry *HandlerRegistry
}

// NewParser creates a new SQL parser with default handler registry.
func NewParser() *Parser {
	return &Parser{
		schema:   model.NewSchema(),
		errors:   make([]error, 0),
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
		applier:  applier,
		registry: registry,
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
	p.sql = sql

	tree, err := pg_query.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("pg_query parse failed: %w", err)
	}

	for _, rawStmt := range tree.Stmts {
		if err := p.visitNode(rawStmt.Stmt); err != nil {
			p.errors = append(p.errors, err)
		}
	}

	if len(p.errors) > 0 {
		first := p.errors[0]
		return p.schema, fmt.Errorf("parsing completed with %d errors, first: %w", len(p.errors), first)
	}
	return p.schema, nil
}

// visitNode dispatches node to the registered handler via HandlerRegistry.
func (p *Parser) visitNode(stmt *pg_query.Node) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ParseError{
				Message:  fmt.Sprintf("recovered from panic: %v", r),
				Position: -1,
			}
		}
	}()

	rawStmt := stmt.GetRawStmt()
	if rawStmt == nil {
		return &ParseError{
			Message:  fmt.Sprintf("expected RawStmt, got: %T", stmt),
			Position: -1,
		}
	}

	actualStmt := rawStmt.Stmt
	pos := int(rawStmt.StmtLocation)

	handler, found := p.registry.Dispatch(actualStmt)
	if !found {
		return &ParseError{
			Message:   fmt.Sprintf("unsupported statement type: %T", actualStmt),
			Position:  pos,
			Statement: p.getStatementSnippet(pos),
		}
	}

	mutations, err := handler.Handle(actualStmt)
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
