package packet

import (
	"fmt"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const (
	CurrentProtocol uint32 = 100

	NetworkHeaderSize = 5
)

var (
	pool = make(map[uint32]func() Packet)
)

type Packet interface {
	ID() uint32
	Marshal(io protocol.IO, cloudProto uint32)
}

func Get(id uint32) (Packet, bool) {
	if pkFn, ok := pool[id]; ok {
		return pkFn(), true
	}
	return nil, false
}

func Register(id uint32, fn func() Packet) {
	if _, exists := pool[id]; exists {
		panic(fmt.Errorf("packet with ID %d already registered", id))
	}
	pool[id] = fn
}
