package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"time"

	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session"
	"github.com/cooldogedev/spectrum/transport"
	"github.com/cooldogedev/spectrum/util"
	"github.com/go-echarts/statsview"
	"github.com/go-echarts/statsview/viewer"
	v589 "github.com/oomph-ac/multiversion/multiversion/protocols/1_20/v589"
	v594 "github.com/oomph-ac/multiversion/multiversion/protocols/1_20/v594"
	v618 "github.com/oomph-ac/multiversion/multiversion/protocols/1_20/v618"
	v622 "github.com/oomph-ac/multiversion/multiversion/protocols/1_20/v622"
	v630 "github.com/oomph-ac/multiversion/multiversion/protocols/1_20/v630"
	v649 "github.com/oomph-ac/multiversion/multiversion/protocols/1_20/v649"
	v662 "github.com/oomph-ac/multiversion/multiversion/protocols/1_20/v662"
	v671 "github.com/oomph-ac/multiversion/multiversion/protocols/1_20/v671"
	v686 "github.com/oomph-ac/multiversion/multiversion/protocols/1_21/v686"
	v712 "github.com/oomph-ac/multiversion/multiversion/protocols/1_21/v712"
	v729 "github.com/oomph-ac/multiversion/multiversion/protocols/1_21/v729"
	v748 "github.com/oomph-ac/multiversion/multiversion/protocols/1_21/v748"
	v766 "github.com/oomph-ac/multiversion/multiversion/protocols/1_21/v766"
	v776 "github.com/oomph-ac/multiversion/multiversion/protocols/1_21/v776"
	"github.com/oomph-ac/oconfig"
	"github.com/oomph-ac/oomph"
	"github.com/oomph-ac/oomph/player"
	otransport "github.com/oomph-ac/oomph/transport"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"

	_ "net/http/pprof"
)

func main() {
	logger := slog.Default()
	oomphLog := logrus.New()
	oomphLog.SetLevel(logrus.DebugLevel)

	if len(os.Args) < 3 {
		oomphLog.Fatal("Usage: ./oomph-bin <local_port> <remote_addr> <optional: spectrum_token>")
		return
	}

	if os.Getenv("PPROF_ENABLED") != "" {
		// set configurations before calling `statsview.New()` method
		viewer.SetConfiguration(viewer.WithTheme(viewer.ThemeWesteros), viewer.WithAddr("192.168.1.172:8080"))

		mgr := statsview.New()
		go mgr.Start()
		//go http.ListenAndServe("localhost:8080", nil)
	}

	debug.SetGCPercent(-1)
	debug.SetMemoryLimit(4 * 1024 * 1024 * 1024) // 4GB

	opts := util.DefaultOpts()
	opts.ClientDecode = player.ClientDecode
	opts.AutoLogin = false
	opts.Addr = ":" + os.Args[1]
	opts.SyncProtocol = false
	opts.LatencyInterval = int64(time.Second)

	if len(os.Args) >= 4 {
		opts.Token = os.Args[3]
	}
	statusProvider, err := minecraft.NewForeignStatusProvider(os.Args[2])
	if err != nil {
		panic(err)
	}

	oconfig.Cfg = oconfig.DefaultConfig
	oconfig.Cfg.Network.Transport = oconfig.NetworkTransportTCP

	oconfig.Cfg.Movement.AcceptClientPosition = false
	oconfig.Cfg.Movement.AcceptClientVelocity = false
	oconfig.Cfg.Movement.PersuasionThreshold = 0.005

	oconfig.Cfg.Combat.FullAuthoritative = true
	oconfig.Cfg.Combat.MaxRewind = 4

	//oconfig.Cfg.Movement.AcceptClientPosition = true
	//oconfig.Cfg.Movement.PositionAcceptanceThreshold = 0.0625

	packs, err := utils.ResourcePacks("/home/ethaniccc/temp/proxy-packs", "content_keys.json")
	if err != nil {
		panic(err)
	}

	var netTransport transport.Transport
	switch tr := oconfig.Network().Transport; tr {
	case oconfig.NetworkTransportTCP:
		netTransport = otransport.NewTCP()
	default:
		if tr != oconfig.NetworkTransportSpectral {
			logrus.Warnf("unknown/unsupported transport: %s, defaulting to spectral", tr)
		}
		netTransport = transport.NewSpectral(logger)
	}

	proxy := spectrum.NewSpectrum(
		server.NewStaticDiscovery(os.Args[2], os.Args[2]),
		logger,
		opts,
		netTransport,
	)
	if err := proxy.Listen(minecraft.ListenConfig{
		StatusProvider: statusProvider,
		FlushRate:      -1, // FlushRate is set to -1 to allow Oomph to manually flush the connection.
		AcceptedProtocols: []minecraft.Protocol{
			v776.Protocol(),
			v766.Protocol(),
			v748.Protocol(),
			v729.Protocol(),
			v712.Protocol(),
			v686.Protocol1(),
			v686.Protocol2(),
			v671.Protocol(),
			v662.Protocol(),
			v649.Protocol(),
			v630.Protocol(),
			v622.Protocol(),
			v618.Protocol(),
			v594.Protocol(),
			v589.Protocol(),
		},
		ResourcePacks:        packs,
		TexturePacksRequired: true,

		AllowInvalidPackets: true,
		AllowUnknownPackets: true,

		/* PacketFunc: func(header packet.Header, payload []byte, src, dst net.Addr) {
			var pk packet.Packet
			if f, ok := minecraft.DefaultProtocol.Packets(false)[header.PacketID]; ok {
				pk = f()
			} else if f, ok := minecraft.DefaultProtocol.Packets(true)[header.PacketID]; ok {
				pk = f()
			}

			fmt.Printf("%s -> %s: %T\n", src, dst, pk)
		}, */
	}); err != nil {
		panic(err)
	}

	go func() {
		var interrupt = make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)
		<-interrupt

		for _, s := range proxy.Registry().GetSessions() {
			s.Server().WritePacket(&packet.Disconnect{})
			s.Disconnect("Proxy restarting...")
		}
		os.Exit(0)
	}()

	for {
		initalSession, err := proxy.Accept()
		if err != nil {
			continue
		}

		go func(s *session.Session) {
			// Disable auto-login so that Oomph's processor can modify the StartGame data to allow server-authoritative movement.
			f, err := os.OpenFile(fmt.Sprintf("./logs/%s.log", s.Client().IdentityData().DisplayName), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0744)
			if err != nil {
				s.Disconnect("failed to create log file")
				return
			}
			playerLog := logrus.New()
			playerLog.SetOutput(f)
			playerLog.SetLevel(logrus.DebugLevel)

			proc := oomph.NewProcessor(s, proxy.Registry(), proxy.Listener(), playerLog)
			s.SetProcessor(proc)

			if err := s.Login(); err != nil {
				s.Disconnect(err.Error())
				if !errors.Is(err, context.Canceled) {
					logger.Error("failed to login session", "err", err)
				}
				return
			}

			proc.Player().SetServerConn(s.Server())
		}(initalSession)
	}
}
