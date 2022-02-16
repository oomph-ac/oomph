package utils

import (
	"fmt"
	"strings"
)

// PrettyParams converts the given parameters to a readable string.
func PrettyParams(params map[string]interface{}) string {
	if len(params) == 0 {
		// Don't waste our time if there are no parameters.
		return "[]"
	}
	// Hacky but simple way to create a readable string.
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(fmt.Sprint(params), "map"), " ", ", "), ":", "=")
}
