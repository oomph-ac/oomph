package player

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func stripAttributeModifiers(attribute protocol.Attribute) protocol.Attribute {
	switch attribute.Name {
	case "minecraft:movement", "minecraft:underwater_movement", "minecraft:lava_movement":
	default:
		return attribute
	}

	if len(attribute.Modifiers) == 0 {
		return attribute
	}

	base := attribute.Default
	for _, modifier := range attribute.Modifiers {
		if modifier.Operation == protocol.AttributeModifierOperationAddition {
			base += modifier.Amount
		}
	}

	value := base
	for _, modifier := range attribute.Modifiers {
		if modifier.Operation == protocol.AttributeModifierOperationMultiplyBase {
			value += base * modifier.Amount
		}
	}
	for _, modifier := range attribute.Modifiers {
		if modifier.Operation == protocol.AttributeModifierOperationMultiplyTotal {
			value *= 1 + modifier.Amount
		}
	}

	attribute.Value = value
	attribute.Default = base
	attribute.Modifiers = nil
	return attribute
}
