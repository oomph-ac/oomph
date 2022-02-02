package main

import (
	"errors"
	"fmt"
	"github.com/RestartFU/gophig"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/justtaldevelops/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"sync"
)

func main() {
	config := readConfig()

	p, err := minecraft.NewForeignStatusProvider(config.Connection.RemoteAddress)
	if err != nil {
		panic(err)
	}
	listener, err := minecraft.ListenConfig{
		StatusProvider: p,
	}.Listen("raknet", config.Connection.LocalAddress)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	fmt.Printf("Oomph is now listening on %v and directing connections to %v!\n", config.Connection.LocalAddress, config.Connection.RemoteAddress)
	for {
		c, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go handleConn(c.(*minecraft.Conn), listener, config)
	}
}

// handleConn handles a new incoming minecraft.Conn from the minecraft.Listener passed.
func handleConn(conn *minecraft.Conn, listener *minecraft.Listener, config config) {
	serverConn, err := minecraft.Dialer{
		IdentityData: conn.IdentityData(),
		ClientData:   conn.ClientData(),
	}.Dial("raknet", config.Connection.RemoteAddress)
	if err != nil {
		return
	}

	var g sync.WaitGroup
	g.Add(2)
	go func() {
		if err := conn.StartGame(serverConn.GameData()); err != nil {
			return
		}
		g.Done()
	}()
	go func() {
		if err := serverConn.DoSpawn(); err != nil {
			return
		}
		g.Done()
	}()
	g.Wait()

	lg := logrus.New()
	lg.Formatter = &logrus.TextFormatter{ForceColors: true}
	lg.Level = logrus.DebugLevel

	viewDistance := int32(8)
	p := player.NewPlayer(lg, world.Overworld, viewDistance, conn, serverConn)

	g.Add(2)
	go func() {
		defer listener.Disconnect(conn, "connection lost")
		defer serverConn.Close()
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				return
			}
			p.Process(pk, conn)
			if err := serverConn.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
		}
		g.Done()
	}()
	go func() {
		defer serverConn.Close()
		defer listener.Disconnect(conn, "connection lost")
		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
			p.Process(pk, serverConn)
			if err := conn.WritePacket(pk); err != nil {
				return
			}
		}
		g.Done()
	}()
	g.Wait()
	p.Close()
}

type config struct {
	Connection struct {
		LocalAddress  string
		RemoteAddress string
	}
}

func readConfig() config {
	var c config
	if _, err := os.Stat("config.toml"); os.IsNotExist(err) {
		if err := gophig.SetConfComplex("config.toml", gophig.TOMLMarshaler{}, c, 0777); err != nil {
			log.Fatalf("error creating config: %v", err)
		}
	}
	if err := gophig.GetConfComplex("config.toml", gophig.TOMLMarshaler{}, &c); err != nil {
		log.Fatalf("error reading config: %v", err)
	}
	if c.Connection.LocalAddress == "" {
		c.Connection.LocalAddress = "0.0.0.0:19132"
	}
	if c.Connection.RemoteAddress == "" {
		c.Connection.RemoteAddress = "0.0.0.0:19133"
	}
	if err := gophig.SetConfComplex("config.toml", gophig.TOMLMarshaler{}, c, 0777); err != nil {
		log.Fatalf("error writing config file: %v", err)
	}
	return c
}
