package entity

import (
	"github.com/df-mc/dragonfly/server/entity/physics"
)

type Entity struct {
	Location
	// AABB represents the Axis Aligned Bounding Box of the entity.
	AABB     physics.AABB
	BBWidth  float64
	BBHeight float64
	IsPlayer bool
}
