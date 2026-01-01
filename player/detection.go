package player

import (
	"fmt"
	"math"
	"time"

	"github.com/df-mc/dragonfly/server/event"
	"github.com/oomph-ac/oconfig"
	oevent "github.com/oomph-ac/oomph/player/event"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
)

var DefaultDetectionDisconnectMessage = text.Colourf("<bold><red>You've been removed from the server for cheating.</red></bold>")

type Detection interface {
	// Type returns the primary type of the detection. E.G - "Reach", "KillAura", etc.
	Type() string
	// SubType returns the sub-type of the detection. This is mainly a letter or a number representing a
	// detection for the same cheat defined in Type(), but with a different method.
	SubType() string
	// Description returns the description of what the detection does.
	Description() string
	// Punishable returns true if the detection should trigger a punishment.
	Punishable() bool
	// Metadata returns the initial metadata that should be registered for a detection.
	Metadata() *DetectionMetadata

	// Detect lets the detection handle a packet for any suspicious behavior that might flag it.
	Detect(pk packet.Packet)
}

type DetectionMetadata struct {
	Violations    float64
	MaxViolations float64

	Buffer     float64
	FailBuffer float64
	MaxBuffer  float64

	TrustDuration int64
	LastFlagged   int64
}

func (p *Player) PassDetection(d Detection, sub float64) {
	m := d.Metadata()
	m.Buffer = math.Max(0, m.Buffer-sub)
}

func (p *Player) FailDetection(d Detection, extraData ...any) {
	// Ensure kvs are in pairs to avoid misaligned formatting.
	if len(extraData)%2 != 0 {
		extraData = extraData[:len(extraData)-1]
	}
	// Append latency consistently to all failure logs/events.
	extraData = append(extraData, "latency", fmt.Sprintf("%dms", p.StackLatency.Milliseconds()))

	m := d.Metadata()
	m.Buffer = math.Min(m.Buffer+1.0, m.MaxBuffer)
	if m.Buffer < m.FailBuffer {
		return
	}

	oldVl := m.Violations
	if m.TrustDuration > 0 {
		m.Violations += math.Max(0, float64(m.TrustDuration)-float64(p.ServerTick-m.LastFlagged)) / float64(m.TrustDuration)
	} else {
		m.Violations++
	}

	ctx := event.C(p)
	p.EventHandler().HandleFlag(ctx, d, extraData)
	if ctx.Cancelled() {
		m.Violations = oldVl
		return
	}

	m.LastFlagged = p.ServerTick
	if m.Violations >= 0.01 {
		extraDatString := utils.KeyValsToString(extraData)
		if oconfig.Global.UseLegacyEvents {
			p.SendRemoteEvent(oevent.NewFlaggedEvent(
				p.IdentityDat.DisplayName,
				d.Type(),
				d.SubType(),
				float32(m.Violations),
				extraDatString,
			))
		}
		p.Log().Warn(
			"failed detection",
			"ign", p.IdentityDat.DisplayName,
			"dtc", d.Type(),
			"subDtc", d.SubType(),
			"vl", m.Violations,
			"data", extraDatString,
		)
	}

	if !oconfig.Global.UseLegacyEvents && d.Punishable() && m.Violations >= m.MaxViolations {
		ctx = event.C(p)
		message := DefaultDetectionDisconnectMessage
		p.EventHandler().HandlePunishment(ctx, d, &message)
		if ctx.Cancelled() {
			return
		}

		p.Log().Warn(
			"punishment issued",
			"ign", p.IdentityDat.DisplayName,
			"dtc", d.Type(),
			"subDtc", d.SubType(),
			"message", message,
		)
		p.Disconnect(message)
		p.Close()
	}
}

func (p *Player) ReceiveAlert(alertMsg string) {
	if !p.ReceiveAlerts || time.Since(p.LastAlert) < p.AlertDelay {
		return
	}
	p.LastAlert = time.Now()
	p.RawMessage(alertMsg)
}
