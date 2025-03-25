package player

import (
	"encoding/json"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
)

type ReplayHeader struct {
	ShieldID int32
	Protocol int32

	ClientData   []byte
	IdentityData []byte
}

func NewReplayHeader(
	shieldID int32,
	protocol int32,
	clientData login.ClientData,
	identityData login.IdentityData,
) *ReplayHeader {
	cDat, _ := json.Marshal(clientData)
	iDat, _ := json.Marshal(identityData)
	return &ReplayHeader{
		ShieldID:     shieldID,
		Protocol:     protocol,
		ClientData:   cDat,
		IdentityData: iDat,
	}
}

func (r *ReplayHeader) Marshal(io protocol.IO) {
	io.Int32(&r.ShieldID)
	io.Int32(&r.Protocol)
	io.ByteSlice(&r.ClientData)
	io.ByteSlice(&r.IdentityData)
}
