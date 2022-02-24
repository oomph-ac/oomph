package utils

import (
	"fmt"
	"strings"
)

// PrettyParameters converts the given parameters to a readable string.
func PrettyParameters(params map[string]interface{}) string {
	if len(params) == 0 {
		// Don't waste time if there aren't any parameters.
		return "[]"
	}
	// Hacky but simple way to create a readable string.
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(fmt.Sprint(params), "map"), " ", ", "), ":", "=")
}
