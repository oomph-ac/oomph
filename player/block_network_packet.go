package player

import (
	"github.com/oomph-ac/oomph/world/blocknetwork"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const fallingBlockEntityType = "minecraft:falling_block"

func translateBlockNetworkID(id uint32, translator blocknetwork.Translator) (uint32, bool) {
	translated := translator.Translate(id)
	return translated, translated != id
}

func rewriteBlockNetworkID(id *uint32, translator blocknetwork.Translator) bool {
	translated, modified := translateBlockNetworkID(*id, translator)
	if modified {
		*id = translated
	}
	return modified
}

func rewriteStackBlockNetworkID(stack *protocol.ItemStack, translator blocknetwork.Translator) bool {
	if stack.BlockRuntimeID == 0 {
		return false
	}
	translated, modified := translateBlockNetworkID(uint32(stack.BlockRuntimeID), translator)
	if modified {
		stack.BlockRuntimeID = int32(translated)
	}
	return modified
}

func rewriteStacksBlockNetworkIDs(stacks []protocol.ItemStack, translator blocknetwork.Translator) ([]protocol.ItemStack, bool) {
	var rewritten []protocol.ItemStack
	for i := range stacks {
		stack := stacks[i]
		if !rewriteStackBlockNetworkID(&stack, translator) {
			continue
		}
		if rewritten == nil {
			rewritten = append([]protocol.ItemStack(nil), stacks...)
		}
		rewritten[i] = stack
	}
	return rewritten, rewritten != nil
}

func rewriteShapedRecipeBlockNetworkIDs(recipe protocol.ShapedRecipe, translator blocknetwork.Translator) (protocol.ShapedRecipe, bool) {
	output, modified := rewriteStacksBlockNetworkIDs(recipe.Output, translator)
	if modified {
		recipe.Output = output
	}
	return recipe, modified
}

func rewriteShapelessRecipeBlockNetworkIDs(recipe protocol.ShapelessRecipe, translator blocknetwork.Translator) (protocol.ShapelessRecipe, bool) {
	output, modified := rewriteStacksBlockNetworkIDs(recipe.Output, translator)
	if modified {
		recipe.Output = output
	}
	return recipe, modified
}

func rewriteRecipeBlockNetworkIDs(recipe protocol.Recipe, translator blocknetwork.Translator) (protocol.Recipe, bool) {
	switch recipe := recipe.(type) {
	case *protocol.ShapedRecipe:
		clientRecipe, modified := rewriteShapedRecipeBlockNetworkIDs(*recipe, translator)
		if modified {
			return &clientRecipe, true
		}
	case *protocol.ShulkerBoxRecipe:
		output, modified := rewriteShapelessRecipeBlockNetworkIDs(recipe.ShapelessRecipe, translator)
		if modified {
			clientRecipe := *recipe
			clientRecipe.ShapelessRecipe = output
			return &clientRecipe, true
		}
	case *protocol.ShapelessChemistryRecipe:
		output, modified := rewriteShapelessRecipeBlockNetworkIDs(recipe.ShapelessRecipe, translator)
		if modified {
			clientRecipe := *recipe
			clientRecipe.ShapelessRecipe = output
			return &clientRecipe, true
		}
	case *protocol.ShapedChemistryRecipe:
		output, modified := rewriteShapedRecipeBlockNetworkIDs(recipe.ShapedRecipe, translator)
		if modified {
			clientRecipe := *recipe
			clientRecipe.ShapedRecipe = output
			return &clientRecipe, true
		}
	case *protocol.ShapelessRecipe:
		clientRecipe, modified := rewriteShapelessRecipeBlockNetworkIDs(*recipe, translator)
		if modified {
			return &clientRecipe, true
		}
	case *protocol.SmithingTransformRecipe:
		clientRecipe := *recipe
		if rewriteStackBlockNetworkID(&clientRecipe.Result, translator) {
			return &clientRecipe, true
		}
	}
	return recipe, false
}

func rewriteLevelEventBlockNetworkID(pk *packet.LevelEvent, translator blocknetwork.Translator) bool {
	switch pk.EventType {
	case packet.LevelEventParticlesDestroyBlock, packet.LevelEventParticlesDestroyBlockNoSound:
		translated, modified := translateBlockNetworkID(uint32(pk.EventData), translator)
		if modified {
			pk.EventData = int32(translated)
		}
		return modified
	default:
		return false
	}
}

func rewriteLevelSoundBlockNetworkID(pk *packet.LevelSoundEvent, translator blocknetwork.Translator) bool {
	if !levelSoundUsesBlockNetworkID(pk.SoundType) {
		return false
	}
	translated, modified := translateBlockNetworkID(uint32(pk.ExtraData), translator)
	if modified {
		pk.ExtraData = int32(translated)
	}
	return modified
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

func rewriteActorBlockMetadata(metadata map[uint32]any, translator blocknetwork.Translator, fallingBlock bool) (map[uint32]any, bool) {
	keys := []uint32{protocol.EntityDataKeyDisplayTileRuntimeID, protocol.EntityDataKeyCarryBlockRuntimeID}
	if fallingBlock {
		keys = append(keys, protocol.EntityDataKeyVariant)
	}
	var clientMetadata map[uint32]any
	for _, key := range keys {
		value, ok := metadata[key].(int32)
		if !ok {
			continue
		}
		translated := int32(translator.Translate(uint32(value)))
		if translated == value {
			continue
		}
		if clientMetadata == nil {
			clientMetadata = make(map[uint32]any, len(metadata))
			for key, value := range metadata {
				clientMetadata[key] = value
			}
		}
		clientMetadata[key] = translated
	}
	if clientMetadata == nil {
		return metadata, false
	}
	return clientMetadata, true
}
