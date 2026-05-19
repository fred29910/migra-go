package diff

import (
	"reflect"
	"sort"

	"github.com/fred29910/migra-go/internal/model"
)

// diffTables compares tables between two namespaces
func (c *diffContext) diffTables(source, target *model.Namespace) {
	// If source is nil, all tables in target are new
	if source == nil {
		tableNames := make([]string, 0, len(target.Tables))
		for name := range target.Tables {
			tableNames = append(tableNames, name)
		}
		sort.Strings(tableNames)

		for _, name := range tableNames {
			table := target.Tables[name]
			c.addOp(NewAddTableOp(target.Name, name, table))
		}
		return
	}

	// Find tables to add (in target but not in source) - sorted for deterministic output
	addNames := make([]string, 0, len(target.Tables))
	for name := range target.Tables {
		if _, exists := source.Tables[name]; !exists {
			addNames = append(addNames, name)
		}
	}
	sort.Strings(addNames)
	for _, name := range addNames {
		table := target.Tables[name]
		c.addOp(NewAddTableOp(target.Name, name, table))
	}

	// Find tables to drop (in source but not in target) - sorted for deterministic output
	dropNames := make([]string, 0, len(source.Tables))
	for name := range source.Tables {
		if _, exists := target.Tables[name]; !exists {
			dropNames = append(dropNames, name)
		}
	}
	sort.Strings(dropNames)
	for _, name := range dropNames {
		c.addOp(NewDropTableOp(source.Name, name))
	}

	// Compare tables that exist in both - sorted for deterministic output
	bothNames := make([]string, 0, len(target.Tables))
	for name := range target.Tables {
		if _, exists := source.Tables[name]; exists {
			bothNames = append(bothNames, name)
		}
	}
	sort.Strings(bothNames)
	for _, name := range bothNames {
		targetTable := target.Tables[name]
		sourceTable := source.Tables[name]
		c.diffTableColumns(source.Name, sourceTable, targetTable)
		c.diffTableIndexes(source.Name, sourceTable, targetTable)
		c.diffTableConstraints(source.Name, sourceTable, targetTable)
	}
}

// diffTableConstraints compares constraints between two tables
func (c *diffContext) diffTableConstraints(schema string, source, target *model.Table) {
	addNames := make([]string, 0, len(target.Constraints))
	for name := range target.Constraints {
		if _, exists := source.Constraints[name]; !exists {
			addNames = append(addNames, name)
		}
	}
	sort.Strings(addNames)
	for _, name := range addNames {
		c.addOp(NewAddConstraintOp(schema, target.Name, target.Constraints[name]))
	}

	changeNames := make([]string, 0, len(target.Constraints))
	for name, targetConstraint := range target.Constraints {
		if sourceConstraint, exists := source.Constraints[name]; exists && !sameConstraintContent(sourceConstraint, targetConstraint) {
			// Skip if the constraint is semantically identical (same type and columns).
			// This prevents unnecessary DROP+ADD cycles when the only difference
			// is in derived fields like Definition (from pg_get_constraintdef).
			if sameConstraintSemantics(sourceConstraint, targetConstraint) {
				continue
			}
			changeNames = append(changeNames, name)
		}
	}
	sort.Strings(changeNames)
	for _, name := range changeNames {
		c.addOp(NewDropConstraintOp(schema, source.Name, name))
		c.addOp(NewAddConstraintOp(schema, target.Name, target.Constraints[name]))
	}

	dropNames := make([]string, 0, len(source.Constraints))
	for name := range source.Constraints {
		if _, exists := target.Constraints[name]; !exists {
			dropNames = append(dropNames, name)
		}
	}
	sort.Strings(dropNames)
	for _, name := range dropNames {
		c.addOp(NewDropConstraintOp(schema, source.Name, name))
	}
}

func sameConstraintContent(a, b *model.Constraint) bool {
	if a == nil || b == nil {
		return a == b
	}
	// Note: Definition is excluded from comparison because it's a derived field
	// from pg_get_constraintdef (DB introspect only), not set by SQL file parser.
	// Content equality is determined by structured fields: Type, Columns, Refs, Expression.
	return a.Type == b.Type &&
		reflect.DeepEqual(a.Columns, b.Columns) &&
		a.RefSchema == b.RefSchema &&
		a.RefTable == b.RefTable &&
		reflect.DeepEqual(a.RefColumns, b.RefColumns) &&
		a.Expression == b.Expression &&
		a.OnDelete == b.OnDelete &&
		a.OnUpdate == b.OnUpdate
}

