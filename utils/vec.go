package utils

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
)

var EmptyVec32 = mgl32.Vec3{}
var EmptyVec64 = mgl64.Vec3{}

func NormalizeVec32(v mgl32.Vec3) mgl32.Vec3 {
	len := v.Len()
	if len < 1e-8 {
		return mgl32.Vec3{}
	}
	l := 1.0 / len
	return mgl32.Vec3{v[0] * l, v[1] * l, v[2] * l}
}
