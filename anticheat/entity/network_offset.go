package entity

import (
	"github.com/oomph-ac/oomph/anticheat/game"
	"github.com/oomph-ac/oomph/anticheat/utils"
)

// networkOffset ...
func networkOffset(entityType string, metadata map[uint32]any) float32 {
	switch entityType {
	case TypePlayer:
		if utils.HasMetadataFlag[byte](metadata, DataKeyPlayerFlags, DataPlayerFlagSleep) || utils.HasMetadataFlag[int64](metadata, DataKeyFlagsTwo, DataFlagSleeping) {
			return game.SleepingPlayerHeightOffset
		}
		if utils.HasMetadataFlag[int64](metadata, DataKeyFlags, DataFlagSwimming) ||
			utils.HasMetadataFlag[int64](metadata, DataKeyFlags, DataFlagGliding) ||
			utils.HasMetadataFlag[int64](metadata, DataKeyFlags, DataFlagSpinAttack) ||
			utils.HasMetadataFlag[int64](metadata, DataKeyFlagsTwo, DataFlagCrawling) {
			return game.PronePlayerNetworkOffset
		}
		if utils.HasMetadataFlag[int64](metadata, DataKeyFlags, DataFlagSneaking) {
			return game.SneakingPlayerNetworkOffset
		}
		return game.DefaultPlayerHeightOffset
	case TypeItem, TypeFallingBlock, TypeMinecart, TypeChestMinecart, TypeCommandBlockMinecart, TypeHopperMinecart, TypeTNTMinecart:
		return game.ItemAndMinecartNetworkOffset
	case TypeBoat, TypeChestBoat:
		return game.BoatNetworkOffset
	case TypeTNT:
		return game.PrimedTNTNetworkOffset
	default:
		return 0
	}
}
