package player

import (
	"github.com/oomph-ac/oomph/world/blocknetwork"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const fallingBlockEntityType = "minecraft:falling_block"

func rewriteLevelEventBlockNetworkID(pk *packet.LevelEvent, translator blocknetwork.Translator) bool {
	switch pk.EventType {
	case packet.LevelEventParticlesDestroyBlock, packet.LevelEventParticlesDestroyBlockNoSound:
		pk.EventData = int32(translator.Translate(uint32(pk.EventData)))
		return true
	default:
		return false
	}
}

func rewriteLevelSoundBlockNetworkID(pk *packet.LevelSoundEvent, translator blocknetwork.Translator) bool {
	if !levelSoundUsesBlockNetworkID(pk.SoundType) {
		return false
	}
	pk.ExtraData = int32(translator.Translate(uint32(pk.ExtraData)))
	return true
}

func levelSoundUsesBlockNetworkID(soundType string) bool {
	switch soundType {
	case packet.SoundEventItemUseOn,
		packet.SoundEventHit,
		packet.SoundEventStep,
		packet.SoundEventBreak,
		packet.SoundEventPlace,
		packet.SoundEventLand,
		packet.SoundEventPressurePlateClickOff,
		packet.SoundEventPressurePlateClickOn,
		packet.SoundEventDoorOpen,
		packet.SoundEventDoorClose,
		packet.SoundEventTrapdoorOpen,
		packet.SoundEventTrapdoorClose,
		packet.SoundEventFenceGateOpen,
		packet.SoundEventFenceGateClose:
		return true
	default:
		return false
	}
}

func rewriteFallingBlockMetadata(metadata map[uint32]any, translator blocknetwork.Translator) (map[uint32]any, bool) {
	value, ok := metadata[protocol.EntityDataKeyVariant].(int32)
	if !ok {
		return metadata, false
	}
	clientMetadata := make(map[uint32]any, len(metadata))
	for key, value := range metadata {
		clientMetadata[key] = value
	}
	clientMetadata[protocol.EntityDataKeyVariant] = int32(translator.Translate(uint32(value)))
	return clientMetadata, true
}
