package proxy

import (
	"math"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (s *session) rewriteClientPacket(pk packet.Packet) {
	from, to := s.clientRuntimeID, s.backendRuntimeID
	switch pk := pk.(type) {
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.MobEquipment:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.Animate:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.PlayerAction:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.Respawn:
		if pk.EntityRuntimeID == from {
			pk.EntityRuntimeID = to
		}
	case *packet.Interact:
		switch pk.TargetEntityRuntimeID {
		case math.MaxInt64:
			pk.TargetEntityRuntimeID = from
		case from:
			pk.TargetEntityRuntimeID = to
		}
	case *packet.InventoryTransaction:
		if tx, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
			switch tx.TargetEntityRuntimeID {
			case math.MaxInt64:
				tx.TargetEntityRuntimeID = from
			case from:
				tx.TargetEntityRuntimeID = to
			}
		}
	case *packet.ContainerOpen:
		if pk.ContainerEntityUniqueID == s.clientUniqueID {
			pk.ContainerEntityUniqueID = s.backendUniqueID
		}
	}
}

func (s *session) rewriteServerPacket(pk packet.Packet) bool {
	from, to := s.backendRuntimeID, s.clientRuntimeID
	rewriteRuntimeID := func(id *uint64) {
		if *id == from {
			*id = to
		} else if from != to && *id == to {
			*id = math.MaxInt64
		}
	}
	rewriteUniqueID := func(id *int64) {
		if *id == s.backendUniqueID {
			*id = s.clientUniqueID
		} else if s.backendUniqueID != s.clientUniqueID && *id == s.clientUniqueID {
			*id = math.MaxInt64
		}
	}
	switch pk := pk.(type) {
	case *packet.AddPlayer:
		if pk.EntityRuntimeID == from {
			return false
		}
		rewriteRuntimeID(&pk.EntityRuntimeID)
		rewriteUniqueID(&pk.AbilityData.EntityUniqueID)
	case *packet.AddActor:
		if pk.EntityRuntimeID == from {
			return false
		}
		rewriteRuntimeID(&pk.EntityRuntimeID)
		rewriteUniqueID(&pk.EntityUniqueID)
	case *packet.AddItemActor:
		rewriteRuntimeID(&pk.EntityRuntimeID)
		rewriteUniqueID(&pk.EntityUniqueID)
	case *packet.AddPainting:
		rewriteRuntimeID(&pk.EntityRuntimeID)
		rewriteUniqueID(&pk.EntityUniqueID)
	case *packet.MoveActorAbsolute:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.MovePlayer:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.MobEquipment:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.Animate:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.ActorEvent:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.PlayerAction:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.SetActorData:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.SetActorMotion:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.UpdateAttributes:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.MobEffect:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.Respawn:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.MobArmourEquipment:
		rewriteRuntimeID(&pk.EntityRuntimeID)
	case *packet.UpdateAbilities:
		rewriteUniqueID(&pk.AbilityData.EntityUniqueID)
	case *packet.ContainerOpen:
		rewriteUniqueID(&pk.ContainerEntityUniqueID)
	case *packet.RemoveActor:
		rewriteUniqueID(&pk.EntityUniqueID)
	}
	return true
}
