package parser

import (
	"reflect"

	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// Handler parses a single pg_query AST node into schema mutations.
type Handler interface {
	Handle(node pg_nodes.Node) ([]SchemaMutation, error)
}

// HandlerRegistry routes AST nodes to their handlers using reflect.Type lookup.
type HandlerRegistry struct {
	handlers map[reflect.Type]Handler
}

// NewHandlerRegistry creates an empty registry.
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{handlers: make(map[reflect.Type]Handler)}
}

// Register associates a handler with a specific AST node type.
func (r *HandlerRegistry) Register(nodeType pg_nodes.Node, h Handler) {
	r.handlers[reflect.TypeOf(nodeType)] = h
}

// Dispatch finds the handler for the given AST node.
func (r *HandlerRegistry) Dispatch(node pg_nodes.Node) (Handler, bool) {
	h, ok := r.handlers[reflect.TypeOf(node)]
	return h, ok
}

// DefaultRegistry returns a registry with all built-in handlers.
func DefaultRegistry() *HandlerRegistry {
	r := NewHandlerRegistry()
	r.Register(pg_nodes.CreateStmt{}, &CreateTableHandler{})
	r.Register(pg_nodes.AlterTableStmt{}, &AlterTableHandler{})
	r.Register(pg_nodes.CreateEnumStmt{}, &CreateEnumHandler{})
	return r
}
