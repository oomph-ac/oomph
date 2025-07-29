package player

import "github.com/oomph-ac/oomph/cloud/packet"

func (p *Player) WriteToCloud(pk packet.Packet) {
	cloudClient := p.cloudClient
	if cloudClient == nil {
		return
	}
	if err := cloudClient.WritePacket(pk); err != nil {
		p.log.Error("failed to write packet to cloud client", "error", err)
	}
}
