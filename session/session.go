package session

import (
	"time"

	"github.com/oomph-ac/oomph/event"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/handler/ack"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type Session struct {
	// Player is the player that is associated with the current session.
	Player *player.Player
	// State is the state for the current session.
	State SessionState

	eventQueue    chan event.Event
	stopRecording chan struct{}
	log           *logrus.Logger
}

type SessionState struct {
	// IsRecording is true if the events passed to the current session are being replayed.
	// IsRecording and IsReplay may not be true at the same time.
	IsRecording bool
	// RecordingFile is the file that the events will be recorded to.
	RecordingFile string
	// RecordingDuration is the duration of the recording in server ticks,
	RecordingDuration int64

	// IsReplay is true if the current session is a replay of a previous session.
	IsReplay bool
	// DirectMode is true if the session is not a replay, and is in use with Dragonfly directly.
	DirectMode bool

	// Closed is true if the session is no longer active.
	Closed bool

	// CurrentTime is the current time of the session. This is used primarily for support of replays.
	CurrentTime time.Time
}

// New creates a new session with the logger and given settings.
func New(log *logrus.Logger, s SessionState) *Session {
	p := player.New(log, player.MonitoringState{
		IsReplay:    s.IsReplay,
		IsRecording: s.IsRecording,
	})

	session := &Session{
		Player: p,
		State:  s,

		eventQueue:    make(chan event.Event, 128),
		stopRecording: make(chan struct{}),
		log:           log,
	}

	if s.DirectMode {
		p.ClientPkFunc = func(pks []packet.Packet) error {
			p.ProcessMu.Lock()
			defer p.ProcessMu.Unlock()

			return p.DefaultHandleFromClient(pks)
		}
		p.ServerPkFunc = func(pks []packet.Packet) error {
			p.ProcessMu.Lock()
			defer p.ProcessMu.Unlock()

			return p.DefaultHandleFromServer(pks)
		}
	} else {
		p.ClientPkFunc = func(pks []packet.Packet) error {
			p.ProcessMu.Lock()
			defer p.ProcessMu.Unlock()

			if err := session.Player.DefaultHandleFromClient(pks); err != nil {
				return err
			}

			if session.State.IsReplay {
				return nil
			}
			return p.ServerConn().Flush()
		}
		p.ServerPkFunc = func(pks []packet.Packet) error {
			p.ProcessMu.Lock()
			defer p.ProcessMu.Unlock()

			if err := p.DefaultHandleFromServer(pks); err != nil {
				return err
			}

			ackHandler := p.Handler(handler.HandlerIDAcknowledgements).(*handler.AcknowledgementHandler)
			ev := event.AckRefreshEvent{
				SendTimestamp: ackHandler.CurrentTimestamp,
			}
			ackHandler.Flush(p)
			ev.RefreshedTimestmap = ackHandler.CurrentTimestamp
			ev.EvTime = time.Now().UnixNano()
			session.QueueEvent(ev)

			if session.State.IsReplay {
				return nil
			}
			return p.Conn().Flush()
		}
	}

	go session.startTicking()
	return session
}

// QueueEvent queues an event to be processed by the session.
func (s *Session) QueueEvent(ev event.Event) error {
	if s.State.IsReplay {
		return nil
	}

	if s.State.IsRecording {
		select {
		case s.eventQueue <- ev:
			break
		case <-time.After(time.Second * 5):
			return oerror.New("event queue full - deadlock")
		}
	}

	return s.ProcessEvent(ev)
}

func (s *Session) Close() error {
	s.State.Closed = true
	s.StopRecording()

	return s.Player.Close()
}

// ProcessEvent processes an event.
func (s *Session) ProcessEvent(ev event.Event) error {
	if s.State.Closed {
		return nil
	}

	s.State.CurrentTime = time.Unix(0, ev.Time())
	s.Player.SetTime(s.State.CurrentTime)

	switch ev := ev.(type) {
	case event.PacketEvent:
		if len(ev.Packets) == 0 {
			return oerror.New("0 packets in batch")
		}

		if ev.Server {
			return s.Player.ServerPkFunc(ev.Packets)
		}
		return s.Player.ClientPkFunc(ev.Packets)
	case event.AckRefreshEvent:
		// This shouldn't be processed in an active session.
		if !s.State.IsReplay {
			return nil
		}

		ackHandler := s.Player.Handler(handler.HandlerIDAcknowledgements).(*handler.AcknowledgementHandler)
		if ackHandler.CurrentTimestamp != ev.SendTimestamp {
			ackHandler.AckMap[ev.SendTimestamp] = ackHandler.AckMap[ackHandler.CurrentTimestamp]
			delete(ackHandler.AckMap, ackHandler.CurrentTimestamp)
		}

		ackHandler.CurrentTimestamp = ev.RefreshedTimestmap
	case event.AckInsertEvent:
		// This shouldn't be processed in an active session.
		if !s.State.IsReplay {
			return nil
		}

		ackHandler := s.Player.Handler(handler.HandlerIDAcknowledgements).(*handler.AcknowledgementHandler)

		ackBatch := ack.NewBatch()
		for _, a := range ev.Acks {
			ackBatch.Add(a)
		}
		ackHandler.AckMap[ev.Timestamp] = ackBatch
	case event.TickEvent:
		// This shouldn't be processed in an active session.
		if !s.State.IsReplay {
			return nil
		}

		s.Player.Tick()
		s.Player.ServerTick = ev.Tick
	case event.AddChunkEvent:
		// This shouldn't be processed in an active session.
		if !s.State.IsReplay {
			return nil
		}

		s.Player.World.AddChunk(ev.Position, ev.Chunk)
	}

	if s.Player.Closed {
		s.State.Closed = true
	}

	return nil
}

// startTicking runs a startTicking on the associated player.
func (s *Session) startTicking() {
	if s.State.IsReplay {
		return
	}

	t := time.NewTicker(time.Millisecond * 50)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			ackHandler := s.Player.Handler(handler.HandlerIDAcknowledgements).(*handler.AcknowledgementHandler)
			var ackEv *event.AckRefreshEvent
			if s.State.DirectMode {
				ackEv = &event.AckRefreshEvent{
					SendTimestamp: ackHandler.CurrentTimestamp,
				}
				ackEv.EvTime = time.Now().UnixNano()
			}
			s.Player.Tick()

			ev := event.TickEvent{
				Tick: s.Player.ServerTick,
			}
			ev.EvTime = time.Now().UnixNano()
			s.QueueEvent(ev)

			if ackEv != nil {
				ackEv.RefreshedTimestmap = ackHandler.CurrentTimestamp
				s.QueueEvent(ackEv)
			}

			s.State.RecordingDuration--
			if s.State.IsRecording && s.State.RecordingDuration <= 0 {
				s.StopRecording()
			}
		case f := <-s.Player.RunChan:
			s.Player.ProcessMu.Lock()
			f()
			s.Player.ProcessMu.Unlock()
		case <-s.Player.CloseChan:
			return
		}
	}
}
