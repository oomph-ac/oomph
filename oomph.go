package oomph

import (
	"errors"
	"fmt"
	"sync"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/justtaldevelops/oomph/oomph"
	"github.com/justtaldevelops/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
)

type Oomph struct {
	playerMutex sync.Mutex
	playerChan  chan *player.Player
	players     map[string]*player.Player
	allower     oomph.Allower
	closer      oomph.Closer
}

// New returns a new Oomph instance.
// If your server is using Dragonfly, be sure to use the Listener function instead.
func New() *Oomph {
	return &Oomph{
		players:    make(map[string]*player.Player),
		playerChan: make(chan *player.Player),
	}
}

// Accept accepts an incoming player into the server. It blocks until a player connects to the server.
// Accept returns an error if the Server is no longer available.
func (o *Oomph) Accept() (*player.Player, error) {
	p, ok := <-o.playerChan
	if !ok {
		return nil, errors.New("oomph shutdown")
	}
	o.playerMutex.Lock()
	o.players[p.Name()] = p
	o.playerMutex.Unlock()
	return p, nil
}

// SetAllower makes Oomph filter which connections to the Server are accepted. Connections on which the Allower returns
// false are rejected immediately. If nil is passed, all connections are accepted.
func (o *Oomph) SetAllower(a oomph.Allower) {
	o.allower = a
}

// SetCloser lets you handle players that leave before they fully join.
func (o *Oomph) SetCloser(c oomph.Closer) {
	o.closer = c
}

// Start will start oomph! remoteAddr is the address of the target server, and localAddr is the address that players will connect to.
// Addresses should be formatted in the following format: "ip:port", ex: "127.0.0.1:19132".
// If you're using dragonfly, use Listen instead of Start.
func (o *Oomph) Start(remoteAddr, localAddr string) error {
	p, err := minecraft.NewForeignStatusProvider(remoteAddr)
	if err != nil {
		panic(err)
	}
	serverConn, err := minecraft.Dialer{}.Dial("raknet", remoteAddr)
	if err != nil {
		panic(err)
	}
	l, err := minecraft.ListenConfig{
		StatusProvider: p,
		ResourcePacks:  serverConn.ResourcePacks(),
	}.Listen("raknet", localAddr)
	if err != nil {
		return err
	}
	defer l.Close()
	fmt.Printf("Oomph is now listening on %v and directing connections to %v!\n", localAddr, remoteAddr)
	for {
		c, err := l.Accept()
		if err != nil {
			panic(err)
		}
		go o.handleConn(c.(*minecraft.Conn), l, remoteAddr)
	}
}

// handleConn handles a new incoming minecraft.Conn from the minecraft.Listener passed.
func (o *Oomph) handleConn(conn *minecraft.Conn, listener *minecraft.Listener, remoteAddr string) {
	serverConn, err := minecraft.Dialer{
		IdentityData: conn.IdentityData(),
		ClientData:   conn.ClientData(),
	}.Dial("raknet", remoteAddr)
	if err != nil {
		return
	}

	if o.closer != nil {
		defer o.closer.Close(conn.IdentityData())
	}

	if o.allower != nil {
		if dc, ok := o.allower.Allow(conn.RemoteAddr(), conn.IdentityData(), conn.ClientData()); !ok {
			_ = listener.Disconnect(conn, dc)
			return
		}
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

	p := player.NewPlayer(lg, world.Overworld, 8, conn, serverConn)
	o.playerChan <- p

	g.Add(2)
	go func() {
		defer func() {
			_ = listener.Disconnect(conn, "connection lost")
			_ = serverConn.Close()
			g.Done()
		}()
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				return
			}
			p.Process(pk, true)
			if err := serverConn.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
		}
	}()
	go func() {
		defer func() {
			_ = serverConn.Close()
			_ = listener.Disconnect(conn, "connection lost")
			g.Done()
		}()
		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
			p.Process(pk, false)
			if err := conn.WritePacket(pk); err != nil {
				return
			}
		}
	}()
	g.Wait()
	p.ClosePlayer()
}
