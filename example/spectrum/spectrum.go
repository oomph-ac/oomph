package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session"
	"github.com/cooldogedev/spectrum/util"
	"github.com/oomph-ac/oomph"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := slog.Default()
	oomphLog := logrus.New()
	oomphLog.SetLevel(logrus.DebugLevel)

	if len(os.Args) < 3 {
		oomphLog.Fatal("Usage: ./oomph-bin <local_port> <remote_addr> <optional: spectrum_token>")
		return
	}

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
		StatusProvider:    statusProvider,
		FlushRate:         -1, // FlushRate is set to -1 to allow Oomph to manually flush the connection.
		AcceptedProtocols: []minecraft.Protocol{},
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
