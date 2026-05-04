package parser

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"github.com/migra-go/migra-go/internal/model"
)

// Parser parses SQL statements and builds a Schema model
type Parser struct {
	schema *model.Schema
	errors []error
}

// NewParser creates a new SQL parser
func NewParser() *Parser {
	return &Parser{
		schema: model.NewSchema(),
		errors: make([]error, 0),
	}
}

// ParseSQL parses SQL string and returns the schema
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
	// Normalize SQL: remove comments and extra whitespace
	sql = p.removeComments(sql)
	
	// Split into statements (by semicolon)
	scanner := bufio.NewScanner(strings.NewReader(sql))
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// Simple semicolon splitting for MVP
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		for i, b := range data {
			if b == ';' {
				return i + 1, data[0:i], nil
			}
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	for scanner.Scan() {
		stmt := strings.TrimSpace(scanner.Text())
		if stmt == "" {
			continue
		}
		if err := p.handleStatement(stmt); err != nil {
			p.errors = append(p.errors, err)
		}
	}

	if len(p.errors) > 0 {
		return p.schema, fmt.Errorf("parsing completed with %d errors", len(p.errors))
	}
	return p.schema, nil
}

// removeComments removes SQL comments (-- and /* */)
func (p *Parser) removeComments(sql string) string {
	// Remove single-line comments
	re := regexp.MustCompile(`--[^\n]*`)
	sql = re.ReplaceAllString(sql, "")
	
	// Remove multi-line comments
	re = regexp.MustCompile(`/\*.*?\*/`)
	sql = re.ReplaceAllString(sql, "")
	
	return strings.TrimSpace(sql)
}

// handleStatement dispatches statement handling
func (p *Parser) handleStatement(stmt string) error {
	upper := strings.ToUpper(strings.TrimSpace(stmt))
	
	if strings.HasPrefix(upper, "CREATE TABLE") {
		return p.handleCreateTable(stmt)
	}
	
	if strings.HasPrefix(upper, "ALTER TABLE") {
		return p.handleAlterTable(stmt)
	}
	
	if strings.HasPrefix(upper, "CREATE INDEX") || strings.HasPrefix(upper, "CREATE UNIQUE INDEX") {
		return p.handleCreateIndex(stmt)
	}
	
	if strings.HasPrefix(upper, "CREATE TYPE") && strings.Contains(upper, "AS ENUM") {
		return p.handleCreateEnum(stmt)
	}
	
	return fmt.Errorf("unsupported statement: %s", stmt[:min(len(stmt), 50)])
}

// handleCreateTable processes CREATE TABLE statements (simplified MVP)
func (p *Parser) handleCreateTable(stmt string) error {
	// Extract table name
	re := regexp.MustCompile(`(?i)CREATE TABLE\s+(?:IF NOT EXISTS\s+)?(?:"?(\w+)"?\.)?"?(\w+)"?\s*\(`)
	matches := re.FindStringSubmatch(stmt)
	if matches == nil {
		return fmt.Errorf("failed to parse CREATE TABLE statement")
	}
	
	schemaName := matches[1]
	tableName := matches[2]
	if schemaName == "" {
		schemaName = "public"
	}
	
	// Get or create namespace
	ns := p.schema.GetOrCreateNamespace(schemaName)
	
	// Create table
	table := model.NewTable(schemaName, tableName)
	
	// Extract column definitions (simplified)
	// Find content between first ( and last )
	start := strings.Index(stmt, "(")
	end := strings.LastIndex(stmt, ")")
	if start == -1 || end == -1 || start >= end {
		ns.Tables[tableName] = table
		return nil
	}
	
	body := stmt[start+1 : end]
	// Split by comma, but not inside parentheses
	parts := p.splitByComma(body)
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Try to parse as column definition
		col := p.parseColumnDef(part)
		if col != nil {
			table.AddColumn(col)
		}
	}
	
	ns.Tables[tableName] = table
	return nil
}

// parseColumnDef parses a column definition string
func (p *Parser) parseColumnDef(def string) *model.Column {
	// Simple column definition: name type [constraints]
	re := regexp.MustCompile(`^"?"?(\w+)"?\s+(\w+(?:\s*\([^)]+\))?)(.*)$`)
	matches := re.FindStringSubmatch(def)
	if matches == nil {
		return nil
	}
	
	col := &model.Column{
		Name:       matches[1],
		DataType:   strings.ToLower(strings.TrimSpace(matches[2])),
		IsNullable: true,
	}
	
	// Check for NOT NULL
	rest := strings.ToUpper(matches[3])
	if strings.Contains(rest, "NOT NULL") {
		col.IsNullable = false
	}
	
	// Check for DEFAULT
	if strings.Contains(rest, "DEFAULT") {
		// Simplified: just mark that there is a default
		// TODO: extract actual default value
	}
	
	return col
}

