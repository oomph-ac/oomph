package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session"
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
	"github.com/oomph-ac/oomph"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft"
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
		viewer.SetConfiguration(viewer.WithTheme(viewer.ThemeWesteros), viewer.WithAddr("localhost:8080"))

		mgr := statsview.New()
		go mgr.Start()
		//go http.ListenAndServe("localhost:8080", nil)
	}

	debug.SetGCPercent(-1)
	debug.SetMemoryLimit(1024 * 1024 * 1024) // 1GB

	opts := util.DefaultOpts()
	opts.ClientDecode = player.DecodeClientPackets
	opts.AutoLogin = false
	opts.Addr = ":" + os.Args[1]
	if len(os.Args) >= 4 {
		opts.Token = os.Args[3]
	}

	statusProvider, err := minecraft.NewForeignStatusProvider(os.Args[2])
	if err != nil {
		panic(err)
	}

	proxy := spectrum.NewSpectrum(server.NewStaticDiscovery(os.Args[2], ""), logger, opts, nil)
	if err := proxy.Listen(minecraft.ListenConfig{
		StatusProvider: statusProvider,
		FlushRate:      -1, // FlushRate is set to -1 to allow Oomph to manually flush the connection.
		AcceptedProtocols: []minecraft.Protocol{
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
	}); err != nil {
		panic(err)
	}

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
			proc.Player().Movement().SetValidationThreshold(0.3)
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
