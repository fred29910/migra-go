package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/migra-go/migra-go/internal/model"
	"github.com/stretchr/testify/require"
)

// LoadSchemaFromJSON loads a schema from a JSON file for testing
func LoadSchemaFromJSON(t *testing.T, path string) *model.Schema {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	schema := &model.Schema{}
	err = json.Unmarshal(data, schema)
	require.NoError(t, err)
	return schema
}

// SaveSchemaToJSON saves a schema to a JSON file for golden testing
func SaveSchemaToJSON(t *testing.T, schema *model.Schema, path string) {
	t.Helper()
	data, err := json.MarshalIndent(schema, "", "  ")
	require.NoError(t, err)

	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(path, data, 0644)
	require.NoError(t, err)
}

// CompareJSON compares two JSON strings ignoring formatting
func CompareJSON(t *testing.T, expected, actual string) {
	t.Helper()
	var exp, act interface{}
	err := json.Unmarshal([]byte(expected), &exp)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(actual), &act)
	require.NoError(t, err)

	expJSON, err := json.Marshal(exp)
	require.NoError(t, err)
	actJSON, err := json.Marshal(act)
	require.NoError(t, err)

	require.JSONEq(t, string(expJSON), string(actJSON))
}

// GoldenFile performs a golden file test
// expectedPath: path to the expected output file
// actual: the actual output to compare
// updateFlag: if true, update the golden file
func GoldenFile(t *testing.T, expectedPath string, actual string, updateFlag bool) {
	t.Helper()

	if updateFlag {
		err := os.MkdirAll(filepath.Dir(expectedPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(expectedPath, []byte(actual), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	require.Equal(t, string(expected), actual)
}
