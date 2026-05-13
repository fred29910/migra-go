package parser

import (
	"fmt"
	"testing"
)

func TestDebugParseTarget(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) NOT NULL,
			email VARCHAR(100) DEFAULT 'unknown',
			created_at TIMESTAMP DEFAULT now(),
			age INTEGER
		);
		CREATE TABLE posts (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			title VARCHAR(200) NOT NULL,
			content TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		CREATE INDEX idx_posts_user_id ON posts(user_id);
		CREATE UNIQUE INDEX idx_users_username ON users(username);
		CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest');
		CREATE TABLE comments (
			id SERIAL PRIMARY KEY,
			post_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT now()
		);
	`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ns := schema.Schemas["public"]

	// Check posts table
	posts := ns.Tables["posts"]
	if posts == nil {
		t.Fatal("posts table not found")
	}
	fmt.Printf("=== posts table ===\n")
	fmt.Printf("Columns:\n")
	for _, col := range posts.Columns {
		fmt.Printf("  %s: %s, nullable=%v\n", col.Name, col.DataType, col.IsNullable)
	}
	fmt.Printf("PrimaryKey: %#v\n", posts.PrimaryKey)
	fmt.Printf("Constraints:\n")
	for name, c := range posts.Constraints {
		fmt.Printf("  %s: type=%s, cols=%v, refTable=%s, refSchema=%s, refCols=%v, def=%s\n",
			name, c.Type, c.Columns, c.RefTable, c.RefSchema, c.RefColumns, c.Definition)
	}
	fmt.Printf("Indexes:\n")
	for name, idx := range posts.Indexes {
		fmt.Printf("  %s: cols=%v, unique=%v\n", name, idx.Columns, idx.Unique)
	}

	// Check users table
	users := ns.Tables["users"]
	if users == nil {
		t.Fatal("users table not found")
	}
	fmt.Printf("\n=== users table ===\n")
	fmt.Printf("Columns:\n")
	for _, col := range users.Columns {
		fmt.Printf("  %s: %s, nullable=%v\n", col.Name, col.DataType, col.IsNullable)
	}
	fmt.Printf("PrimaryKey: %#v\n", users.PrimaryKey)
	fmt.Printf("Constraints:\n")
	for name, c := range users.Constraints {
		fmt.Printf("  %s: type=%s, cols=%v, def=%s\n",
			name, c.Type, c.Columns, c.Definition)
	}
}
