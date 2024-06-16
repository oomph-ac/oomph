package utils

import (
	"github.com/df-mc/dragonfly/server/block/cube"
)

// GetFaceFromRotation returns the a block face given the yaw of the player.
func GetFaceFromRotation(yaw float32) cube.Face {
	if yaw <= -135 || yaw > 135 {
		return cube.FaceNorth
	} else if yaw <= -45 {
		return cube.FaceEast
	} else if yaw <= 45 {
		return cube.FaceSouth
	} else {
		return cube.FaceWest
	}
}
