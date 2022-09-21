package oomph

import (
	"fmt"
	"os"
	"testing"

	"github.com/df-mc/dragonfly/server/entity/healing"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/sandertv/gophertunnel/minecraft"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
)

func TestOomphDirect(t *testing.T) {
	log := logrus.New()
	log.Formatter = &logrus.TextFormatter{ForceColors: true}
	log.Level = logrus.DebugLevel

	chat.Global.Subscribe(chat.StdoutSubscriber{})

	cfg, err := readConfig(log)
	if err != nil {
		log.Fatalln(err)
	}

	srv := cfg.New()
	srv.CloseOnProgramEnd()
	/* if err := srv.Start(); err != nil {
		log.Fatalln(err)
	} */

	go func() {
		oomph := New(log, ":19132")
		if err := oomph.Listen(&cfg, cfg.Name, []minecraft.Protocol{}, false); err != nil {
			log.Fatalln(err)
		}

		for {
			if _, err := oomph.Accept(); err != nil {
				return
			}
		}
	}()

	for srv.Accept(func(p *player.Player) {
		p.ShowCoordinates()
		p.SetGameMode(world.GameModeSurvival)
		p.Heal(20, healing.SourceFood{})
		p.AddFood(20)
	}) {
	}
}

// readConfig reads the configuration from the config.toml file, or creates the
// file if it does not yet exist.
func readConfig(log server.Logger) (server.Config, error) {
	c := server.DefaultConfig()
	var zero server.Config
	if _, err := os.Stat("config.toml"); os.IsNotExist(err) {
		data, err := toml.Marshal(c)
		if err != nil {
			return zero, fmt.Errorf("encode default config: %v", err)
		}
		if err := os.WriteFile("config.toml", data, 0644); err != nil {
			return zero, fmt.Errorf("create default config: %v", err)
		}
		return zero, nil
	}
	data, err := os.ReadFile("config.toml")
	if err != nil {
		return zero, fmt.Errorf("read config: %v", err)
	}
	if err := toml.Unmarshal(data, &c); err != nil {
		return zero, fmt.Errorf("decode config: %v", err)
	}
	return c.Config(log)
}
