package world

import (
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/ethaniccc/float32-cube/cube"
)

func FromDragonflyPos(pos df_cube.Pos) cube.Pos {
	return cube.Pos{
		pos.X(),
		pos.Y(),
		pos.Z(),
	}
}

func ToDragonflyPos(pos cube.Pos) df_cube.Pos {
	return df_cube.Pos{
		pos.X(),
		pos.Y(),
		pos.Z(),
	}
}
