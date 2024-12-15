package main

import (
	"log/slog"

	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/util"
	"github.com/oomph-ac/oomph"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := slog.Default()
	oomphLog := logrus.New()
	oomphLog.SetLevel(logrus.DebugLevel)
	opts := util.DefaultOpts()
	opts.ClientDecode = []uint32{
		packet.IDScriptMessage,
		packet.IDText,
		packet.IDPlayerAuthInput,
		packet.IDNetworkStackLatency,
		packet.IDRequestChunkRadius,
		packet.IDInventoryTransaction,
		packet.IDMobEquipment,
		packet.IDAnimate,
	}
	proxy := spectrum.NewSpectrum(server.NewStaticDiscovery("127.0.0.1:19133", ""), logger, opts, nil)
	if err := proxy.Listen(minecraft.ListenConfig{StatusProvider: util.NewStatusProvider("Spectrum Proxy", "Spectrum")}); err != nil {
		return
	}

	for {
		s, err := proxy.Accept()
		if err != nil {
			continue
		}
		s.SetProcessor(oomph.NewProcessor(s, proxy.Registry(), proxy.Listener(), oomphLog))
	}
}
