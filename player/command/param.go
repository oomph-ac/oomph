package command

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func MakeNormalParam(name string, pType uint32, optional bool) protocol.CommandParameter {
	return protocol.CommandParameter{
		Name:     name,
		Type:     protocol.CommandArgValid | pType,
		Optional: optional,
		Options:  0,
	}
}