// sameConstraintSemantics checks if two constraints have the same semantic meaning
// (same type and affected columns), ignoring derived/implementation details.
// This is used as a safety net to avoid unnecessary DROP+ADD cycles when
// sameConstraintContent returns false due to Definition or other derived fields.
func sameConstraintSemantics(a, b *model.Constraint) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Type != b.Type {
		return false
	}
	if !reflect.DeepEqual(a.Columns, b.Columns) {
		return false
	}
	// For foreign keys, also check reference target
	if a.Type == "foreign_key" {
		if a.RefSchema != b.RefSchema || a.RefTable != b.RefTable {
			return false
		}
		if !reflect.DeepEqual(a.RefColumns, b.RefColumns) {
			return false
		}
		if a.OnDelete != b.OnDelete || a.OnUpdate != b.OnUpdate {
			return false
		}
	}
	// For check constraints, also check expression
	if a.Type == "check" && a.Expression != b.Expression {
		return false
	}
	return true
}

// diffTableColumns compares columns between two tables
func (c *diffContext) diffTableColumns(schema string, source, target *model.Table) {
	// Use ColumnByName index for quick lookup (avoid building temporary maps)
	// Find columns to add (in target but not in source) - sorted for deterministic output
	addColNames := make([]string, 0, len(target.ColumnByName))
	for name := range target.ColumnByName {
		if _, exists := source.ColumnByName[name]; !exists {
			addColNames = append(addColNames, name)
		}
	}
	sort.Strings(addColNames)
	for _, name := range addColNames {
		col := target.ColumnByName[name]
		c.addOp(NewAddColumnOp(schema, target.Name, col))
	}

	// Find columns to drop (in source but not in target)
	for name := range source.ColumnByName {
		if _, exists := target.ColumnByName[name]; !exists {
			c.addOp(NewDropColumnOp(schema, source.Name, name))
		}
	}

	// Compare columns that exist in both - iterate in target column order for consistency
	for _, targetCol := range target.Columns {
		if sourceCol, exists := source.ColumnByName[targetCol.Name]; exists {
			c.diffColumn(schema, target.Name, sourceCol, targetCol)
		}
	}
}

// diffTableIndexes compares indexes between two tables
func (c *diffContext) diffTableIndexes(schema string, source, target *model.Table) {
	addIdx := make([]string, 0, len(target.Indexes))
	for name := range target.Indexes {
		if _, exists := source.Indexes[name]; !exists {
			addIdx = append(addIdx, name)
		}
	}
	sort.Strings(addIdx)
	for _, name := range addIdx {
		c.addOp(NewCreateIndexOp(schema, target.Indexes[name]))
	}

	dropIdx := make([]string, 0, len(source.Indexes))
	for name := range source.Indexes {
		if _, exists := target.Indexes[name]; !exists {
			dropIdx = append(dropIdx, name)
		}
	}
	sort.Strings(dropIdx)
	for _, name := range dropIdx {
		c.addOp(NewDropIndexOp(schema, name))
	}

	// Compare indexes that exist in both source and target for content changes
	bothIdx := make([]string, 0, len(target.Indexes))
	for name := range target.Indexes {
		if _, exists := source.Indexes[name]; exists {
			bothIdx = append(bothIdx, name)
		}
	}
	sort.Strings(bothIdx)
	for _, name := range bothIdx {
		srcIdx := source.Indexes[name]
		tgtIdx := target.Indexes[name]
		if !sameIndexContent(srcIdx, tgtIdx) {
			c.addOp(NewDropIndexOp(schema, name))
			c.addOp(NewCreateIndexOp(schema, tgtIdx))
		}
	}
}

// sameIndexContent checks if two indexes have the same content
func sameIndexContent(a, b *model.Index) bool {
	if a.Unique != b.Unique {
		return false
	}
	if a.Method != b.Method {
		return false
	}
	if a.WhereClause != b.WhereClause {
		return false
	}
	if len(a.Elements) != len(b.Elements) {
		return false
	}
	for i := range a.Elements {
		if a.Elements[i].Name != b.Elements[i].Name {
			return false
		}
		if a.Elements[i].Expr != b.Elements[i].Expr {
			return false
		}
		if a.Elements[i].Opclass != b.Elements[i].Opclass {
			return false
		}
		if a.Elements[i].Ordering != b.Elements[i].Ordering {
			return false
		}
		if a.Elements[i].NullsOrdering != b.Elements[i].NullsOrdering {
			return false
		}
	}
	return true
}
