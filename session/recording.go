package session

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/disgoorg/json"
	"github.com/oomph-ac/oomph/event"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
)

const CurrentRecordingVer = "1"

type Recording struct {
	Version string

	ClientDat   login.ClientData
	IdentityDat login.IdentityData
	GameDat     minecraft.GameData
	Protocol    int

	Events []event.Event
}

func (s *Session) StartRecording() {
	if s.State.IsRecording {
		return
	}

	if s.State.IsReplay {
		s.log.Warnf("recording on replay is not allowed")
		return
	}

	s.State.IsRecording = true
	go s.handleRecording()

	// Add all the chunks currently in the world into the recording.
	for pos, c := range s.Player.World.GetAllChunks() {
		ev := event.AddChunkEvent{
			Chunk: c,
		}
		ev.EvTime = time.Now().UnixNano()
		ev.Position = pos
		ev.Range = c.Range()
		ev.Chunk = c

		// Add the chunk to the recording.
		s.eventQueue <- ev
	}

	// Update the ACK handler.
	ackHandler := s.Player.Handler(handler.HandlerIDAcknowledgements).(*handler.AcknowledgementHandler)
	ev := event.AckEvent{
		RefreshedTimestmap: ackHandler.CurrentTimestamp,
		SendTimestamp:      ackHandler.CurrentTimestamp,
	}
	ev.EvTime = time.Now().UnixNano()
	s.eventQueue <- ev
}

func (s *Session) StopRecording() {
	if !s.State.IsRecording {
		return
	}

	s.State.IsRecording = false
	select {
	case s.stopRecording <- struct{}{}:
		break
	case <-time.After(time.Second * 5):
		panic(oerror.New("unable to stop recording"))
	}
}

func (s *Session) handleRecording() {
	os.Remove(s.State.RecordingFile)
	f, err := os.OpenFile(s.State.RecordingFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(oerror.New("unable to open recording file: " + err.Error()))
	}
	defer f.Close()

	// Encode the recording version into the header of the recording. This is to ensure that replays will
	// be able to decode the recording.
	f.WriteString(CurrentRecordingVer + "\n")

	// Encode the client data into the header of the recording.
	enc, _ := json.Marshal(s.Player.ClientDat)
	f.Write(enc)
	f.WriteString("\n")

	// Encode the identity data into the header of the recording.
	enc, _ = json.Marshal(s.Player.IdentityDat)
	f.Write(enc)
	f.WriteString("\n")

	// Encode the game data into the header of the recording.
	enc, _ = json.Marshal(s.Player.GameDat)
	f.Write(enc)
	f.WriteString("\n")

	// Encode the version of the player into the header of the recording.
	f.WriteString(fmt.Sprintf("%d\n", s.Player.Version))

	for {
		select {
		case ev := <-s.eventQueue:
			f.Write(ev.Encode())
			f.WriteString("\n")
		case <-s.stopRecording:
			return
		}
	}
}

// DecodeRecording decodes an oomph recording file into a Recording. It returns an error if
// the recording file could not be parsed, or if the version of the recording is not supported.
func DecodeRecording(file string, protocols []minecraft.Protocol) (*Recording, error) {
	protocols = append(protocols, minecraft.DefaultProtocol)

	f, err := os.Open(file)
	if err != nil {
		return nil, oerror.New("unable to open recording file: " + err.Error())
	}

	rawDat, err := io.ReadAll(f)
	if err != nil {
		return nil, oerror.New("unable to read recording file: " + err.Error())
	}

	dat := string(rawDat)
	lines := strings.Split(dat, "\n")

	rec := &Recording{}
	rec.Version = lines[0]

	if rec.Version != CurrentRecordingVer {
		return nil, oerror.New("unsupported recording version: " + rec.Version)
	}

	var clientDat login.ClientData
	if err := json.Unmarshal([]byte(lines[1]), &clientDat); err != nil {
		return nil, oerror.New("unable to decode client data: " + err.Error())
	}

	var identityDat login.IdentityData
	if err := json.Unmarshal([]byte(lines[2]), &identityDat); err != nil {
		return nil, oerror.New("unable to decode identity data: " + err.Error())
	}

	var gameDat minecraft.GameData
	if err := json.Unmarshal([]byte(lines[3]), &gameDat); err != nil {
		return nil, oerror.New("unable to decode game data: " + err.Error())
	}

	_, err = fmt.Sscanf(lines[4], "%d", &rec.Protocol)
	if err != nil {
		return nil, oerror.New("unable to get protocol version: " + err.Error())
	}

	var proto minecraft.Protocol
	for _, p := range protocols {
		if p.ID() == int32(rec.Protocol) {
			proto = p
			break
		}
	}

	if proto == nil {
		return nil, oerror.New("unsupported protocol version: " + fmt.Sprint(rec.Protocol))
	}

	rec.ClientDat = clientDat
	rec.IdentityDat = identityDat
	rec.GameDat = gameDat

	// Remove the header from the recording.
	lines = lines[5:]

	//rec.Events = make([]event.Event, 0, len(lines))
	rec.Events = []event.Event{}
	for _, line := range lines {
		if line == "" {
			continue
		}

		ev, err := event.Decode([]byte(line), minecraft.DefaultProtocol)
		if err != nil {
			return nil, oerror.New("unable to decode event: " + err.Error())
		}

		rec.Events = append(rec.Events, ev)
	}

	return rec, nil
}
