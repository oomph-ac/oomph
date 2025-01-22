package component

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"

	"github.com/cespare/xxhash/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/opts"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component/acknowledgement"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

var (
	blobs      = make(map[uint64]blobInfo)
	blobHashes = make([]uint64, 0)
)

func init() {
	if len(opts.Global.APIToken) > 0 {
		// Fetch the identifier blobs from the API
		req, err := http.NewRequest("POST", "https://api.oomph.ac/mcbe/account/blobs", bytes.NewBuffer([]byte{}))
		if err != nil {
			panic(err)
		}
		req.Header.Set("X-Authentication", opts.Global.APIToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}

		if resp.StatusCode != http.StatusOK {
			panic(fmt.Errorf("failed to fetch blobs from API (code %d)", resp.StatusCode))
		}

		var apiBlobs apiIdentifierBlobs
		if err := json.NewDecoder(resp.Body).Decode(&apiBlobs); err != nil {
			panic(err)
		}

		bit := 0
		for _, blob := range apiBlobs.Blobs {
			blobHash := xxhash.Sum64String(blob)
			if _, exists := blobs[blobHash]; exists {
				continue
			}

			blobs[blobHash] = blobInfo{payload: []byte(blob), associatedBit: bit}
			blobHashes = append(blobHashes, blobHash)
			bit++
		}
		logrus.Infof("loaded %d blobs", len(blobs))
	}
}

type IdentifierComponent struct {
	hasTimeout        bool
	stage             byte
	ticksUntilTimeout uint16

	mPlayer         *player.Player
	identity        *big.Int
	pendingIdentity *big.Int
}

func NewIdentifier(p *player.Player) *IdentifierComponent {
	if len(opts.Global.APIToken) == 0 {
		panic("unable to use this component w/o an Oomph API token")
	}

	return &IdentifierComponent{mPlayer: p}
}

func (i *IdentifierComponent) Stage() byte {
	return i.stage
}

func (i *IdentifierComponent) SetStage(stage byte) {
	i.stage = stage
}

func (i *IdentifierComponent) StartTimeout(t uint16) {
	i.hasTimeout = true
	i.ticksUntilTimeout = t
}

func (i *IdentifierComponent) Tick() bool {
	if !i.hasTimeout {
		return true
	}

	i.ticksUntilTimeout--
	return i.ticksUntilTimeout > 0
}

func (i *IdentifierComponent) Identity() *big.Int {
	return i.identity
}

func (i *IdentifierComponent) Request() {
	i.mPlayer.SendPacketToClient(&packet.LevelChunk{
		Position:      protocol.ChunkPos{134217727, 134217727},
		Dimension:     0,
		SubChunkCount: 64,
		CacheEnabled:  true,
		BlobHashes:    blobHashes[:64],
		RawPayload:    []byte{0},
	})
	i.mPlayer.SendPacketToClient(&packet.LevelChunk{
		Position:      protocol.ChunkPos{134217727, 134217727},
		Dimension:     0,
		SubChunkCount: 64,
		CacheEnabled:  true,
		BlobHashes:    blobHashes[64:],
		RawPayload:    []byte{0},
	})
	i.mPlayer.ACKs().Add(acknowledgement.NewIdentifierNextStatusACK(i.mPlayer))
}

func (i *IdentifierComponent) HandleResponse(pk *packet.ClientCacheBlobStatus) {
	if i.stage == player.IdentifierStageComplete {
		return
	}

	i.identity = new(big.Int)
	hasHit, hasMiss := false, false
	for _, miss := range pk.MissHashes {
		if _, blobExists := blobs[miss]; !blobExists {
			hasMiss = true
			break
		}
	}
	for _, hit := range pk.HitHashes {
		if blobInfo, blobExists := blobs[hit]; blobExists {
			i.identity.SetBit(i.identity, blobInfo.associatedBit, 1)
			hasHit = true
		}
	}

	// If at least one hit/miss is present here, we can assume that the ClientCacheBlobStatus packet sent
	// by Oomph is the response to the identifier request.
	if hasMiss || hasHit {
		// Remove the timeout, because we have a response.
		i.hasTimeout = false
		// If the player is missing all the blobs, we can assume that the player does not have an Oomph identity.
		if i.identity.String() == "0" && i.stage == player.IdentifierStageInit {
			i.stage = player.IdentifierStageCreatingNewID
			go i.createID()
		} else {
			if i.stage == player.IdentifierStageCreatingNewID {
				go i.notifyNewID()
			} else {
				go i.validateID()
			}

			i.mPlayer.Message("id=%s", i.identity.String())
			i.stage = player.IdentifierStageComplete
		}
	}
}

func (i *IdentifierComponent) findAlts() {
	host, _, _ := net.SplitHostPort(i.mPlayer.Conn().RemoteAddr().String())
	identity := accountIdentity{
		XUID:     i.mPlayer.IdentityData().XUID,
		Username: i.mPlayer.IdentityData().DisplayName,
		IP:       host,
		HWID:     i.identity.String(),
	}

	enc, _ := json.Marshal(identity)
	req, err := http.NewRequest("POST", "https://api.oomph.ac/mcbe/account/alts", bytes.NewBuffer(enc))
	if err != nil {
		i.mPlayer.Log().Errorf("failed to find alts: %v", err)
		return
	}

	req.Header.Set("X-Authentication", opts.Global.APIToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		i.mPlayer.Log().Errorf("failed to find alts: %v", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		i.mPlayer.Log().Errorf("failed to find alts: %v", string(body))
		return
	}

	var respData []accountIdentity
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		i.mPlayer.Log().Errorf("failed to find alts: %v", err)
		return
	}

	i.mPlayer.Pause()
	defer i.mPlayer.Resume()

	if i.mPlayer.Closed {
		return
	}

	for _, alt := range respData {
		i.mPlayer.Message("alt=%s", alt.Username)
	}
}

