package dragonfly

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/df-mc/dragonfly/server/session"
	"github.com/oomph-ac/oomph/anticheat/player"
	playercontext "github.com/oomph-ac/oomph/anticheat/player/context"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// sessionConn adapts an Oomph player to Dragonfly's connection contract. It
// owns all embedded-server packet routing so player.Player remains independent
// of Dragonfly's transport lifecycle.
type sessionConn struct {
	session.Conn
	player *player.Player

	reads chan packetRead
	done  chan struct{}

	injectedMu     sync.Mutex
	injected       []packet.Packet
	injectedHead   int
	injectedNotify chan struct{}

	gameData minecraft.GameData
	closeOne sync.Once
	closeErr error
}

type packetRead struct {
	packet packet.Packet
	err    error
}

func newSessionConn(raw session.Conn, p *player.Player) *sessionConn {
	c := &sessionConn{
		Conn:           raw,
		player:         p,
		reads:          make(chan packetRead),
		done:           make(chan struct{}),
		injectedNotify: make(chan struct{}, 1),
	}
	go c.readClientPackets()
	return c
}

func (c *sessionConn) readClientPackets() {
	for {
		pk, err := c.Conn.ReadPacket()
		select {
		case c.reads <- packetRead{packet: pk, err: err}:
		case <-c.done:
			return
		}
		if err != nil {
			return
		}
	}
}

func (c *sessionConn) ReadPacket() (packet.Packet, error) {
	for {
		if pk, ok := c.popInjected(); ok {
			return pk, nil
		}
		select {
		case <-c.injectedNotify:
			continue
		case result := <-c.reads:
			if result.err != nil {
				return nil, result.err
			}
			if c.player == nil {
				return result.packet, nil
			}
			ctx := playercontext.NewHandlePacketContext(&result.packet)
			c.player.HandleClientPacket(ctx)
			if ctx.Cancelled() {
				continue
			}
			return *ctx.Packet(), nil
		case <-c.done:
			return nil, io.EOF
		}
	}
}

func (c *sessionConn) WritePacket(pk packet.Packet) error {
	if c.player != nil {
		ctx := playercontext.NewHandlePacketContext(&pk)
		c.player.HandleServerPacket(ctx)
		if ctx.Cancelled() {
			return nil
		}
		pk = *ctx.Packet()
	}
	return c.Conn.WritePacket(pk)
}

// ClientCacheEnabled remains disabled because Oomph does not yet track the
// client blob cache as part of its authoritative world state.
func (*sessionConn) ClientCacheEnabled() bool { return false }

func (c *sessionConn) StartGameContext(ctx context.Context, data minecraft.GameData) error {
	data.PlayerMovementSettings.RewindHistorySize = 100
	c.gameData = data
	if c.player != nil {
		c.player.SetServerConn(&embeddedServerConn{conn: c})
	}
	if err := c.Conn.StartGameContext(ctx, data); err != nil {
		return err
	}
	if c.player != nil {
		go c.player.StartTicking()
	}
	return nil
}

func (c *sessionConn) Close() error {
	c.closeOne.Do(func() {
		close(c.done)
		var playerErr error
		if c.player != nil {
			playerErr = c.player.Close()
		}
		c.closeErr = errors.Join(playerErr, c.Conn.Close())
	})
	return c.closeErr
}

func (c *sessionConn) enqueueInjected(pk packet.Packet) {
	c.injectedMu.Lock()
	c.injected = append(c.injected, pk)
	c.injectedMu.Unlock()
	select {
	case c.injectedNotify <- struct{}{}:
	default:
	}
}

func (c *sessionConn) popInjected() (packet.Packet, bool) {
	c.injectedMu.Lock()
	defer c.injectedMu.Unlock()
	if c.injectedHead == len(c.injected) {
		return nil, false
	}
	pk := c.injected[c.injectedHead]
	c.injected[c.injectedHead] = nil
	c.injectedHead++
	if c.injectedHead == len(c.injected) {
		c.injected = c.injected[:0]
		c.injectedHead = 0
	} else if c.injectedHead >= 64 && c.injectedHead*2 >= len(c.injected) {
		c.injected = append(c.injected[:0], c.injected[c.injectedHead:]...)
		c.injectedHead = 0
	}
	return pk, true
}

type embeddedServerConn struct {
	conn *sessionConn
}

func (c *embeddedServerConn) WritePacket(pk packet.Packet) error {
	c.conn.enqueueInjected(pk)
	return nil
}

func (c *embeddedServerConn) GameData() minecraft.GameData { return c.conn.gameData }
func (*embeddedServerConn) Close() error                   { return nil }

var _ session.Conn = (*sessionConn)(nil)
var _ player.ServerConn = (*embeddedServerConn)(nil)
