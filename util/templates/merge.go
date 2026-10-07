package templates

import (
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/evcc-io/evcc/util"
)

// https://github.com/peterbourgon/mergemap

const mergeMaxDepth = 100

var matchKey = strings.EqualFold

// mergeMaps recursively merges other into target, preferring target spelling for duplicate keys.
func mergeMaps(other map[string]any, target map[string]any) error {
	merge(target, other, 0, "")
	return nil
}

func merge(dst, src map[string]any, depth int, path string) map[string]any {
	if depth > mergeMaxDepth {
		panic("too deep!")
	}

	seen := make(map[string]string)
	// Sorting also gives duplicates without an exact target spelling a stable winner.
	for _, sourceKey := range slices.Sorted(maps.Keys(src)) {
		key := sourceKey
		for k := range dst {
			if matchKey(k, key) {
				key = k
				break
			}
		}

		if _, ok := src[key]; ok && key != sourceKey {
			warnDuplicateKey(path, sourceKey, key)
			continue
		}
		if previous, ok := seen[key]; ok {
			warnDuplicateKey(path, sourceKey, previous)
			continue
		}
		seen[key] = sourceKey

		srcVal := src[sourceKey]
		if srcMap, ok := mapify(srcVal); ok {
			dstMap, _ := mapify(dst[key])
			srcVal = merge(dstMap, srcMap, depth+1, path+key+".")
		}
		dst[key] = srcVal
	}
	return dst
}

func warnDuplicateKey(path, ignored, selected string) {
	util.NewLogger("templates").WARN.Printf(
		"duplicate config keys %q and %q: using %q; remove %q from the configuration",
		path+ignored, path+selected, path+selected, path+ignored,
	)
}

func mapify(i any) (map[string]any, bool) {
	value := reflect.ValueOf(i)
	if value.Kind() == reflect.Map {
		m := map[string]any{}
		for _, k := range value.MapKeys() {
			m[k.String()] = value.MapIndex(k).Interface()
		}
		return m, true
	}
	return map[string]any{}, false
}
