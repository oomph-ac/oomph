package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

const (
	ClientTypePlayer uint8 = iota
	ClientTypeHealthCheck
)

func init() {
	Register(IDInitConnectionRequest, func() Packet {
		return &InitConnectionRequest{}
	})
}

// InitConnectionRequest is a packet sent by the client to the server to initalize the connection. It allows the server to know the protocol version
// the client is using, which is important for future compatibility (since the server may update its protocol whilst the client has not). Ideally
// this connection initalization is never changed.
type InitConnectionRequest struct {
	ClientType     uint8  // 1 byte
	CloudProtocol  uint32 // 1-5 bytes
	PlayerProtocol int32  // 1-5 bytes

	// ClientData is a JSON byte slice that contains information
	ClientData   []byte // 1-5 bytes + len(ClientData) bytes
	IdentityData []byte // 1-5 bytes + len(IdentityData) bytes
}

func (*InitConnectionRequest) ID() uint32 {
	return IDInitConnectionRequest
}

func (pk *InitConnectionRequest) Marshal(io protocol.IO, _ uint32) {
	io.Uint8(&pk.ClientType)
	io.Varuint32(&pk.CloudProtocol)
	if pk.ClientType == ClientTypePlayer {
		io.Varint32(&pk.PlayerProtocol)
		io.ByteSlice(&pk.ClientData)
		io.ByteSlice(&pk.IdentityData)
	}
}
