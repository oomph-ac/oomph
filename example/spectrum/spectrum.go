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

	"github.com/akmalfairuz/legacy-version/legacyver"
	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session"
	"github.com/cooldogedev/spectrum/util"
	"github.com/go-echarts/statsview"
	"github.com/go-echarts/statsview/viewer"
	"github.com/oomph-ac/oconfig"
	"github.com/oomph-ac/oomph"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"

	_ "net/http/pprof"

	_ "github.com/oomph-ac/oomph/utils/collisions"

	"github.com/oomph-ac/oomph/utils"
)

var evHandler = player.NewExampleEventHandler()

func main() {
	logger := slog.Default()
	if len(os.Args) < 3 {
		logger.Info("Usage: ./oomph-bin <local_port> <remote_addr> <optional: spectrum_token>")
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
	fmt.Println("process ID:", os.Getpid())

	opts := util.DefaultOpts()
	opts.ClientDecode = player.ClientDecode
	opts.AutoLogin = false
	opts.Addr = ":" + os.Args[1]
	opts.SyncProtocol = false
	opts.LatencyInterval = 1000

	/* if len(os.Args) >= 4 {
		opts.Token = os.Args[3]
	} */
	statusProvider, err := minecraft.NewForeignStatusProvider(os.Args[2])
	if err != nil {
		panic(err)
	}

	oconfig.Global = oconfig.DefaultConfig
	//oconfig.Global.Network.Transport = oconfig.NetworkTransportSpectral

	oconfig.Global.Movement.AcceptClientPosition = false
	oconfig.Global.Movement.PositionAcceptanceThreshold = 0.003
	oconfig.Global.Movement.AcceptClientVelocity = false
	oconfig.Global.Movement.VelocityAcceptanceThreshold = 0.077

	oconfig.Global.Movement.PersuasionThreshold = 0.001
	oconfig.Global.Movement.CorrectionThreshold = 0.003

	oconfig.Global.Combat.MaximumAttackAngle = 90
	oconfig.Global.Combat.EnableClientEntityTracking = true

	oconfig.Global.Network.GlobalMovementCutoffThreshold = -1
	oconfig.Global.Network.MaxEntityRewind = 6
	oconfig.Global.Network.MaxGhostBlockChain = 7
	oconfig.Global.Network.MaxKnockbackDelay = -1
	oconfig.Global.Network.MaxBlockUpdateDelay = -1

	/* packs, err := utils.ResourcePacks("/home/ethaniccc/temp/proxy-packs", "content_keys.json")
	if err != nil {
		panic(err)
	} */

	/* var netTransport transport.Transport
	switch tr := oconfig.Network().Transport; tr {
	case oconfig.NetworkTransportTCP:
		netTransport = otransport.NewTCP()
	default:
		if tr != oconfig.NetworkTransportSpectral {
			logger.Warn("unknown/unsupported transport, defaulting to spectral", "transportMode", tr)
		}
		netTransport = transport.NewSpectral(logger)
	} */

	// Register custom blocks here
	world.FinalizeBlockRegistry()

	proxy := spectrum.NewSpectrum(
		server.NewStaticDiscovery(os.Args[2], os.Args[2]),
		logger,
		opts,
		nil,
	)
	protos := legacyver.All(false)
	if err := proxy.Listen(minecraft.ListenConfig{
		StatusProvider:    statusProvider,
		FlushRate:         -1, // FlushRate is set to -1 to allow Oomph to manually flush the connection.
		AcceptedProtocols: protos,
		//ResourcePacks:        packs,
		TexturePacksRequired: false,

		AllowInvalidPackets: false,
		AllowUnknownPackets: false,

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
		time.Sleep(time.Second)
		os.Exit(0)
	}()

	utils.InitializeBlockNameMapping()

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
			playerLogHandler := slog.NewTextHandler(f, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})
			playerLog := slog.New(playerLogHandler)
			proc := oomph.NewProcessor(s, proxy.Registry(), proxy.Listener(), playerLog)
			proc.Player().SetCloser(func() {
				f.Close()
			})
			proc.Player().SetRecoverFunc(func(p *player.Player, err any) {
				fmt.Println("ERROR:", err)
				debug.PrintStack()
				fmt.Println("Please remember this is an example, and you should set this recovery function to something that can log errors, like Sentry.")
				os.Exit(1)
			})
			proc.Player().AddPerm(player.PermissionDebug)
			proc.Player().AddPerm(player.PermissionAlerts)
			proc.Player().AddPerm(player.PermissionLogs)
			proc.Player().HandleEvents(evHandler)
			s.SetProcessor(proc)

			if err := s.Login(); err != nil {
				s.Disconnect(err.Error())
				f.Close()
				if !errors.Is(err, context.Canceled) {
					logger.Error("failed to login session", "err", err)
				}
				return
			}

			proc.Player().SetServerConn(s.Server())
		}(initalSession)
	}
}
