package diff

import (
	"sort"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

// diffTables compares tables between two namespaces
func (c *diffContext) diffTables(source, target *model.Namespace) {
	if source == nil {
		tableNames := make([]string, 0, len(target.Tables))
		for name := range target.Tables {
			tableNames = append(tableNames, name)
		}
		sort.Strings(tableNames)
		for _, name := range tableNames {
			c.addOp(NewAddTableOp(target.Name, name, target.Tables[name]))
		}
		return
	}

	if target == nil {
		return
	}

	// Find tables to add (in target but not in source)
	addNames := make([]string, 0, len(target.Tables))
	for name := range target.Tables {
		if _, exists := source.Tables[name]; !exists {
			addNames = append(addNames, name)
		}
	}
	sort.Strings(addNames)
	for _, name := range addNames {
		c.addOp(NewAddTableOp(target.Name, name, target.Tables[name]))
	}

	// Find tables to drop (in source but not in target)
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

	// Compare tables that exist in both
	bothNames := make([]string, 0, len(target.Tables))
	for name := range target.Tables {
		if _, exists := source.Tables[name]; exists {
			bothNames = append(bothNames, name)
		}
	}
	sort.Strings(bothNames)
	for _, name := range bothNames {
		c.diffTableColumns(source.Name, source.Tables[name], target.Tables[name])
		c.diffTableIndexes(source.Name, source.Tables[name], target.Tables[name])
		c.diffTableConstraints(source.Name, source.Tables[name], target.Tables[name])
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
	return a.Type == b.Type &&
		util.SameStringSlice(a.Columns, b.Columns) &&
		a.RefSchema == b.RefSchema &&
		a.RefTable == b.RefTable &&
		util.SameStringSlice(a.RefColumns, b.RefColumns) &&
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
	if !util.SameStringSlice(a.Columns, b.Columns) {
		return false
	}
	if a.Type == "foreign_key" {
		if a.RefSchema != b.RefSchema || a.RefTable != b.RefTable {
			return false
		}
		if !util.SameStringSlice(a.RefColumns, b.RefColumns) {
			return false
		}
		if a.OnDelete != b.OnDelete || a.OnUpdate != b.OnUpdate {
			return false
		}
	}
	if a.Type == "check" && a.Expression != b.Expression {
		return false
	}
	return true
}

// isColumnRenameCandidate checks if a source column and target column match
// the heuristic for being the same column that was renamed.
// This is a best-effort heuristic: it compares structural properties that
// typically stay the same after a rename. Matches on DataType, IsNullable,
// DefaultExpr, and Collation.
//
// NOTE: This heuristic CAN produce false positives — for example, if a user
// drops a column with properties X and adds a new column with the same
// properties (but different semantics), it will be detected as a rename.
// Future iterations could reduce false positives by:
//   - allowing users to explicitly declare renames via SQL comment hints
//   - considering ordinal_position proximity
//   - using pg_attribute.attnum stability across introspect snapshots
func isColumnRenameCandidate(src, tgt *model.Column) bool {
	if src.DataType != tgt.DataType {
		return false
	}
	if src.IsNullable != tgt.IsNullable {
		return false
	}
	if !sameDefault(src.DefaultExpr, tgt.DefaultExpr) {
		return false
	}
	if src.Collation != tgt.Collation {
		return false
	}
	return true
}

// diffTableColumns compares columns between two tables
func (c *diffContext) diffTableColumns(schema string, source, target *model.Table) {
	sourceOnlyNames := make([]string, 0, len(source.ColumnByName))
	for name := range source.ColumnByName {
		if _, exists := target.ColumnByName[name]; !exists {
			sourceOnlyNames = append(sourceOnlyNames, name)
		}
	}
	sort.Strings(sourceOnlyNames)

	targetOnlyNames := make([]string, 0, len(target.ColumnByName))
	for name := range target.ColumnByName {
		if _, exists := source.ColumnByName[name]; !exists {
			targetOnlyNames = append(targetOnlyNames, name)
		}
	}
	sort.Strings(targetOnlyNames)

	renamedSource := make(map[string]bool)
	renamedTarget := make(map[string]bool)

	// Build a map from column signature to target column name for O(n) lookup.
	// Signature = dataType + isNullable + defaultExpr + collation
	targetBySig := make(map[string]string, len(targetOnlyNames))
	for _, tgtName := range targetOnlyNames {
		tgtCol := target.ColumnByName[tgtName]
		sig := columnSignature(tgtCol)
		targetBySig[sig] = tgtName
	}

	for _, srcName := range sourceOnlyNames {
		srcCol := source.ColumnByName[srcName]
		sig := columnSignature(srcCol)
		if tgtName, found := targetBySig[sig]; found && !renamedTarget[tgtName] {
			c.addOp(NewRenameColumnOp(schema, target.Name, srcName, tgtName))
			renamedSource[srcName] = true
			renamedTarget[tgtName] = true
		}
	}

	// Phase 2: Add columns that are truly new (not rename targets)
	addColNames := make([]string, 0, len(targetOnlyNames))
	for _, name := range targetOnlyNames {
		if !renamedTarget[name] {
			addColNames = append(addColNames, name)
		}
	}
	sort.Strings(addColNames)
	for _, name := range addColNames {
		col := target.ColumnByName[name]
		c.addOp(NewAddColumnOp(schema, target.Name, col))
	}

	// Phase 3: Drop columns that are truly removed (not rename sources)
	for name := range source.ColumnByName {
		if _, exists := target.ColumnByName[name]; !exists && !renamedSource[name] {
			c.addOp(NewDropColumnOp(schema, source.Name, name))
		}
	}

	// Phase 4: Compare columns that exist in both (unchanged)
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

// columnSignature returns a string that uniquely identifies a column's
// structural properties used for rename detection.
func columnSignature(col *model.Column) string {
	var b strings.Builder
	b.WriteString(col.DataType)
	b.WriteByte('|')
	if col.IsNullable {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteByte('|')
	if col.DefaultExpr != nil {
		b.WriteString(*col.DefaultExpr)
	}
	b.WriteByte('|')
	b.WriteString(col.Collation)
	return b.String()
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
		if a.Elements[i].IndexColName != b.Elements[i].IndexColName {
			return false
		}
		if a.Elements[i].Collation != b.Elements[i].Collation {
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
