package session

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/oomph-ac/oomph/event"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (s *Session) Conn() *minecraft.Conn {
	return s.Player.Conn()
}

func (s *Session) ServerConn() *minecraft.Conn {
	return s.Player.ServerConn()
}

func (s *Session) SetConn(conn *minecraft.Conn) {
	s.Player.SetConn(conn)
}

func (s *Session) SetServerConn(conn *minecraft.Conn) {
	s.Player.SetServerConn(conn)
}

func (s *Session) ChunkRadius() int {
	return s.Player.ChunkRadius()
}

func (s *Session) ClientCacheEnabled() bool {
	return false
}

func (s *Session) IdentityData() login.IdentityData {
	return s.Player.IdentityData()
}

func (s *Session) ClientData() login.ClientData {
	return s.Player.ClientData()
}

func (s *Session) Flush() error {
	return s.Player.Flush()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.Player.RemoteAddr()
}

func (s *Session) Latency() time.Duration {
	return s.Player.Latency()
}

func (s *Session) WritePacket(pk packet.Packet) error {
	defer func() {
		if err := recover(); err != nil {
			s.log.Errorf("WritePacket() panic: %v", err)
			hub := sentry.CurrentHub().Clone()
			hub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("conn_type", "serverDirect")
				scope.SetTag("player", s.IdentityData().DisplayName)
			})

			hub.Recover(oerror.New(fmt.Sprintf("%v", err)))
			hub.Flush(time.Second * 5)
		}
	}()

	if s.Player.Closed || s.Player.ServerPkFunc == nil {
		return oerror.New("oomph player was closed")
	}

	if s.Conn() == nil {
		return oerror.New("conn is nil in session")
	}

	ev := event.PacketEvent{
		Packets: []packet.Packet{pk},
		Server:  true,
	}
	ev.EvTime = time.Now().UnixNano()

	return s.QueueEvent(ev)
}

func (s *Session) ReadPacket() (pk packet.Packet, err error) {
	defer func() {
		if err := recover(); err != nil {
			s.log.Errorf("ReadPacket() panic: %v", err)
			hub := sentry.CurrentHub().Clone()
			hub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("conn_type", "clientDirect")
				scope.SetTag("player", s.IdentityData().DisplayName)
			})

			hub.Recover(oerror.New(fmt.Sprintf("%v", err)))
			hub.Flush(time.Second * 5)
		}
	}()

	if s.Player.Closed || s.Player.ClientPkFunc == nil {
		return pk, oerror.New("oomph player was closed")
	}

	if s.Conn() == nil {
		return pk, oerror.New("conn is nil")
	}

	if len(s.Player.PacketQueue) > 0 {
		pk = s.Player.PacketQueue[0]
		s.Player.PacketQueue = s.Player.PacketQueue[1:]
		return pk, nil
	}

	pk, err = s.Conn().ReadPacket()
	if err != nil {
		return pk, err
	}

	ev := event.PacketEvent{
		Packets: []packet.Packet{pk},
		Server:  false,
	}
	ev.EvTime = time.Now().UnixNano()

	if err := s.QueueEvent(ev); err != nil {
		s.log.Errorf("error handling packets from client: %v", err)
		return pk, err
	}

	s.Player.PacketQueue = append(s.Player.PacketQueue, pk)
	return pk, nil
}

func (s *Session) StartGameContext(ctx context.Context, data minecraft.GameData) error {
	return s.Player.StartGameContext(ctx, data)
}
