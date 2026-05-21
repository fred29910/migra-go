package model

// View represents a PostgreSQL view
type View struct {
	Name         string
	Definition   string
	Materialized bool
}
