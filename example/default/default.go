package main

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/oomph-ac/oomph"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/oomph-ac/oomph/player/detection"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

var (
	localPort  string
	remoteAddr string
)

// The following program implements a proxy that forwards players from one local address to a remote address.
func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: ./bin <local_port> <remote_addr>")
		return
	}

	localPort, remoteAddr = os.Args[1], os.Args[2]
	p, err := minecraft.NewForeignStatusProvider(remoteAddr)
	if err != nil {
		panic(err)
	}
	listener, err := minecraft.ListenConfig{
		StatusProvider: p,
		FlushRate:      -1,
	}.Listen("raknet", ":"+localPort)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	for {
		c, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go handleConn(c.(*minecraft.Conn), listener)
	}
}

// handleConn handles a new incoming minecraft.Conn from the minecraft.Listener passed.
func handleConn(conn *minecraft.Conn, listener *minecraft.Listener) {
	clientDat := conn.ClientData()
	clientDat.ThirdPartyName = conn.IdentityData().DisplayName

	serverConn, err := minecraft.Dialer{
		ClientData:   clientDat,
		IdentityData: conn.IdentityData(),
		FlushRate:    -1,
	}.Dial("raknet", remoteAddr)
	if err != nil {
		panic(err)
	}

	f, err := os.OpenFile(fmt.Sprintf("./logs/%s.log", conn.IdentityData().DisplayName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:     false,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	})
	logger.SetOutput(f)
	p := player.New(logger, player.MonitoringState{
		IsReplay:    false,
		IsRecording: false,
		CurrentTime: time.Now(),
	}, listener)
	component.Register(p)
	detection.Register(p)
	p.SetConn(conn)
	defer p.Close()

	var g sync.WaitGroup
	g.Add(2)
	go func() {
		gameData := serverConn.GameData()
		gameData.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
		gameData.PlayerMovementSettings.RewindHistorySize = 100
		if err := conn.StartGame(gameData); err != nil {
			panic(err)
		}
		g.Done()
	}()
	go func() {
		if err := serverConn.DoSpawn(); err != nil {
			panic(err)
		}
		g.Done()
	}()
	g.Wait()

	p.SetServerConn(serverConn)
	go p.StartTicking()

	fmt.Println(p.RuntimeId)

	completion := make(chan struct{}, 1)
	go func() {
		defer listener.Disconnect(conn, "connection lost")
		defer serverConn.Close()
		defer func() {
			completion <- struct{}{}
		}()

		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				return
			}

			if cancel := p.HandleClientPacket(pk); cancel {
				continue
			}

			if err := serverConn.WritePacket(pk); err != nil {
				var disc minecraft.DisconnectError
				if ok := errors.As(err, &disc); ok {
					_ = listener.Disconnect(conn, disc.Error())
				}
				return
			}
			serverConn.Flush()
		}
	}()
	go func() {
		defer serverConn.Close()
		defer listener.Disconnect(conn, "connection lost")
		defer func() {
			completion <- struct{}{}
		}()

		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				var disc minecraft.DisconnectError
				if ok := errors.As(err, &disc); ok {
					_ = listener.Disconnect(conn, disc.Error())
				}
				return
			}
			p.HandleServerPacket(pk)
			if err := conn.WritePacket(pk); err != nil {
				return
			}
		}
	}()
	<-completion
}
