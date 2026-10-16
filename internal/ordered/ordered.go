// Package ordered recovers the document order of the keys of a YAML mapping
// decoded into a Go map.
package ordered

import (
	"fmt"
	"reflect"
	"sort"

	"gopkg.in/yaml.v2"
)

// Keys returns the keys of m, a map with string keys decoded from the
// top-level YAML mapping in data, in the order in which they appear in data.
//
// The values are still decoded into m by yaml.v2 as before, data is read
// again only to recover the order. A key that appears more than once takes
// the position of its first occurrence (yaml.v2 keeps its last value, like a
// PHP array would). Keys that cannot be matched with the document, if any,
// follow in lexical order, so the result is always deterministic.
func Keys(data []byte, m interface{}) ([]string, error) {
	mv := reflect.ValueOf(m)
	if mv.Kind() != reflect.Map || mv.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("ordered: %T is not a map with string keys", m)
	}
	var items yaml.MapSlice
	if err := yaml.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	keys := make([]string, 0, mv.Len())
	used := make(map[string]bool, mv.Len())
	for _, item := range items {
		k, ok := item.Key.(string)
		if !ok {
			k = fmt.Sprint(item.Key)
		}
		if used[k] || !mv.MapIndex(reflect.ValueOf(k).Convert(mv.Type().Key())).IsValid() {
			continue
		}
		used[k] = true
		keys = append(keys, k)
	}
	if len(keys) < mv.Len() {
		var rest []string
		for _, k := range mv.MapKeys() {
			if !used[k.String()] {
				rest = append(rest, k.String())
			}
		}
		sort.Strings(rest)
		keys = append(keys, rest...)
	}
	return keys, nil
}
