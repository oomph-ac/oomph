package player

import (
	"encoding/json"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// SendOomphEventToServer sends data to the server.
func (p *Player) SendOomphEventToServer(event string, data map[string]interface{}) {
	if p.serverConn == nil {
		return
	}

	enc, _ := json.Marshal(data)
	p.serverConn.WritePacket(&packet.ScriptMessage{
		Identifier: event,
		Data:       enc,
	})
}
