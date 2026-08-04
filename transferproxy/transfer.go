package proxy

import (
	"context"
	"fmt"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	initialFallbackRetryDelay = 100 * time.Millisecond
	maximumFallbackRetryDelay = 2 * time.Second
)

func fallbackRetryDelay(consecutiveFailures int) time.Duration {
	delay := initialFallbackRetryDelay
	for i := 1; i < consecutiveFailures && delay < maximumFallbackRetryDelay; i++ {
		delay = min(delay*2, maximumFallbackRetryDelay)
	}
	return delay
}

func waitForFallbackRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *session) transfer(ctx context.Context, address string) (bool, error) {
	s.transferMu.Lock()
	defer s.transferMu.Unlock()
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	return s.transferLocked(ctx, address)
}

func (s *session) recoverFallback(ctx context.Context, consecutiveFailures int) (bool, error) {
	s.transferMu.Lock()
	defer s.transferMu.Unlock()
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	if consecutiveFailures > 0 {
		if err := waitForFallbackRetry(ctx, fallbackRetryDelay(consecutiveFailures)); err != nil {
			return false, err
		}
	}
	return s.transferLocked(ctx, s.proxy.cfg.RemoteAddress)
}

func (s *session) transferLocked(ctx context.Context, address string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	backend, err := s.proxy.cfg.Dial(ctx, address, s.identity, s.clientData, s.clientAddress)
	if err != nil {
		return false, err
	}
	backendUsesHashes := backend.GameData().UseBlockNetworkIDHashes
	if backendUsesHashes != s.blockNetworkIDHashes {
		_ = backend.Close()
		return false, fmt.Errorf("proxy: backend block-hash setting %t does not match session setting %t", backendUsesHashes, s.blockNetworkIDHashes)
	}
	if err := backend.DoSpawn(); err != nil {
		_ = backend.Close()
		return false, err
	}

	if err := s.handler.TransferBackend(backend); err != nil {
		_ = backend.Close()
		return false, err
	}
	old := s.swapBackend(backend)
	err = s.resetTransferState()
	_ = old.Close()
	return true, err
}

func (s *session) resetTransferState() error {
	for _, pk := range s.state.clearPackets(s.clientRuntimeID) {
		if err := s.client.WritePacket(pk); err != nil {
			return err
		}
	}
	data := s.backend.GameData()
	for _, pk := range transferResetPackets(s.clientDimension, data) {
		s.rewriteServerPacket(pk)
		if err := s.client.WritePacket(pk); err != nil {
			return err
		}
	}
	s.clientDimension = data.Dimension
	radius := s.chunkRadius
	if provider, ok := s.handler.(ChunkRadiusProvider); ok {
		radius = provider.ChunkRadius()
	}
	radius = min(max(radius, 1), 255)
	if err := s.backend.WritePacket(&packet.RequestChunkRadius{ChunkRadius: radius, MaxChunkRadius: uint8(radius)}); err != nil {
		return err
	}
	return s.backend.Flush()
}

func transferResetPackets(currentDimension int32, data minecraft.GameData) []packet.Packet {
	fakeDimension := int32(packet.DimensionOverworld)
	for _, candidate := range []int32{packet.DimensionOverworld, packet.DimensionNether, packet.DimensionEnd} {
		if candidate != currentDimension && candidate != data.Dimension {
			fakeDimension = candidate
			break
		}
	}
	packets := make([]packet.Packet, 0, 12)
	packets = append(packets,
		&packet.StopSound{StopAll: true},
		&packet.LevelEvent{EventType: packet.LevelEventStopRaining, EventData: 10_000},
		&packet.LevelEvent{EventType: packet.LevelEventStopThunderstorm},
	)
	if currentDimension == data.Dimension {
		packets = append(packets,
			&packet.ChangeDimension{Dimension: fakeDimension, Position: data.PlayerPosition},
			&packet.PlayStatus{Status: packet.PlayStatusPlayerSpawn},
			&packet.PlayerAction{EntityRuntimeID: data.EntityRuntimeID, ActionType: protocol.PlayerActionDimensionChangeDone},
		)
	}
	packets = append(packets,
		&packet.ChangeDimension{Dimension: data.Dimension, Position: data.PlayerPosition},
		&packet.PlayStatus{Status: packet.PlayStatusPlayerSpawn},
		&packet.PlayerAction{EntityRuntimeID: data.EntityRuntimeID, ActionType: protocol.PlayerActionDimensionChangeDone},
		&packet.SetPlayerGameType{GameType: data.PlayerGameMode},
		&packet.SetDifficulty{Difficulty: uint32(data.Difficulty)},
		&packet.GameRulesChanged{GameRules: data.GameRules},
		&packet.MovePlayer{EntityRuntimeID: data.EntityRuntimeID, Position: data.PlayerPosition, Pitch: data.Pitch, Yaw: data.Yaw, HeadYaw: data.Yaw, Mode: packet.MoveModeReset},
	)
	return packets
}
