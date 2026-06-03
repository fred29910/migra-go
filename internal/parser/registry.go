package parser

import (
	"reflect"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// Handler parses a single pg_query AST node into schema mutations.
type Handler interface {
	Handle(node *pg_query.Node) ([]SchemaMutation, error)
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
// nodeType must have a non-nil .Node oneof field (e.g., &pg_query.Node{Node: &pg_query.Node_CreateStmt{}}).
func (r *HandlerRegistry) Register(nodeType *pg_query.Node, h Handler) {
	r.handlers[reflect.TypeOf(nodeType.Node)] = h
}

// Dispatch finds the handler for the given AST node.
// It uses the protobuf oneof type (via GetNode()) as the lookup key.
func (r *HandlerRegistry) Dispatch(node *pg_query.Node) (Handler, bool) {
	if node == nil || node.GetNode() == nil {
		return nil, false
	}
	h, ok := r.handlers[reflect.TypeOf(node.GetNode())]
	return h, ok
}

// DefaultRegistry returns a registry with all built-in handlers.
func DefaultRegistry() *HandlerRegistry {
	r := NewHandlerRegistry()
	r.Register(&pg_query.Node{Node: &pg_query.Node_CreateStmt{}}, &CreateTableHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_AlterTableStmt{}}, &AlterTableHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_CreateEnumStmt{}}, &CreateEnumHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_IndexStmt{}}, &CreateIndexHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_CreateSchemaStmt{}}, &CreateSchemaHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_RenameStmt{}}, &RenameStmtHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_ViewStmt{}}, &CreateViewHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_CreateTableAsStmt{}}, &CreateMaterializedViewHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_CreateSeqStmt{}}, &CreateSequenceHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_CreateExtensionStmt{}}, &CreateExtensionHandler{})
	return r
}
