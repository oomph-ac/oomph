package command

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var (
	subCmds = make(map[string][]SubCommandsFn)
)

type Permissible interface {
	HasPerm(perm uint64) bool
}

type SubCommandsFn func(p Permissible, cmdPk *packet.AvailableCommands) *protocol.CommandOverload

func RegisterSubCommand(name string, fn SubCommandsFn) {
	arr, ok := subCmds[name]
	if !ok {
		arr = []SubCommandsFn{}
	}
	arr = append(arr, fn)
	subCmds[name] = arr
}

func SubCommands() map[string][]SubCommandsFn {
	return subCmds
}
