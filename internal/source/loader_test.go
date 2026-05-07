package source

import (
	"testing"
)

func TestDBLoader_Match(t *testing.T) {
	loader := &DBLoader{}
	tests := []struct {
		source   string
		expected bool
	}{
		{"postgres://user:pass@localhost/db", true},
		{"POSTGRES://user:pass@localhost/db", true},
		{"mysql://user:pass@localhost/db", true},
		{"MYSQL://user:pass@localhost/db", true},
		{"file:///path/to/file.sql", false},
		{"/path/to/file.sql", false},
	}
	for _, tt := range tests {
		got := loader.Match(tt.source)
		if got != tt.expected {
			t.Errorf("DBLoader.Match(%q) = %v, want %v", tt.source, got, tt.expected)
		}
	}
}

func TestSQLFileLoader_Match(t *testing.T) {
	loader := &SQLFileLoader{}
	tests := []struct {
		source   string
		expected bool
	}{
		{"file:///path/to/file.sql", true},
		{"FILE:///path/to/file.sql", true},
		{"/path/to/file.sql", true},
		{"/path/to/file.SQL", true},
		{"postgres://user:pass@localhost/db", false},
	}
	for _, tt := range tests {
		got := loader.Match(tt.source)
		if got != tt.expected {
			t.Errorf("SQLFileLoader.Match(%q) = %v, want %v", tt.source, got, tt.expected)
		}
	}
}
