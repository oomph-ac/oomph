package player

import "time"

type MonitoringState struct {
	// IsReplay is set to true if the current player's actions are being replayed.
	IsReplay bool
	// IsRecording is set to true if the current player's actions are being recorded.
	// IsReplay must be set to false if this is set to true.
	IsRecording bool
	// CurrentTime is the current time of the player. This is used primarily for support of replays.
	CurrentTime time.Time
}
