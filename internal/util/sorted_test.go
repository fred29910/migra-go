package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortedKeys(t *testing.T) {
	m := map[string]int{"c": 3, "a": 1, "b": 2}
	assert.Equal(t, []string{"a", "b", "c"}, SortedKeys(m))
}

func TestSortedKeys_Single(t *testing.T) {
	assert.Equal(t, []string{"z"}, SortedKeys(map[string]int{"z": 1}))
}

func TestSortedKeys_Empty(t *testing.T) {
	assert.Nil(t, SortedKeys(map[string]int{}))
}

func TestSortedKeys_Nil(t *testing.T) {
	assert.Nil(t, SortedKeys[int](nil))
}
