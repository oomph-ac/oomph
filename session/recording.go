package session

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/disgoorg/json"
	"github.com/oomph-ac/oomph/event"
	"github.com/oomph-ac/oomph/handler"
	_ "github.com/oomph-ac/oomph/handler/ackfunc"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
)

const StartRecDescription = "============== RECORDING STARTS HERE ==============\n"

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
	ev := event.AckRefreshEvent{
		RefreshedTimestmap: ackHandler.CurrentTimestamp,
		SendTimestamp:      ackHandler.CurrentTimestamp,
	}
	ev.EvTime = time.Now().UnixNano()
	s.eventQueue <- ev

	for t, ackList := range ackHandler.AckMap {
		ev := event.AckInsertEvent{
			Timestamp: t,
			Acks:      ackList,
		}
		ev.EvTime = time.Now().UnixNano()
		s.eventQueue <- ev
	}
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
	f.WriteString(event.EventsVersion + "\n")

	// Encode the client data into the header of the recording.
	enc, _ := json.Marshal(s.Player.ClientDat)
	f.WriteString(base64.StdEncoding.EncodeToString(enc))
	f.WriteString("\n")

	// Encode the identity data into the header of the recording.
	enc, _ = json.Marshal(s.Player.IdentityDat)
	f.WriteString(base64.StdEncoding.EncodeToString(enc))
	f.WriteString("\n")

	// Encode the game data into the header of the recording.
	enc, _ = json.Marshal(s.Player.GameDat)
	f.WriteString(base64.StdEncoding.EncodeToString(enc))
	f.WriteString("\n")

	// Encode the version of the player into the header of the recording.
	f.WriteString(fmt.Sprintf("%d\n", s.Player.Version))

	// Little description hack lol.
	f.WriteString(StartRecDescription)

	for {
		select {
		case ev := <-s.eventQueue:
			f.Write(ev.Encode())
		case <-s.stopRecording:
			return
		}
	}
}

// DecodeRecording decodes an oomph recording file into a Recording. It returns an error if
// the recording file could not be parsed, or if the version of the recording is not supported.
func DecodeRecording(file string) (*Recording, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, oerror.New("unable to open recording file: " + err.Error())
	}

	rawDat, err := io.ReadAll(f)
	if err != nil {
		return nil, oerror.New("unable to read recording file: " + err.Error())
	}

	dat := string(rawDat)
	split := strings.Split(dat, StartRecDescription)
	header := strings.Split(split[0], "\n")
	events := split[1]

	rec := &Recording{}
	rec.Version = header[0]
	fmt.Println(rec.Version)

	if rec.Version != event.EventsVersion {
		return nil, oerror.New("unsupported recording version: " + rec.Version)
	}

	var clientDat login.ClientData
	b64dec, err := base64.StdEncoding.DecodeString(header[1])
	if err != nil {
		return nil, oerror.New("unable to decode client data: " + err.Error())
	}

	if err := json.Unmarshal(b64dec, &clientDat); err != nil {
		return nil, oerror.New("unable to decode client data: " + err.Error())
	}

	var identityDat login.IdentityData
	b64dec, err = base64.StdEncoding.DecodeString(header[2])
	if err != nil {
		return nil, oerror.New("unable to decode identity data: " + err.Error())
	}

	if err := json.Unmarshal(b64dec, &identityDat); err != nil {
		return nil, oerror.New("unable to decode identity data: " + err.Error())
	}

	var gameDat minecraft.GameData
	b64dec, err = base64.StdEncoding.DecodeString(header[3])
	if err != nil {
		return nil, oerror.New("unable to decode game data: " + err.Error())
	}

	if err := json.Unmarshal(b64dec, &gameDat); err != nil {
		return nil, oerror.New("unable to decode game data: " + err.Error())
	}

	_, err = fmt.Sscanf(header[4], "%d", &rec.Protocol)
	if err != nil {
		return nil, oerror.New("unable to get protocol version: " + err.Error())
	}

	if rec.Protocol != int(minecraft.DefaultProtocol.ID()) {
		return nil, oerror.New("outdated protocol version: " + fmt.Sprint(rec.Protocol))
	}

	rec.ClientDat = clientDat
	rec.IdentityDat = identityDat
	rec.GameDat = gameDat

	// Remove the header from the recording.
	rec.Events, err = event.DecodeEvents([]byte(events))
	if err != nil {
		fmt.Printf("unable to decode events: %v\n", err.Error())
	}

	return rec, nil
}
