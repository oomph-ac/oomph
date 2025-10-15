package utils

import (
	"fmt"
)

// KeyValsToString formats slog-style keyvals into a single bracketed string.
// Example: KeyValsToString("foo", 1, "bar", true) => "[foo=1 bar=true]".
// If an odd number of values is provided, the last value is ignored.
func KeyValsToString(kv ...any) string {
	if len(kv) < 2 {
		return "[]"
	}
	dataString := "["
	// Only iterate pairs.
	pairCount := len(kv) / 2
	for i := 0; i < pairCount; i++ {
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
