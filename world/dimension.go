package world

import (
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
)

var (
	Overworld overworld
)

type overworld struct{}

func (overworld) Range() cube.Range                 { return cube.Range{-64, 319} }
func (overworld) WaterEvaporates() bool             { return false }
func (overworld) LavaSpreadDuration() time.Duration { return time.Second * 3 / 2 }
func (overworld) WeatherCycle() bool                { return false }
func (overworld) TimeCycle() bool                   { return false }
func (overworld) String() string                    { return "Overworld" }
