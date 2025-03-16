package utils

import (
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/item"
)

// noinspection ALL
//
//go:linkname ReadItem github.com/df-mc/dragonfly/server/internal/nbtconv.Item
func ReadItem(data map[string]any, s *item.Stack) item.Stack
