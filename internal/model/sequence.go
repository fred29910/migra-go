package model

// Sequence represents a PostgreSQL sequence
type Sequence struct {
	Name          string
	DataType      string
	StartValue    int64
	IncrementBy   int64
	MinValue      int64
	MaxValue      int64
	CacheSize     int64
	Cycle         bool
	OwnedByTable  string
	OwnedByColumn string
}
