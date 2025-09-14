package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/akmalfairuz/legacy-version/legacyver"

	_ "github.com/oomph-ac/oomph"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/oomph-ac/oomph/player/context"
	"github.com/oomph-ac/oomph/player/detection"

	"github.com/sandertv/gophertunnel/minecraft"

	"github.com/go-echarts/statsview"
	"github.com/go-echarts/statsview/viewer"
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
	protos := legacyver.All(false)
	listener, err := minecraft.ListenConfig{
		StatusProvider:      p,
		FlushRate:           -1,
		AllowUnknownPackets: true,
		AllowInvalidPackets: true,

		AcceptedProtocols: protos,
	}.Listen("raknet", ":"+localPort)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	if os.Getenv("PPROF_ENABLED") != "" {
		// set configurations before calling `statsview.New()` method
		viewer.SetConfiguration(viewer.WithTheme(viewer.ThemeWesteros), viewer.WithAddr("localhost:8080"))

		mgr := statsview.New()
		go mgr.Start()
		//go http.ListenAndServe("localhost:8080", nil)
	}

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

	// Create a TextHandler that writes to the file
	handler := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
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
		//gameData.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
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

			/* switch pk := pk.(type) {
			case *packet.PlayerAuthInput, *packet.NetworkStackLatency:
			default:
				fmt.Printf("Client -> Server: %T\n", pk)
			} */
			pkCtx := context.NewHandlePacketContext(&pk)
			p.HandleClientPacket(pkCtx)
			if pkCtx.Cancelled() {
				continue
			}

			if err := serverConn.WritePacket(pk); err != nil {
				var disc minecraft.DisconnectError
				if ok := errors.As(err, &disc); ok {
					fmt.Println(err, "Client -> Server")
					_ = listener.Disconnect(conn, disc.Error())
				}
				return
			}
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
					fmt.Println(err, "Server -> Client")
					_ = listener.Disconnect(conn, disc.Error())
				}
				return
			}

			pkCtx := context.NewHandlePacketContext(&pk)
			p.HandleServerPacket(pkCtx)
			if !pkCtx.Cancelled() {
				if err := conn.WritePacket(pk); err != nil {
					return
				}
			}
		}
	}()
	<-completion
	fmt.Println("Connection closed.")
}
