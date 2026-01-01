package utils

import (
	"fmt"
)

func KeyValsToMap(kv []any) (m map[string]any) {
	m = make(map[string]any, len(kv)/2)
	if len(kv) < 2 {
		return
	}

	isKey := true
	var key string
	for _, val := range kv {
		if isKey {
			key = val.(string)
			isKey = false
			continue
		}
		m[key] = val
		isKey = false
	}
	return
}

// KeyValsToString formats slog-style keyvals into a single bracketed string.
// Example: KeyValsToString("foo", 1, "bar", true) => "[foo=1 bar=true]".
// If an odd number of values is provided, the last value is ignored.
func KeyValsToString(kv []any) string {
	if len(kv) < 2 {
		return "[]"
	}
	dataString := "["
	// Only iterate pairs.
	pairCount := len(kv) / 2
	for i := range pairCount {
		if i > 0 {
			dataString += " "
		}
		key := kv[i*2]
		val := kv[i*2+1]
		// Coerce non-string keys to fmt string.
		var keyStr string
		if s, ok := key.(string); ok {
			keyStr = s
		} else {
			keyStr = fmt.Sprintf("%v", key)
		}
		dataString += fmt.Sprintf("%s=%v", keyStr, val)
	}
	dataString += "]"
	return dataString
}
