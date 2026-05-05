package diff

import (
	"sort"

	"github.com/fred29910/migra-go/internal/model"
)

// diffTables compares tables between two namespaces
func (d *Differ) diffTables(source, target *model.Namespace) {
	// If source is nil, all tables in target are new
	if source == nil {
		tableNames := make([]string, 0, len(target.Tables))
		for name := range target.Tables {
			tableNames = append(tableNames, name)
		}
		sort.Strings(tableNames)

		for _, name := range tableNames {
			table := target.Tables[name]
			d.addOp(NewAddTableOp(target.Name, name, table))
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
		d.addOp(NewAddTableOp(target.Name, name, table))
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
		d.addOp(NewDropTableOp(source.Name, name))
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
		d.diffTableColumns(source.Name, sourceTable, targetTable)
		d.diffTableIndexes(source.Name, sourceTable, targetTable)
	}
}

// diffTableColumns compares columns between two tables
func (d *Differ) diffTableColumns(schema string, source, target *model.Table) {
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
		d.addOp(NewAddColumnOp(schema, target.Name, col))
	}

	// Find columns to drop (in source but not in target)
	for name := range source.ColumnByName {
		if _, exists := target.ColumnByName[name]; !exists {
			// MVP: skip column drops for safety
			_ = name
			d.warnf("column drop is not implemented yet (ignored): %s.%s.%s", schema, source.Name, name)
		}
	}

	// Compare columns that exist in both - iterate in target column order for consistency
	for _, targetCol := range target.Columns {
		if sourceCol, exists := source.ColumnByName[targetCol.Name]; exists {
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
