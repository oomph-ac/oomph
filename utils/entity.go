package utils

func GetValueFromStringMap[T any](list map[string]any, key string, value *T) bool {
	if v, ok := list[key]; ok {
		if finalVal, ok := v.(T); ok {
			*value = finalVal
			return true
		}
	}
	return false
}
