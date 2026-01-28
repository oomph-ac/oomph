package component

import (
	"github.com/oomph-ac/oomph/utils"
)

// BBoxCache returns the bounding box cache for collision optimization.
func (mc *AuthoritativeMovementComponent) BBoxCache() *utils.BBoxCache {
	return mc.bboxCache
}
