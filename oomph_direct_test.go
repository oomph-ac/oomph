package oomph

import (
	"fmt"
	"os"
	"testing"

	"github.com/df-mc/dragonfly/server/entity"
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

	oomph := New(log, ":19132")
	oomph.Listen(&cfg, cfg.Name, []minecraft.Protocol{}, false)

	srv := cfg.New()
	srv.CloseOnProgramEnd()

	go func() {
		for {
			if _, err := oomph.Accept(); err != nil {
				return
			}
		}
	}()

	srv.Listen()

	for srv.Accept(func(p *player.Player) {
		p.ShowCoordinates()
		p.SetGameMode(world.GameModeSurvival)
		p.Heal(20, entity.FoodHealingSource{})
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
