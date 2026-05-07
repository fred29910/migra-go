package parser

// MVP 范围声明：
// 本 parser 仅支持以下功能的解析（基于计划 2026-05-04-parser-enhancement-design.md）：
//   - CREATE TABLE（仅提取 ColumnDef，跳过约束）
//   - ALTER TABLE ADD COLUMN（仅处理 AT_AddColumn）
// 不支持的功能（返回 ParseError）：
//   - CREATE INDEX / CREATE UNIQUE INDEX
//   - CREATE TYPE ... AS ENUM
//   - ALTER TABLE 的其他子命令（DROP COLUMN、ALTER COLUMN TYPE 等）
//   - 约束（PRIMARY KEY、FOREIGN KEY、CHECK 等）

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg "github.com/lfittl/pg_query_go"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// ParseError represents a parsing error with position information
type ParseError struct {
	Message   string
	Position  int
	StmtLen   int
	Statement string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at position %d: %s", e.Position, e.Message)
}

// Parser parses SQL statements using pg_query_go and builds a Schema model
type Parser struct {
	schema  *model.Schema
	errors  []error
	sql     string // Original SQL for extracting statement snippets
	applier *MutationApplier
}

// NewParser creates a new SQL parser
func NewParser() *Parser {
	return &Parser{
		schema:  model.NewSchema(),
		errors:  make([]error, 0),
		applier: &MutationApplier{},
	}
}

// ParseSQL parses SQL string and returns the schema
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
	// Reset parser state for each ParseSQL call
	p.schema = model.NewSchema()
	p.errors = p.errors[:0]
	p.sql = sql

	// Use pg_query_go to parse SQL into AST
	tree, err := pg.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("pg_query parse failed: %w", err)
	}

	// Traverse AST statements
	for _, stmt := range tree.Statements {
		if err := p.visitNode(stmt); err != nil {
			p.errors = append(p.errors, err)
		}
	}

	if len(p.errors) > 0 {
		first := p.errors[0]
		return p.schema, fmt.Errorf("parsing completed with %d errors, first: %w", len(p.errors), first)
	}
	return p.schema, nil
}

// visitNode dispatches node to specific handlers using value type assertions
func (p *Parser) visitNode(stmt pg_nodes.Node) error {
	// Statements from pg_query.Parse() are wrapped in RawStmt
	rawStmt, ok := stmt.(pg_nodes.RawStmt)
	if !ok {
		return &ParseError{
			Message:   fmt.Sprintf("expected RawStmt, got: %T", stmt),
			Position:  -1,
			Statement: "",
		}
	}

	// Get actual statement from RawStmt.Stmt
	actualStmt := rawStmt.Stmt
	pos := rawStmt.StmtLocation

	switch n := actualStmt.(type) {
	case pg_nodes.CreateStmt:
		return p.handleCreateTable(n)
	case pg_nodes.AlterTableStmt:
		return p.handleAlterTable(n)
	default:
		return &ParseError{
			Message:   fmt.Sprintf("unsupported statement type: %T", actualStmt),
			Position:  pos,
			Statement: p.getStatementSnippet(pos),
		}
	}
}

// handleCreateTable processes CREATE TABLE statements (MVP: ColumnDef only)
func (p *Parser) handleCreateTable(stmt pg_nodes.CreateStmt) error {
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var columns []model.Column
	for _, item := range stmt.TableElts.Items {
		switch elt := item.(type) {
		case pg_nodes.ColumnDef:
			columns = append(columns, *parserutil.ParseColumnDef(elt))
			// MVP: Skip constraints for now
		}
	}

	mut := CreateTableMutation{
		Schema:  schemaName,
		Name:    tableName,
		Columns: columns,
	}
	return p.applier.Apply(p.schema, []SchemaMutation{mut})
}

