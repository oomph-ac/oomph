package player

import (
	"github.com/oomph-ac/oomph/cloud/client"
	"github.com/oomph-ac/oomph/cloud/packet"
	"github.com/oomph-ac/oomph/oerror"
)

func (p *Player) SetCloud(cl *client.Client, cloudId uint64) {
	if p.cloud != nil && cl != nil {
		panic(oerror.New("cloud client already set for player"))
	}
	if !p.opts.Combat.EnableClientEntityTracking {
		panic(oerror.New("cloud functionality requires client-view entity tracking to be enabled"))
	}
	p.cloud = cl
	if cl != nil {
		p.cloudId = cloudId
	} else {
		p.cloudId = 0
	}
}

func (p *Player) Cloud() *client.Client {
	return p.cloud
}

func (p *Player) CloudID() uint64 {
	return p.cloudId
}

func (p *Player) WriteToCloud(pk packet.Packet) {
	cloudClient := p.cloud
	if cloudClient == nil {
		return
	}
	if err := cloudClient.WritePacket(pk); err != nil {
		p.log.Error("failed to write packet to cloud client", "error", err)
	}
}
