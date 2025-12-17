package command

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func FindOrCreateEnum(pk *packet.AvailableCommands, enumType string, options []string) uint32 {
	// Try to find existing enum by type.
	for i, e := range pk.Enums {
		if e.Type == enumType {
			return uint32(i)
		}
	}
	// Build a fast index for EnumValues to deduplicate options.
	valueIndex := make(map[string]uint32, len(pk.EnumValues))
	for i, v := range pk.EnumValues {
		valueIndex[v] = uint32(i)
	}
	// Map each option to its value index in pk.EnumValues, appending if needed.
	valueIndices := make([]uint32, 0, len(options))
	for _, opt := range options {
		idx, ok := valueIndex[opt]
		if !ok {
			idx = uint32(len(pk.EnumValues))
			pk.EnumValues = append(pk.EnumValues, opt)
			valueIndex[opt] = idx
		}
		valueIndices = append(valueIndices, uint32(idx))
	}
	// Append the enum and return its index.
	pk.Enums = append(pk.Enums, protocol.CommandEnum{Type: enumType, ValueIndices: valueIndices})
	return uint32(len(pk.Enums) - 1)
}

func FindOrCreateDynamicEnum(pk *packet.AvailableCommands, enumType string, options []string) uint32 {
	for i, e := range pk.DynamicEnums {
		if e.Type == enumType {
			// Optionally refresh values; safest to set to latest desired list.
			pk.DynamicEnums[i].Values = options
			return uint32(i)
		}
	}
	pk.DynamicEnums = append(pk.DynamicEnums, protocol.DynamicEnum{Type: enumType, Values: options})
	return uint32(len(pk.DynamicEnums) - 1)
}

func MakeEnumParam(name string, enumIndex uint32, isSoft bool, optional bool) protocol.CommandParameter {
	var t uint32 = protocol.CommandArgValid
	if isSoft {
		t |= protocol.CommandArgSoftEnum | enumIndex
	} else {
		t |= protocol.CommandArgEnum | enumIndex
	}
	return protocol.CommandParameter{
		Name:     name,
		Type:     t,
		Optional: optional,
		Options:  0,
	}
}
