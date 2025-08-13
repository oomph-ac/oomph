package player

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// GODBLESS THE CLANKERS (gpt-5-high) FOR ONE-SHOTTING THIS!!!
func (p *Player) initOomphCommand(pk *packet.AvailableCommands) {
	// Don't bother registering the command if the player has no permissions.
	if p.perms == 0 {
		return
	}

	// Helper to lookup or create a static enum and return its index in pk.Enums.
	findOrCreateEnum := func(enumType string, options []string) uint32 {
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
		valueIndices := make([]uint, 0, len(options))
		for _, opt := range options {
			idx, ok := valueIndex[opt]
			if !ok {
				idx = uint32(len(pk.EnumValues))
				pk.EnumValues = append(pk.EnumValues, opt)
				valueIndex[opt] = idx
			}
			valueIndices = append(valueIndices, uint(idx))
		}
		// Append the enum and return its index.
		pk.Enums = append(pk.Enums, protocol.CommandEnum{Type: enumType, ValueIndices: valueIndices})
		return uint32(len(pk.Enums) - 1)
	}

	// Helper to lookup or create a dynamic enum and return its index in pk.DynamicEnums.
	findOrCreateDynamicEnum := func(enumType string, options []string) uint32 {
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

	// Sub-commands we support.
	// We use single-option enums for the first token to model proper sub-command flows like Dragonfly does for cmd.SubCommand.
	alertsEnumIdx := findOrCreateEnum("oomph:alerts", []string{"alerts"})
	logsEnumIdx := findOrCreateEnum("oomph:logs", []string{"logs"})
	debugEnumIdx := findOrCreateEnum("oomph:debug", []string{"debug"})
	enableEnumIdx := findOrCreateEnum("enabled", []string{"true", "false", "enable", "disable"})

	debugModes := append(DebugModeList, "type_message", "type_log")
	debugModesEnumIdx := findOrCreateDynamicEnum("oomph:debug_modes", debugModes)

	// Construct the AC command with three overloads:
	// 1) ac alerts <enabled: bool>
	// 2) ac logs <player:target>
	// 3) ac debug <mode: string>
	// For each sub-command we use a single-value enum as the first parameter, matching how cmd.SubCommand is encoded.
	mkEnumParam := func(name string, enumIndex uint32, isSoft bool, optional bool) protocol.CommandParameter {
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

	overloads := []protocol.CommandOverload{}
	if p.HasPerm(PermissionAlerts) {
		overloads = append(overloads, protocol.CommandOverload{Parameters: []protocol.CommandParameter{
			mkEnumParam("alerts", alertsEnumIdx, false, false),
			protocol.CommandParameter{
				Name:     "enable_alerts",
				Type:     protocol.CommandArgValid | protocol.CommandArgEnum | enableEnumIdx,
				Optional: false,
			},
		}})
		overloads = append(overloads, protocol.CommandOverload{Parameters: []protocol.CommandParameter{
			mkEnumParam("alerts", alertsEnumIdx, false, false),
			mkNormalParam("delayMs", protocol.CommandArgTypeInt, false),
		}})
	}
	if p.HasPerm(PermissionLogs) {
		overloads = append(overloads, protocol.CommandOverload{Parameters: []protocol.CommandParameter{
			mkEnumParam("logs", logsEnumIdx, false, false),
			mkNormalParam("player", protocol.CommandArgTypeTarget, false),
		}})
	}
	if p.HasPerm(PermissionDebug) {
		overloads = append(overloads, protocol.CommandOverload{Parameters: []protocol.CommandParameter{
			mkEnumParam("debug", debugEnumIdx, false, false),
			mkEnumParam("mode", debugModesEnumIdx, true, true),
		}})
	}

	pk.Commands = append(pk.Commands, protocol.Command{
		Name:                     "ac",
		Description:              "Command for anti-cheat functionality",
		Flags:                    0,
		PermissionLevel:          0,
		AliasesOffset:            ^uint32(0), // MaxUint32 (no aliases)
		ChainedSubcommandOffsets: []uint16{},
		Overloads:                overloads,
	})
}

func mkNormalParam(name string, pType uint32, optional bool) protocol.CommandParameter {
	return protocol.CommandParameter{
		Name:     name,
		Type:     protocol.CommandArgValid | pType,
		Optional: optional,
		Options:  0,
	}
}
