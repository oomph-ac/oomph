package utils

import (
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/item"
	_ "github.com/df-mc/dragonfly/server/session"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func IsItemProjectile(i world.Item) bool {
	switch i.(type) {
	case item.Egg, item.Snowball, item.SplashPotion, item.EnderPearl:
		return true
	}

	return false
}

func ItemName(i world.Item) string {
	if i == nil {
		return ""
	}
	n, _ := i.EncodeItem()
	return n
}

// noinspection ALL
//
//go:linkname ReadItem github.com/df-mc/dragonfly/server/internal/nbtconv.Item
func ReadItem(data map[string]any, s *item.Stack) item.Stack

// noinspection ALL
//
//go:linkname InstanceFromItem github.com/df-mc/dragonfly/server/session.instanceFromItem
func InstanceFromItem(it item.Stack) protocol.ItemInstance

// noinspection ALL
//
//go:linkname StackToItem github.com/df-mc/dragonfly/server/session.stackToItem
func StackToItem(it protocol.ItemStack) item.Stack
