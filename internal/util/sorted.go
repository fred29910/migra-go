package util

import "sort"

// SortedKeys returns the keys of a string-keyed map in sorted order.
// This is useful for deterministic iteration over maps.
// The map is not modified.
func SortedKeys[V any](m map[string]V) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
