package plan

import (
	"context"
	"fmt"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

// Node represents a node in the dependency graph
type Node struct {
	Op            diff.Operation
	Dependencies  []*Node // Nodes that this node depends on
	Dependents    []*Node // Nodes that depend on this node
	DependencySet map[*Node]struct{}
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
		Op:            op,
		Dependencies:  make([]*Node, 0),
		Dependents:    make([]*Node, 0),
		DependencySet: make(map[*Node]struct{}),
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
	if _, exists := from.DependencySet[to]; exists {
		return
	}
	from.DependencySet[to] = struct{}{}
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
	for _, depKey := range node.Op.DependsOn() {
		if depNode := d.findNodeByObjectKey(depKey); depNode != nil {
			d.AddDependency(node, depNode)
		}
	}
}

// findNodeByObjectKey finds the best matching node for an object key.
// When multiple nodes share the same key (e.g., DropConstraint + AddConstraint
// for the same constraint), DropConstraint is preferred so that
// AddConstraint.DependsOn resolves to the DropConstraint node, not itself.
func (d *DAG) findNodeByObjectKey(key model.ObjectKey) *Node {
	matches := d.byObject[key]
	if len(matches) == 0 {
		return nil
	}
	for _, n := range matches {
		if n.Op.Kind() == diff.KindDropConstraint {
			return n
		}
	}
	return matches[0]
}

// GetExecutionOrder returns operations in topological order
func (d *DAG) GetExecutionOrder(ctx context.Context) ([]diff.Operation, error) {
	// Kahn's algorithm for topological sort

	// Calculate in-degree for each node
	inDegree := make(map[*Node]int)
	for _, node := range d.nodes {
		inDegree[node] = len(node.Dependencies)
	}

	// Queue for nodes with in-degree 0
	queue := make([]*Node, 0, len(d.nodes))
	for _, node := range d.nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	result := make([]diff.Operation, 0, len(d.nodes))

	head := 0
	for head < len(queue) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		node := queue[head]
		head++
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