// handleAlterTable processes ALTER TABLE statements (MVP: AT_AddColumn only)
func (p *Parser) handleAlterTable(stmt pg_nodes.AlterTableStmt) error {
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	// Get or create namespace and table (handles ALTER TABLE before CREATE TABLE in SQL)
	ns := p.schema.GetOrCreateNamespace(schemaName)

	table, exists := ns.Tables[tableName]
	if !exists {
		table = model.NewTable(schemaName, tableName)
		ns.Tables[tableName] = table
	}

	// Traverse Cmds (sub-commands)
	for _, item := range stmt.Cmds.Items {
		cmd, ok := item.(pg_nodes.AlterTableCmd)
		if !ok {
			continue
		}

		switch cmd.Subtype {
		case pg_nodes.AT_AddColumn:
			if cmd.Def != nil {
				if colDef, ok := cmd.Def.(pg_nodes.ColumnDef); ok {
					col := parserutil.ParseColumnDef(colDef)
					if col != nil {
						table.AddColumn(col)
					}
				}
			}
			// MVP: Skip other alter commands
		}
	}

	return nil
}

// parseRelation extracts table name and schema from RangeVar
func (p *Parser) parseRelation(relation *pg_nodes.RangeVar) (tableName, schemaName string) {
	if relation == nil {
		return "", "public"
	}
	if relation.Relname != nil {
		tableName = *relation.Relname
	}
	if relation.Schemaname != nil {
		schemaName = *relation.Schemaname
	} else {
		schemaName = "public"
	}
	return
}

// parseColumnDef extracts column definition from AST node
func (p *Parser) parseColumnDef(colDef pg_nodes.ColumnDef) *model.Column {
	col := &model.Column{
		IsNullable: true,
	}
	if colDef.Colname != nil {
		col.Name = *colDef.Colname
	}

	// Extract data type
	if colDef.TypeName != nil {
		col.DataType = parserutil.ParseTypeName(*colDef.TypeName)
	}

	// Check constraints (NOT NULL, DEFAULT, etc.)
	for _, item := range colDef.Constraints.Items {
		switch c := item.(type) {
		case pg_nodes.Constraint:
			switch c.Contype {
			case pg_nodes.CONSTR_NOTNULL:
				col.IsNullable = false
			case pg_nodes.CONSTR_DEFAULT:
				// Extract default value expression
				if c.RawExpr != nil {
					expr, ok := parserutil.ParseExpression(c.RawExpr)
					if ok {
						col.DefaultExpr = &expr
					}
				}
			}
		}
	}

	return col
}

// parseTypeName extracts type name from TypeName node
func (p *Parser) parseTypeName(typeName pg_nodes.TypeName) string {
	parts := make([]string, 0)
	for _, item := range typeName.Names.Items {
		if s, ok := item.(pg_nodes.String); ok {
			// Skip pg_catalog schema prefix
			if s.Str != "pg_catalog" {
				parts = append(parts, s.Str)
			}
		}
	}

	typeStr := strings.Join(parts, ".")

	// Map PostgreSQL internal type names to standard names
	typeStr = parserutil.MapTypeName(typeStr)

	// Add type modifiers (e.g., varchar(50))
	if len(typeName.Typmods.Items) > 0 {
		mods := make([]string, 0)
		for _, item := range typeName.Typmods.Items {
			if a, ok := item.(pg_nodes.A_Const); ok {
				if a.Val != nil {
					if i, ok := a.Val.(pg_nodes.Integer); ok {
						mods = append(mods, fmt.Sprintf("%d", i.Ival))
					}
				}
			}
		}
		if len(mods) > 0 {
			typeStr += "(" + strings.Join(mods, ",") + ")"
		}
	}

	return strings.ToLower(typeStr)
}

// typeNameMapping maps PostgreSQL internal type names to standard SQL names
var typeNameMapping = map[string]string{
	"int4":   "integer",
	"int8":   "bigint",
	"float4": "real",
	"float8": "double precision",
}

// mapTypeName maps PostgreSQL internal type names to standard SQL names
func mapTypeName(name string) string {
	if mapped, ok := typeNameMapping[name]; ok {
		return mapped
	}
	return name
}

// parseExpression extracts expression as string (simplified)
// Returns the expression string and a boolean indicating if it was successfully parsed
func (p *Parser) parseExpression(expr pg_nodes.Node) (string, bool) {
	switch e := expr.(type) {
	case pg_nodes.A_Const:
		if e.Val != nil {
			switch v := e.Val.(type) {
			case pg_nodes.String:
				return "'" + v.Str + "'", true
			case pg_nodes.Integer:
				return fmt.Sprintf("%d", v.Ival), true
			case pg_nodes.Float:
				return v.Str, true
			}
		}
	}
	return "", false
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
