package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/gameparrot/gtipc"
	"github.com/google/uuid"
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
	_ "github.com/oomph-ac/oomph"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/oomph-ac/oomph/player/context"
	"github.com/oomph-ac/oomph/player/detection"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"

	"github.com/go-echarts/statsview"
	"github.com/go-echarts/statsview/viewer"
)

var (
	localPort  string
	remoteFile string
)

var ipcHandler gtipc.IpcHandler

// The following program implements a proxy that forwards players from one local address to a remote address.
func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: ./bin <local_port> <remote_file>")
		return
	}

	localPort, remoteFile = os.Args[1], os.Args[2]

	ipc, err := gtipc.NewIPCServer(remoteFile, &gtipc.IpcOptions{
		Upstream: gtipc.CreateGophertunnelUpstreamHandler("raknetupstream"),
	})
	if err != nil {
		panic(err)
	}
	ipcHandler = ipc

	minecraft.RegisterNetwork("ipc", func(l *slog.Logger) minecraft.Network { return ipc })

	p := gtipc.NewIpcStatusProvider("default", ipc)
	listener, err := minecraft.ListenConfig{
		StatusProvider:      p,
		FlushRate:           -1,
		AllowUnknownPackets: true,
		AllowInvalidPackets: true,

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
	}.Listen("raknetupstream", ":"+localPort)
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
		ClientData:          clientDat,
		IdentityData:        conn.IdentityData(),
		FlushRate:           -1,
		KeepXBLIdentityData: true,
	}.Dial("ipc", "default;"+conn.RemoteAddr().String())
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
	p.SetRemoteEventFunc(func(e player.RemoteEvent, p *player.Player) {
		if conn, ok := ipcHandler.GetConn("default"); ok {
			buf := bytes.NewBuffer(nil)
			io := protocol.NewWriter(buf, 0)
			uuidRaw, _ := uuid.Parse(p.IdentityDat.Identity)
			uuid := uuidRaw[:]
			io.ByteSlice(&uuid)
			id := e.ID()
			io.String(&id)
			json, _ := json.Marshal(e)
			io.ByteSlice(&json)
			conn.WriteCustomPacket(buf.Bytes())
		}
	})
	defer p.Close()

	var g sync.WaitGroup
	g.Add(2)
	go func() {
		gameData := serverConn.GameData()
		gameData.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
		gameData.PlayerMovementSettings.RewindHistorySize = 40
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
