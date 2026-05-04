package diff

import (
	"github.com/migra-go/migra-go/internal/model"
)

// diffTables compares tables between two namespaces
func (d *Differ) diffTables(source, target *model.Namespace) {
	// If source is nil, all tables in target are new
	if source == nil {
		for name, table := range target.Tables {
			d.addOp(NewAddTableOp(target.Name, name, table))
		}
		return
	}

	// Find tables to add (in target but not in source)
	for name, table := range target.Tables {
		if _, exists := source.Tables[name]; !exists {
			d.addOp(NewAddTableOp(target.Name, name, table))
		}
	}

	// Find tables to drop (in source but not in target)
	for name := range source.Tables {
		if _, exists := target.Tables[name]; !exists {
			d.addOp(NewDropTableOp(source.Name, name))
		}
	}

	// Compare tables that exist in both
	for name, targetTable := range target.Tables {
		if sourceTable, exists := source.Tables[name]; exists {
			d.diffTableColumns(source.Name, sourceTable, targetTable)
			d.diffTableIndexes(source.Name, sourceTable, targetTable)
		}
	}
}

// diffTableColumns compares columns between two tables
func (d *Differ) diffTableColumns(schema string, source, target *model.Table) {
	// Build maps for quick lookup
	sourceCols := make(map[string]*model.Column)
	for _, col := range source.Columns {
		sourceCols[col.Name] = col
	}

	targetCols := make(map[string]*model.Column)
	for _, col := range target.Columns {
		targetCols[col.Name] = col
	}

	// Find columns to add
	for name, col := range targetCols {
		if _, exists := sourceCols[name]; !exists {
			d.addOp(NewAddColumnOp(schema, target.Name, col))
		}
	}

	// Find columns to drop
	for name := range sourceCols {
		if _, exists := targetCols[name]; !exists {
			// MVP: skip column drops for safety
			_ = name
		}
	}

	// Compare columns that exist in both
	for name, targetCol := range targetCols {
		if sourceCol, exists := sourceCols[name]; exists {
			d.diffColumn(schema, target.Name, sourceCol, targetCol)
		}
	}
}

// diffTableIndexes compares indexes between two tables
func (d *Differ) diffTableIndexes(schema string, source, target *model.Table) {
	// Find indexes to add
	for name, index := range target.Indexes {
		if _, exists := source.Indexes[name]; !exists {
			d.addOp(NewCreateIndexOp(schema, index))
		}
	}

	// Find indexes to drop
	for name := range source.Indexes {
		if _, exists := target.Indexes[name]; !exists {
			d.addOp(NewDropIndexOp(schema, name))
		}
	}
}
