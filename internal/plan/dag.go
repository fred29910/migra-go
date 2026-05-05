package plan

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

// Node represents a node in the dependency graph
type Node struct {
	Op           diff.Operation
	Dependencies []*Node // Nodes that this node depends on
	Dependents   []*Node // Nodes that depend on this node
}

// DAG represents a directed acyclic graph of operations
type DAG struct {
	nodes    []*Node
	byObject map[model.ObjectKey][]*Node
}

// NewDAG creates a new DAG
func NewDAG() *DAG {
	return &DAG{
		nodes:    make([]*Node, 0),
		byObject: make(map[model.ObjectKey][]*Node),
	}
}

// AddNode adds an operation to the DAG
func (d *DAG) AddNode(op diff.Operation) *Node {
	node := &Node{
		Op:           op,
		Dependencies: make([]*Node, 0),
		Dependents:   make([]*Node, 0),
	}
	d.nodes = append(d.nodes, node)
	key := op.ObjectKey()
	d.byObject[key] = append(d.byObject[key], node)
	return node
}

// AddDependency adds a dependency: from depends on to
func (d *DAG) AddDependency(from, to *Node) {
	if from == nil || to == nil || from == to {
		return
	}
	for _, dep := range from.Dependencies {
		if dep == to {
			return
		}
	}
	from.Dependencies = append(from.Dependencies, to)
	to.Dependents = append(to.Dependents, from)
}

// BuildDAG builds the dependency graph from operations
func BuildDAG(ops []diff.Operation) *DAG {
	dag := NewDAG()

	// First pass: create all nodes
	for _, op := range ops {
		dag.AddNode(op)
	}

	// Second pass: add dependencies
	for _, node := range dag.nodes {
		dag.addDependencies(node)
	}

	return dag
}

// addDependencies adds dependencies for a node based on operation type
func (d *DAG) addDependencies(node *Node) {
	switch op := node.Op.(type) {
	case *diff.AddColumnOp:
		// AddColumn depends on its table being created first
		tableKey := model.NewObjectKey(op.Schema, op.Table, model.KindTable)
		if tableNode := d.findNodeByOpKind(tableKey, diff.KindAddTable); tableNode != nil {
			d.AddDependency(node, tableNode)
		}

	case *diff.CreateIndexOp:
		// CreateIndex depends on its table being created
		tableKey := model.NewObjectKey(op.Schema, op.Index.Table, model.KindTable)
		if tableNode := d.findNodeByOpKind(tableKey, diff.KindAddTable); tableNode != nil {
			d.AddDependency(node, tableNode)
		}

	case *diff.AlterColumnTypeOp:
		// AlterColumnType depends on the table existing
		tableKey := model.NewObjectKey(op.Schema, op.Table, model.KindTable)
		if tableNode := d.findNodeByOpKind(tableKey, diff.KindAddTable); tableNode != nil {
			d.AddDependency(node, tableNode)
		}

	case *diff.SetNotNullOp:
		tableKey := model.NewObjectKey(op.Schema, op.Table, model.KindTable)
		if tableNode := d.findNodeByOpKind(tableKey, diff.KindAddTable); tableNode != nil {
			d.AddDependency(node, tableNode)
		}

	case *diff.DropNotNullOp:
		tableKey := model.NewObjectKey(op.Schema, op.Table, model.KindTable)
		if tableNode := d.findNodeByOpKind(tableKey, diff.KindAddTable); tableNode != nil {
			d.AddDependency(node, tableNode)
		}

	case *diff.AddEnumTypeOp:
		// Enum types typically don't have dependencies on other operations
		// But columns that use this type depend on the enum type

	case *diff.DropEnumTypeOp:
		// DropEnumType depends on no columns using it
		// TODO: implement reverse dependency check
	}
}

func (d *DAG) findNodeByOpKind(key model.ObjectKey, kind diff.Kind) *Node {
	nodes := d.byObject[key]
	for _, node := range nodes {
		if node.Op.Kind() == kind {
			return node
		}
	}
	return nil
}

// GetExecutionOrder returns operations in topological order
func (d *DAG) GetExecutionOrder() ([]diff.Operation, error) {
	// Kahn's algorithm for topological sort

	// Calculate in-degree for each node
	inDegree := make(map[*Node]int)
	for _, node := range d.nodes {
		inDegree[node] = len(node.Dependencies)
	}

	// Queue for nodes with in-degree 0
	queue := make([]*Node, 0)
	for _, node := range d.nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	result := make([]diff.Operation, 0)

	for len(queue) > 0 {
		// Remove from queue
		node := queue[0]
		queue = queue[1:]
		result = append(result, node.Op)

		// For each dependent, reduce in-degree
		for _, dep := range node.Dependents {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// Check for cycles
	if len(result) != len(d.nodes) {
		return nil, fmt.Errorf("cycle detected in dependency graph")
	}

	return result, nil
}
