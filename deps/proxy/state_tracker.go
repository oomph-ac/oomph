package proxy

import (
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// backendStateTracker records client-visible state that survives a dimension
// change and must therefore be explicitly removed before another backend takes
// ownership of the session.
type backendStateTracker struct {
	bossBars    map[int64]struct{}
	effects     map[int32]struct{}
	entities    map[int64]struct{}
	players     map[uuid.UUID]struct{}
	scoreboards map[string]struct{}
}

func newBackendStateTracker() *backendStateTracker {
	return &backendStateTracker{
		bossBars:    map[int64]struct{}{},
		effects:     map[int32]struct{}{},
		entities:    map[int64]struct{}{},
		players:     map[uuid.UUID]struct{}{},
		scoreboards: map[string]struct{}{},
	}
}

func (t *backendStateTracker) handle(pk packet.Packet, clientRuntimeID uint64) {
	switch pk := pk.(type) {
	case *packet.AddActor:
		t.entities[pk.EntityUniqueID] = struct{}{}
	case *packet.AddItemActor:
		t.entities[pk.EntityUniqueID] = struct{}{}
	case *packet.AddPainting:
		t.entities[pk.EntityUniqueID] = struct{}{}
	case *packet.AddPlayer:
		t.entities[pk.AbilityData.EntityUniqueID] = struct{}{}
	case *packet.RemoveActor:
		delete(t.entities, pk.EntityUniqueID)
	case *packet.BossEvent:
		if pk.EventType == packet.BossEventHide {
			delete(t.bossBars, pk.BossEntityUniqueID)
		} else {
			t.bossBars[pk.BossEntityUniqueID] = struct{}{}
		}
	case *packet.MobEffect:
		if pk.EntityRuntimeID != clientRuntimeID {
			break
		}
		if pk.Operation == packet.MobEffectAdd || pk.Operation == packet.MobEffectModify {
			t.effects[pk.EffectType] = struct{}{}
		} else if pk.Operation == packet.MobEffectRemove {
			delete(t.effects, pk.EffectType)
		}
	case *packet.PlayerList:
		for _, entry := range pk.Entries {
			if pk.ActionType == packet.PlayerListActionAdd {
				t.players[entry.UUID] = struct{}{}
			} else {
				delete(t.players, entry.UUID)
			}
		}
	case *packet.SetDisplayObjective:
		t.scoreboards[pk.ObjectiveName] = struct{}{}
	case *packet.RemoveObjective:
		delete(t.scoreboards, pk.ObjectiveName)
	}
}

func (t *backendStateTracker) clearPackets(clientRuntimeID uint64) []packet.Packet {
	packets := make([]packet.Packet, 0, len(t.entities)+len(t.bossBars)+len(t.effects)+len(t.scoreboards)+1)
	for id := range t.entities {
		packets = append(packets, &packet.RemoveActor{EntityUniqueID: id})
	}
	for id := range t.bossBars {
		packets = append(packets, &packet.BossEvent{BossEntityUniqueID: id, EventType: packet.BossEventHide})
	}
	for id := range t.effects {
		packets = append(packets, &packet.MobEffect{EntityRuntimeID: clientRuntimeID, EffectType: id, Operation: packet.MobEffectRemove})
	}
	if len(t.players) != 0 {
		entries := make([]protocol.PlayerListEntry, 0, len(t.players))
		for id := range t.players {
			entries = append(entries, protocol.PlayerListEntry{UUID: id})
		}
		packets = append(packets, &packet.PlayerList{ActionType: packet.PlayerListActionRemove, Entries: entries})
	}
	for objective := range t.scoreboards {
		packets = append(packets, &packet.RemoveObjective{ObjectiveName: objective})
	}
	clear(t.entities)
	clear(t.bossBars)
	clear(t.effects)
	clear(t.players)
	clear(t.scoreboards)
	return packets
}
