package player

import (
	"github.com/oomph-ac/oomph/cloud/client"
	"github.com/oomph-ac/oomph/cloud/packet"
	"github.com/oomph-ac/oomph/oerror"
)

func (p *Player) SetCloudClient(cl *client.Client) {
	if p.cloudClient != nil && cl != nil {
		panic(oerror.New("cloud client already set for player"))
	}
	if !p.opts.Combat.EnableClientEntityTracking {
		panic(oerror.New("cloud requires client-view entity tracking to be enabled"))
	}
	p.cloudClient = cl
}

func (p *Player) CloudClient() *client.Client {
	return p.cloudClient
}

func (p *Player) WriteToCloud(pk packet.Packet) {
	cloudClient := p.cloudClient
	if cloudClient == nil {
		return
	}
	if err := cloudClient.WritePacket(pk); err != nil {
		p.log.Error("failed to write packet to cloud client", "error", err)
	}
}
