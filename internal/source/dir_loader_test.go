package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryLoader_Match(t *testing.T) {
	loader := &DirectoryLoader{}

	// 创建临时目录
	dir := t.TempDir()

	// 创建临时文件
	file := filepath.Join(dir, "test.sql")
	if err := os.WriteFile(file, []byte("SELECT 1"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		source string
		want   bool
	}{
		{"directory", dir, true},
		{"sql file", file, false},
		{"postgres url", "postgres://localhost/db", false},
		{"file prefix", "file:///tmp/test.sql", false},
		{"non-existent", "/non/existent/path", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := loader.Match(c.source); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.source, got, c.want)
			}
		})
	}
}
