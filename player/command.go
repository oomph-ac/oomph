package player

import (
	"github.com/oomph-ac/oconfig"
	"github.com/oomph-ac/oomph/player/command"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (p *Player) initOomphCommand(pk *packet.AvailableCommands) {
	// Don't bother registering the command if the player has no permissions.
	if p.perms == 0 {
		return
	}

	overloads := []protocol.CommandOverload{}
	for _, fns := range command.SubCommands() {
		for _, fn := range fns {
			if subCmd := fn(p, pk); subCmd != nil {
				overloads = append(overloads, *subCmd)
			}
		}
	}

	pk.Commands = append(pk.Commands, protocol.Command{
		Name:                     oconfig.Global.CommandName,
		Description:              oconfig.Global.CommandDescription,
		Flags:                    0,
		PermissionLevel:          0,
		AliasesOffset:            ^uint32(0), // MaxUint32 (no aliases)
		ChainedSubcommandOffsets: []uint32{},
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
