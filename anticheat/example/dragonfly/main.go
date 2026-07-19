package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player/chat"
	oomphdragonfly "github.com/oomph-ac/oomph/anticheat/integration/dragonfly"
	"github.com/oomph-ac/oomph/anticheat/player"
)

func main() {
	ctx := context.Background()
	log := slog.Default()
	chat.Global.Subscribe(chat.StdoutSubscriber{})

	conf, err := server.DefaultConfig().Config(log)
	if err != nil {
		log.Error("load Dragonfly config", "err", err)
		os.Exit(1)
	}
	address := os.Getenv("OOMPH_ADDRESS")
	if address == "" {
		address = ":19132"
	}
	events := player.NewExampleEventHandler()
	conf.Listeners = []func(server.Config) (server.Listener, error){
		oomphdragonfly.Listener(ctx, oomphdragonfly.Config{
			Address: address,
			Configure: func(p *player.Player) {
				p.HandleEvents(events)
			},
		}),
	}

	srv := conf.New()
	srv.CloseOnProgramEnd()
	srv.Listen()
	for range srv.Accept() {
	}
}
