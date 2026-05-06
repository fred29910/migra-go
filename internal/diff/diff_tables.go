package diff

import (
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
			// MVP: skip column drops for safety
			_ = name
			c.warnf("column drop is not implemented yet (ignored): %s.%s.%s", schema, source.Name, name)
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
}
