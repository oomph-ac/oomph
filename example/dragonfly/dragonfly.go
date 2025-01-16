package main

import (
	"log/slog"

	"github.com/cooldogedev/spectrum-df"
	"github.com/cooldogedev/spectrum-df/util"
	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var packetsToDecode = []uint32{
	packet.IDAddActor,
	packet.IDAddPlayer,
	packet.IDChunkRadiusUpdated,
	packet.IDLevelChunk,
	packet.IDMobEffect,
	packet.IDMoveActorAbsolute,
	packet.IDMovePlayer,
	packet.IDRemoveActor,
	packet.IDSetActorData,
	packet.IDSetActorMotion,
	packet.IDSubChunk,
	packet.IDUpdateAbilities,
	packet.IDUpdateAttributes,
	packet.IDUpdateBlock,
}

func main() {
	for _, id := range packetsToDecode {
		util.RegisterPacketDecode(id, true)
	}

	log := slog.Default()
	chat.Global.Subscribe(chat.StdoutSubscriber{})
	conf, err := server.DefaultConfig().Config(log)
	if err != nil {
		panic(err)
	}

	conf.Listeners = []func(conf server.Config) (server.Listener, error){func(conf server.Config) (server.Listener, error) {
		return spectrum.NewListener(":19133", nil, nil)
	}}
	srv := conf.New()
	srv.CloseOnProgramEnd()
	srv.Listen()
	for _ = range srv.Accept() {
	}
}
