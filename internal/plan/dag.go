package plan

import (
	"fmt"

	"github.com/migra-go/migra-go/internal/diff"
	"github.com/migra-go/migra-go/internal/model"
)

// Node represents a node in the dependency graph
type Node struct {
	Op          diff.Operation
	Dependencies []*Node // Nodes that this node depends on
	Dependents   []*Node // Nodes that depend on this node
	inDegree    int    // Number of dependencies (for topological sort)
}

// DAG represents a directed acyclic graph of operations
type DAG struct {
	nodes map[model.ObjectKey]*Node
}

// NewDAG creates a new DAG
func NewDAG() *DAG {
	return &DAG{
		nodes: make(map[model.ObjectKey]*Node),
	}
}

// AddNode adds an operation to the DAG
func (d *DAG) AddNode(op diff.Operation) *Node {
	key := op.ObjectKey()
	if node, exists := d.nodes[key]; exists {
		return node
	}
	node := &Node{
		Op:          op,
		Dependencies: make([]*Node, 0),
		Dependents:   make([]*Node, 0),
	}
	d.nodes[key] = node
	return node
}

// AddDependency adds a dependency: from depends on to
func (d *DAG) AddDependency(from, to *Node) {
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
	for _, op := range ops {
		node := dag.nodes[op.ObjectKey()]
		dag.addDependencies(node, ops)
	}
	
	return dag
}

// addDependencies adds dependencies for a node based on operation type
func (d *DAG) addDependencies(node *Node, allOps []diff.Operation) {
	op := node.Op
	
	switch op.(type) {
	case *diff.AddColumnOp:
		// AddColumn depends on its table being created first
		if addColOp, ok := op.(*diff.AddColumnOp); ok {
			tableKey := model.NewObjectKey(addColOp.Schema, addColOp.Table, model.KindTable)
			if tableNode, exists := d.nodes[tableKey]; exists {
				d.AddDependency(node, tableNode)
			}
		}
		
	case *diff.CreateIndexOp:
		// CreateIndex depends on its table being created
		if createIdxOp, ok := op.(*diff.CreateIndexOp); ok {
			tableKey := model.NewObjectKey("", createIdxOp.Index.Table, model.KindTable)
			if tableNode, exists := d.nodes[tableKey]; exists {
				d.AddDependency(node, tableNode)
			}
		}
		
	case *diff.AlterColumnTypeOp:
		// AlterColumnType depends on the table existing
		if alterOp, ok := op.(*diff.AlterColumnTypeOp); ok {
			tableKey := model.NewObjectKey(alterOp.Schema, alterOp.Table, model.KindTable)
			if tableNode, exists := d.nodes[tableKey]; exists {
				d.AddDependency(node, tableNode)
			}
		}
		
	case *diff.SetNotNullOp, *diff.DropNotNullOp:
		// These depend on the table existing
		// TODO: extract table name from operation
		
	case *diff.AddEnumTypeOp:
		// Enum types typically don't have dependencies on other operations
		// But columns that use this type depend on the enum type
		
	case *diff.DropEnumTypeOp:
		// DropEnumType depends on no columns using it
		// TODO: implement reverse dependency check
	}
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
