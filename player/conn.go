package player

import (
	"io"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type ServerConn interface {
	io.Closer
	WritePacket(pk packet.Packet) error
	GameData() minecraft.GameData
}