func (i *IdentifierComponent) validateID() {
	host, _, _ := net.SplitHostPort(i.mPlayer.Conn().RemoteAddr().String())
	identity := accountIdentity{
		XUID:     i.mPlayer.IdentityData().XUID,
		Username: i.mPlayer.IdentityData().DisplayName,
		IP:       host,
		HWID:     i.identity.String(),
	}

	enc, _ := json.Marshal(identity)
	req, err := http.NewRequest("POST", "https://api.oomph.ac/mcbe/account/validate", bytes.NewBuffer(enc))
	if err != nil {
		i.mPlayer.Log().Errorf("failed to validate Oomph identity: %v", err)
		return
	}

	req.Header.Set("X-Authentication", opts.Global.APIToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		i.mPlayer.Log().Errorf("failed to validate Oomph identity: %v", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		i.mPlayer.Log().Errorf("failed to validate Oomph identity (code %d): %v", resp.StatusCode, string(body))
		return
	}

	var respData accountValidateIdentityResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		i.mPlayer.Log().Errorf("failed to validate Oomph identity: %v", err)
		return
	}

	i.mPlayer.Pause()
	defer i.mPlayer.Resume()

	if i.mPlayer.Closed {
		return
	} else if !respData.Valid {
		i.mPlayer.Disconnect(game.ErrorInvalidIdentity)
	} else {
		go i.findAlts()
	}
}

func (i *IdentifierComponent) notifyNewID() {
	host, _, _ := net.SplitHostPort(i.mPlayer.Conn().RemoteAddr().String())
	identity := accountIdentity{
		XUID:     i.mPlayer.IdentityData().XUID,
		Username: i.mPlayer.IdentityData().DisplayName,
		IP:       host,
		HWID:     i.identity.String(),
	}

	enc, _ := json.Marshal(identity)
	req, err := http.NewRequest("POST", "https://api.oomph.ac/mcbe/account/create", bytes.NewBuffer(enc))
	if err != nil {
		i.mPlayer.Log().Errorf("failed to create new Oomph identity: %v", err)
		return
	}

	req.Header.Set("X-Authentication", opts.Global.APIToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		i.mPlayer.Log().Errorf("failed to create new Oomph identity: %v", err)
		return
	}

	i.mPlayer.Pause()
	defer i.mPlayer.Resume()

	if resp.StatusCode != http.StatusOK && !i.mPlayer.Closed {
		body, _ := io.ReadAll(resp.Body)
		i.mPlayer.Log().Errorf("failed to create new Oomph identity: %v", string(body))
	}
	go i.findAlts()
}

func (i *IdentifierComponent) createID() {
	host, _, _ := net.SplitHostPort(i.mPlayer.Conn().RemoteAddr().String())
	identity := accountIdentity{
		XUID:     i.mPlayer.IdentityData().XUID,
		Username: i.mPlayer.IdentityData().DisplayName,
		IP:       host,
	}

	enc, _ := json.Marshal(identity)
	req, err := http.NewRequest("POST", "https://api.oomph.ac/mcbe/account/create", bytes.NewBuffer(enc))
	if err != nil {
		i.mPlayer.Log().Errorf("failed to create new Oomph identity: %v", err)
		return
	}

	req.Header.Set("X-Authentication", opts.Global.APIToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		i.mPlayer.Log().Errorf("failed to create new Oomph identity: %v", err)
		return
	}

	var respData accountCreateIdentityResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		i.mPlayer.Log().Errorf("failed to create new Oomph identity: %v", err)
		return
	}

	i.mPlayer.Pause()
	defer i.mPlayer.Resume()
	if i.mPlayer.Closed {
		return
	}

	i.pendingIdentity = new(big.Int)
	i.pendingIdentity.SetString(respData.HWID, 10)

	// Send a miss cache response to the player so that they can store the blobs required by the pending identity.
	sendingBlobs := make([]protocol.CacheBlob, 0)
	expectedHits := 0
	for blobHash, blobInfo := range blobs {
		if i.pendingIdentity.Bit(blobInfo.associatedBit) == 1 {
			expectedHits++
			sendingBlobs = append(sendingBlobs, protocol.CacheBlob{Hash: blobHash, Payload: blobInfo.payload})
		}
	}
	i.mPlayer.SendPacketToClient(&packet.ClientCacheMissResponse{
		Blobs: sendingBlobs,
	})
	i.Request()
}

type apiIdentifierBlobs struct {
	Blobs []string `json:"blobs"`
}

type blobInfo struct {
	payload       []byte
	associatedBit int
}

type accountIdentity struct {
	// XUID is the XBOX-Live ID of the player.
	XUID string `json:"xuid"`
	// Username is the IGN of the player.
	Username string `json:"username"`
	// IP is the IP address of the player. This should be encrypted by a key once it is recieved by the server.
	IP string `json:"ip_address"`
	// HWID, if present, is the HWID that the Oomph has assigned to the player.
	HWID string `json:"hwid"`
}

type accountCreateIdentityResponse struct {
	// HWID is the new identity of the player that should be sent.
	HWID string `json:"hwid"`
	// Inserted is a boolean that indicates wether or not the new identity for the player has been
	// inserted into the API's database.
	Inserted bool `json:"inserted"`
}

type accountValidateIdentityResponse struct {
	// Valid is a boolean that indicates wether or not the identity is valid.
	Valid bool `json:"valid"`
}