// handleAlterTable processes ALTER TABLE statements (MVP - basic)
func (p *Parser) handleAlterTable(stmt string) error {
	// Extract table name
	re := regexp.MustCompile(`(?i)ALTER TABLE\s+(?:IF EXISTS\s+)?"?(\w+)"?\s+ADD\s+COLUMN\s+"?(\w+)"?\s+(\w+(?:\s*\([^)]+\))?)`)
	matches := re.FindStringSubmatch(stmt)
	if matches == nil {
		return fmt.Errorf("failed to parse ALTER TABLE statement (MVP only supports ADD COLUMN)")
	}
	
	tableName := matches[1]
	colName := matches[2]
	colType := strings.ToLower(strings.TrimSpace(matches[3]))
	
	// Get namespace
	ns, exists := p.schema.Schemas["public"]
	if !exists {
		return fmt.Errorf("schema 'public' not found")
	}
	
	// Get table
	table, exists := ns.Tables[tableName]
	if !exists {
		return fmt.Errorf("table '%s' not found", tableName)
	}
	
	// Add column
	col := &model.Column{
		Name:       colName,
		DataType:   colType,
		IsNullable: true,
	}
	
	table.AddColumn(col)
	return nil
}

// handleCreateIndex processes CREATE INDEX statements (MVP - basic)
func (p *Parser) handleCreateIndex(stmt string) error {
	// Extract index name, table name, and columns
	re := regexp.MustCompile(`(?i)CREATE\s+(UNIQUE\s+)?INDEX\s+(?:IF NOT EXISTS\s+)?"?(\w+)"?\s+ON\s+"?(\w+)"?\s*\(([^)]+)\)`)
	matches := re.FindStringSubmatch(stmt)
	if matches == nil {
		return fmt.Errorf("failed to parse CREATE INDEX statement")
	}
	
	indexName := matches[2]
	tableName := matches[3]
	columnsStr := matches[4]
	
	// Get namespace (schema)
	ns, exists := p.schema.Schemas["public"]
	if !exists {
		return fmt.Errorf("schema 'public' not found")
	}
	
	// Get table
	table, exists := ns.Tables[tableName]
	if !exists {
		return fmt.Errorf("table '%s' not found", tableName)
	}
	
	// Parse columns
	columns := make([]string, 0)
	colParts := strings.Split(columnsStr, ",")
	for _, col := range colParts {
		col = strings.TrimSpace(col)
		col = strings.Trim(col, "\"")
		columns = append(columns, col)
	}
	
	// Create index
	unique := matches[1] != ""
	index := &model.Index{
		Name:    indexName,
		Table:   tableName,
		Columns: columns,
		Unique:  unique,
		Method:  "btree",
	}
	
	table.Indexes[indexName] = index
	return nil
}

// handleCreateEnum processes CREATE TYPE ... AS ENUM statements
func (p *Parser) handleCreateEnum(stmt string) error {
	// Extract type name and labels
	re := regexp.MustCompile(`(?i)CREATE TYPE\s+(?:IF NOT EXISTS\s+)?"?(\w+)"?\s+AS ENUM\s*\(([^)]+)\)`)
	matches := re.FindStringSubmatch(stmt)
	if matches == nil {
		return fmt.Errorf("failed to parse CREATE TYPE AS ENUM statement")
	}
	
	typeName := matches[1]
	labelsStr := matches[2]
	
	// Get namespace
	ns := p.schema.GetOrCreateNamespace("public")
	
	// Parse labels
	labels := make([]string, 0)
	labelParts := strings.Split(labelsStr, ",")
	for _, label := range labelParts {
		label = strings.TrimSpace(label)
		label = strings.Trim(label, "'\"")
		if label != "" {
			labels = append(labels, label)
		}
	}
	
	// Create enum type
	ns.Types[typeName] = &model.EnumType{
		Name:   typeName,
		Labels: labels,
	}
	
	return nil
}

// splitByComma splits a string by comma, respecting parentheses
func (p *Parser) splitByComma(s string) []string {
	result := make([]string, 0)
	depth := 0
	current := ""
	
	for _, ch := range s {
		switch ch {
		case '(':
			depth++
			current += string(ch)
		case ')':
			depth--
			current += string(ch)
		case ',':
			if depth == 0 {
				result = append(result, strings.TrimSpace(current))
				current = ""
			} else {
				current += string(ch)
			}
		default:
			current += string(ch)
		}
	}
	
	if current != "" {
		result = append(result, strings.TrimSpace(current))
	}
	
	return result
}

// Errors returns parsing errors
func (p *Parser) Errors() []error {
	return p.errors
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
