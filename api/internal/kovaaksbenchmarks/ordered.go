package kovaaksbenchmarks

import (
	"bytes"
	"encoding/json"
	"sort"
)

type detailOrder struct {
	categories []string
	scenarios  map[string][]string
}

// detailKeyOrder recovers the author's category and scenario order from a
// benchmark detail response. Go maps lose JSON object order, but benchmark
// sheets are meant to be read in the order they were written.
func detailKeyOrder(raw []byte) detailOrder {
	order := detailOrder{scenarios: map[string][]string{}}
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return order
	}
	categoriesRaw, ok := top["categories"]
	if !ok {
		return order
	}
	keys, values := objectEntries(categoriesRaw)
	order.categories = keys
	for i, key := range keys {
		var category map[string]json.RawMessage
		if json.Unmarshal(values[i], &category) != nil {
			continue
		}
		if scenariosRaw, ok := category["scenarios"]; ok {
			order.scenarios[key], _ = objectEntries(scenariosRaw)
		}
	}
	return order
}

// objectEntries returns the keys and raw values of a JSON object in document order.
func objectEntries(raw []byte) ([]string, []json.RawMessage) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, nil
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, nil
	}
	var keys []string
	var values []json.RawMessage
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return keys, values
		}
		key, ok := token.(string)
		if !ok {
			return keys, values
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return keys, values
		}
		keys = append(keys, key)
		values = append(values, value)
	}
	return keys, values
}

// completeOrder keeps the recovered order and appends any keys it missed in
// sorted order, so every map entry appears exactly once.
func completeOrder[T any](order []string, items map[string]T) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, key := range order {
		if _, ok := items[key]; ok && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	var rest []string
	for key := range items {
		if !seen[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}
