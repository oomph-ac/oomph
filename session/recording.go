package session

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/disgoorg/json"
	"github.com/oomph-ac/oomph/detection"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/event"
	"github.com/oomph-ac/oomph/handler"
	_ "github.com/oomph-ac/oomph/handler/ackfunc"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
)

const StartRecDescription = "============== RECORDING STARTS HERE ==============\n"

type Recording struct {
	Version string

	ClientDat   login.ClientData
	IdentityDat login.IdentityData
	GameDat     minecraft.GameData

	Protocol int

	PlayerData struct {
		ClientTick  int64
		ClientFrame int64
		ServerTick  int64
		Tps         float32
		GameMode    int32

		Connected bool
		Ready     bool
		Alive     bool

		MovementHandler handler.MovementHandler
		CombatHandler   handler.CombatHandler

		Detections []player.Handler
	}
	Entities map[uint64]*entity.Entity

	Events []event.Event
}

func (s *Session) StartRecording(duration int64) {
	if s.State.IsRecording || s.Player.Closed || !s.Player.Connected {
		return
	}

	if s.State.IsReplay {
		s.log.Warnf("recording on replay is not allowed")
		return
	}

	s.State.IsRecording = true
	s.State.RecordingDuration = duration
	s.Player.RunWhenFree(s.actuallyStartRecording)
}

func (s *Session) actuallyStartRecording() {
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
	case <-time.After(time.Second * 5): // uh oh what the fuck happened here
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

	// Encode the tick data, gamemode, and other states of the player into the header of the recording.
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	utils.WriteLInt64(buf, s.Player.ClientTick)
	utils.WriteLInt64(buf, s.Player.ClientFrame)
	utils.WriteLInt64(buf, s.Player.ServerTick)
	utils.WriteLFloat32(buf, s.Player.Tps)
	utils.WriteLInt32(buf, s.Player.GameMode)
	utils.WriteBool(buf, s.Player.Connected)
	utils.WriteBool(buf, s.Player.Ready)
	utils.WriteBool(buf, s.Player.Alive)
	f.WriteString(base64.StdEncoding.EncodeToString(buf.Bytes()))
	f.WriteString("\n")

	// Encode the movement data into the header of the recording.
	buf.Reset()
	s.Player.Handler(handler.HandlerIDMovement).(*handler.MovementHandler).Encode(buf)
	f.WriteString(base64.StdEncoding.EncodeToString(buf.Bytes()))
	f.WriteString("\n")

	// Encode the combat data into the header of the recording.
	buf.Reset()
	s.Player.Handler(handler.HandlerIDCombat).(*handler.CombatHandler).Encode(buf)
	f.WriteString(base64.StdEncoding.EncodeToString(buf.Bytes()))
	f.WriteString("\n")

	// Encode all the entities on the player's perspective into the header of the recording.
	buf.Reset()
	for eid, e := range s.Player.Handler(handler.HandlerIDEntities).(*handler.EntitiesHandler).Entities {
		binary.Write(buf, binary.LittleEndian, eid)
		e.Encode(buf)
	}
	f.WriteString(base64.StdEncoding.EncodeToString(buf.Bytes()))
	f.WriteString("\n")

	// Encode all the detections into the recording.
	buf.Reset()
	detections := s.Player.Detections()
	utils.WriteLInt32(buf, int32(len(detections)))
	for _, d := range detections {
		detection.Encode(buf, d)
	}
	f.WriteString(base64.StdEncoding.EncodeToString(buf.Bytes()))
	f.WriteString("\n")

	// Little notice to where the events start.
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

	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer internal.BufferPool.Put(buf)

	b64dec, err = base64.StdEncoding.DecodeString(header[5])
	if err != nil {
		return nil, oerror.New("unable to decode player data: " + err.Error())
	}
	buf.Write(b64dec)

	rec.PlayerData.ClientTick = utils.LInt64(buf.Next(8))
	rec.PlayerData.ClientFrame = utils.LInt64(buf.Next(8))
	rec.PlayerData.ServerTick = utils.LInt64(buf.Next(8))
	rec.PlayerData.Tps = utils.LFloat32(buf.Next(4))
	rec.PlayerData.GameMode = utils.LInt32(buf.Next(4))
	rec.PlayerData.Connected = utils.Bool(buf.Next(1))
	rec.PlayerData.Ready = utils.Bool(buf.Next(1))
	rec.PlayerData.Alive = utils.Bool(buf.Next(1))

	// Decode the movement handler.
	b64dec, err = base64.StdEncoding.DecodeString(header[6])
	if err != nil {
		return nil, oerror.New("unable to decode movement handler: " + err.Error())
	}

	buf.Reset()
	buf.Write(b64dec)
	rec.PlayerData.MovementHandler = handler.DecodeMovementHandler(buf)

	// Decode the combat handler.
	b64dec, err = base64.StdEncoding.DecodeString(header[7])
	if err != nil {
		return nil, oerror.New("unable to decode combat handler: " + err.Error())
	}

	buf.Reset()
	buf.Write(b64dec)
	rec.PlayerData.CombatHandler = handler.DecodeCombatHandler(buf)

	// Decode the entities.
	b64dec, err = base64.StdEncoding.DecodeString(header[8])
	if err != nil {
		return nil, oerror.New("unable to decode entities: " + err.Error())
	}
	rec.Entities = make(map[uint64]*entity.Entity)

	buf.Reset()
	buf.Write(b64dec)
	for buf.Len() > 0 {
		eid := binary.LittleEndian.Uint64(buf.Next(8))
		e := entity.Decode(buf)
		rec.Entities[eid] = e
	}

	// Decode the detections.
	b64dec, err = base64.StdEncoding.DecodeString(header[9])
	if err != nil {
		return nil, oerror.New("unable to decode detections: " + err.Error())
	}

	buf.Reset()
	buf.Write(b64dec)
	detections := int(utils.LInt32(buf.Next(4)))

	for i := 0; i < detections; i++ {
		d := detection.Decode(buf)
		rec.PlayerData.Detections = append(rec.PlayerData.Detections, d)
	}

	// Remove the header from the recording.
	rec.Events, err = event.DecodeEvents([]byte(events))
	if err != nil {
		fmt.Printf("unable to decode events: %v\n", err.Error())
	}

	return rec, nil
}
