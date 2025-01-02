package main

import (
	"context"
	"errors"
	"log/slog"

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

	opts := util.DefaultOpts()
	opts.ClientDecode = player.DecodeClientPackets
	opts.AutoLogin = false

	proxy := spectrum.NewSpectrum(server.NewStaticDiscovery("127.0.0.1:20000", ""), logger, opts, nil)
	if err := proxy.Listen(minecraft.ListenConfig{
		StatusProvider: util.NewStatusProvider("Spectrum Proxy", "Spectrum"),
		FlushRate:      -1, // FlushRate is set to -1 to allow Oomph to manually flush the connection.
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
			s.SetProcessor(oomph.NewProcessor(s, proxy.Registry(), proxy.Listener(), oomphLog))

			if err := s.Login(); err != nil {
				s.Disconnect(err.Error())
				if !errors.Is(err, context.Canceled) {
					logger.Error("failed to login session", "err", err)
				}
				return
			}
			(s.Processor().(*oomph.Processor)).Player().SetServerConn(s.Server())
		}(initalSession)
	}
}
