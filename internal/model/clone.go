package model

// cloneStringPtr returns a deep copy of a *string.
func cloneStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

// cloneStringsSlice returns a deep copy of a []string.
func cloneStringsSlice(s []string) []string {
	if s == nil {
		return nil
	}
	result := make([]string, len(s))
	copy(result, s)
	return result
}

// ---------------------------------------------------------------------------
// Leaf types — value copy suffices
// ---------------------------------------------------------------------------

// Clone returns a deep copy of the View.
func (v *View) Clone() *View {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

// Clone returns a deep copy of the Sequence.
func (s *Sequence) Clone() *Sequence {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

// Clone returns a deep copy of the Extension.
func (e *Extension) Clone() *Extension {
	if e == nil {
		return nil
	}
	c := *e
	return &c
}

// Clone returns a deep copy of the IndexElem (value type).
func (e IndexElem) Clone() IndexElem {
	return e
}

// ---------------------------------------------------------------------------
// Intermediate types — pointer / slice fields
// ---------------------------------------------------------------------------

// Clone returns a deep copy of the PrimaryKey.
func (pk *PrimaryKey) Clone() *PrimaryKey {
	if pk == nil {
		return nil
	}
	return &PrimaryKey{
		Name:    pk.Name,
		Columns: cloneStringsSlice(pk.Columns),
	}
}

// Clone returns a deep copy of the EnumType.
func (e *EnumType) Clone() *EnumType {
	if e == nil {
		return nil
	}
	return &EnumType{
		Name:   e.Name,
		Labels: cloneStringsSlice(e.Labels),
	}
}

// Clone returns a deep copy of the Column.
func (c *Column) Clone() *Column {
	if c == nil {
		return nil
	}
	return &Column{
		Name:         c.Name,
		DataType:     c.DataType,
		IsNullable:   c.IsNullable,
		DefaultExpr:  cloneStringPtr(c.DefaultExpr),
		IsIdentity:   c.IsIdentity,
		IdentityKind: c.IdentityKind,
		Collation:    c.Collation,
	}
}

// Clone returns a deep copy of the Constraint.
func (c *Constraint) Clone() *Constraint {
	if c == nil {
		return nil
	}
	return &Constraint{
		Name:       c.Name,
		Type:       c.Type,
		Definition: c.Definition,
		Table:      c.Table,
		Columns:    cloneStringsSlice(c.Columns),
		RefSchema:  c.RefSchema,
		RefTable:   c.RefTable,
		RefColumns: cloneStringsSlice(c.RefColumns),
		Expression: c.Expression,
		OnDelete:   c.OnDelete,
		OnUpdate:   c.OnUpdate,
	}
}

// Clone returns a deep copy of the Index.
func (idx *Index) Clone() *Index {
	if idx == nil {
		return nil
	}
	elems := make([]IndexElem, len(idx.Elements))
	copy(elems, idx.Elements)
	return &Index{
		Name:         idx.Name,
		Table:        idx.Table,
		Columns:      cloneStringsSlice(idx.Columns),
		Elements:     elems,
		Unique:       idx.Unique,
		Method:       idx.Method,
		Primary:      idx.Primary,
		IsConstraint: idx.IsConstraint,
		WhereClause:  idx.WhereClause,
		Definition:   idx.Definition,
		Concurrent:   idx.Concurrent,
		IfNotExists:  idx.IfNotExists,
	}
}

// ---------------------------------------------------------------------------
// Root types — map and pointer composition
// ---------------------------------------------------------------------------

// Clone returns a deep copy of the Table.
func (t *Table) Clone() *Table {
	if t == nil {
		return nil
	}
	ct := &Table{
		Schema:        t.Schema,
		Name:          t.Name,
		IsPlaceholder: t.IsPlaceholder,
		Columns:       make([]*Column, len(t.Columns)),
		ColumnByName:  make(map[string]*Column, len(t.ColumnByName)),
		ColumnIndex:   make(map[string]int, len(t.ColumnIndex)),
	}
	for i, col := range t.Columns {
		clonedCol := col.Clone()
		ct.Columns[i] = clonedCol
		ct.ColumnByName[col.Name] = clonedCol
		ct.ColumnIndex[col.Name] = i
	}
	ct.PrimaryKey = t.PrimaryKey.Clone()

	ct.Constraints = make(map[string]*Constraint, len(t.Constraints))
	for name, con := range t.Constraints {
		ct.Constraints[name] = con.Clone()
	}

	ct.Indexes = make(map[string]*Index, len(t.Indexes))
	for name, idx := range t.Indexes {
		ct.Indexes[name] = idx.Clone()
	}
	return ct
}

// Clone returns a deep copy of the Namespace.
func (ns *Namespace) Clone() *Namespace {
	if ns == nil {
		return nil
	}
	c := &Namespace{
		Name:       ns.Name,
		Tables:     make(map[string]*Table, len(ns.Tables)),
		Types:      make(map[string]*EnumType, len(ns.Types)),
		Views:      make(map[string]*View, len(ns.Views)),
		Sequences:  make(map[string]*Sequence, len(ns.Sequences)),
		Extensions: make(map[string]*Extension, len(ns.Extensions)),
	}
	for name, t := range ns.Tables {
		c.Tables[name] = t.Clone()
	}
	for name, et := range ns.Types {
		c.Types[name] = et.Clone()
	}
	for name, v := range ns.Views {
		c.Views[name] = v.Clone()
	}
	for name, s := range ns.Sequences {
		c.Sequences[name] = s.Clone()
	}
	for name, e := range ns.Extensions {
		c.Extensions[name] = e.Clone()
	}
	return c
}

// Clone returns a deep copy of the Schema.
func (s *Schema) Clone() *Schema {
	if s == nil {
		return nil
	}
	c := &Schema{
		Schemas: make(map[string]*Namespace, len(s.Schemas)),
	}
	for name, ns := range s.Schemas {
		c.Schemas[name] = ns.Clone()
	}
	return c
}
