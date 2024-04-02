package detection

import (
	"fmt"

	"github.com/elliotchance/orderedmap/v2"
)

// OrderedMapToString converts an orderedmap to a string.
func OrderedMapToString(data orderedmap.OrderedMap[string, any]) string {
	dataString := "["
	count := data.Len()
	for _, key := range data.Keys() {
		v, _ := data.Get(key)
		dataString += fmt.Sprintf("%s=%v", key, v)

		count--
		if count > 0 {
			dataString += " "
		}
	}
	dataString += "]"

	return dataString
}
