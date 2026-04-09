package utils

import (
	"fmt"
	"strconv"
	"strings"
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
	var b strings.Builder
	// Rough lower-bound capacity to reduce reallocations on common payloads.
	b.Grow(len(kv) * 8)
	b.WriteByte('[')
	// Only iterate pairs.
	pairCount := len(kv) / 2
	for i := range pairCount {
		if i > 0 {
			b.WriteByte(' ')
		}
		key := kv[i*2]
		val := kv[i*2+1]
		// Coerce non-string keys to fmt string.
		if s, ok := key.(string); ok {
			b.WriteString(s)
		} else {
			b.WriteString(fmt.Sprint(key))
		}
		b.WriteByte('=')
		appendAny(&b, val)
	}
	b.WriteByte(']')
	return b.String()
}

func appendAny(b *strings.Builder, v any) {
	switch t := v.(type) {
	case string:
		b.WriteString(t)
	case bool:
		b.WriteString(strconv.FormatBool(t))
	case int:
		b.WriteString(strconv.Itoa(t))
	case int8:
		b.WriteString(strconv.FormatInt(int64(t), 10))
	case int16:
		b.WriteString(strconv.FormatInt(int64(t), 10))
	case int32:
		b.WriteString(strconv.FormatInt(int64(t), 10))
	case int64:
		b.WriteString(strconv.FormatInt(t, 10))
	case uint:
		b.WriteString(strconv.FormatUint(uint64(t), 10))
	case uint8:
		b.WriteString(strconv.FormatUint(uint64(t), 10))
	case uint16:
		b.WriteString(strconv.FormatUint(uint64(t), 10))
	case uint32:
		b.WriteString(strconv.FormatUint(uint64(t), 10))
	case uint64:
		b.WriteString(strconv.FormatUint(t, 10))
	case float32:
		b.WriteString(strconv.FormatFloat(float64(t), 'f', -1, 32))
	case float64:
		b.WriteString(strconv.FormatFloat(t, 'f', -1, 64))
	default:
		b.WriteString(fmt.Sprint(v))
	}
}
