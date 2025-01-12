package player

import (
	"fmt"
	"math"
	"strings"

	"github.com/df-mc/dragonfly/server/event"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	oevent "github.com/oomph-ac/oomph/player/event"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
)

var DETECTION_DEFAULT_KICK_MESSAGE = text.Colourf(strings.Join([]string{
	"<red><bold>Cheating Detected</bold></red>",
	"<red>We've identified suspicious behavior from your gameplay</red>",
	"<red>and removed you from the server.</red>",
	"<purple><italic>Cheating ruins the fun of the game for other players.</italic></purple>",
	"<yellow>Read our Fair Play Policy @ github.com/oomph-ac/fpp</yellow>",
}, "\n"))

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
	// Metadata returns the inital metadata that should be registered for a detection.
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

	// Mitigation is true if the detection is simply meant to notify the remote server if there
	// was a certain action that was mitigated (for combat and movement). For instance, a detection
	// that would be considered a mitigation would be Velocity, as all movement is full-authoritative
	// anyway, and no other player would be able to see the movement cheat in action.
	Mitigation bool
}

func (p *Player) PassDetection(d Detection, sub float64) {
	m := d.Metadata()
	m.Buffer = math.Max(0, m.Buffer-sub)
}

func (p *Player) FailDetection(d Detection, extraData *orderedmap.OrderedMap[string, any]) {
	if extraData == nil {
		extraData = orderedmap.NewOrderedMap[string, any]()
	}
	extraData.Set("latency", fmt.Sprintf("%dms", p.StackLatency.Milliseconds()))

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

	ctx := event.C()
	p.EventHandler().HandleFlag(ctx, d, extraData)
	if ctx.Cancelled() {
		m.Violations = oldVl
		return
	}

	m.LastFlagged = p.ServerTick
	if m.Violations >= 0.5 {
		extraDatString := utils.OrderedMapToString(*extraData)
		if !m.Mitigation {
			p.SendRemoteEvent(oevent.NewFlaggedEvent(
				p.IdentityDat.DisplayName,
				d.Type(),
				d.SubType(),
				float32(m.Violations),
				extraDatString,
			))
			p.Log().Warnf("%s flagged %s (%s) <x%f> %s", p.IdentityDat.DisplayName, d.Type(), d.SubType(), game.Round64(m.Violations, 2), extraDatString)
		} else {
			p.SendRemoteEvent(oevent.NewMitigationEvent(
				d.Type(),
				d.SubType(),
				extraDatString,
				m.Violations,
			))
			p.Log().Warnf("%s was mitigated for %s (%s) <%.2f> %s", p.Name(), d.Type(), d.SubType(), m.Violations, extraDatString)
		}
	}

	if d.Punishable() && m.Violations >= m.MaxViolations {
		ctx = event.C()
		message := DETECTION_DEFAULT_KICK_MESSAGE
		p.EventHandler().HandlePunishment(ctx, d, &message)
		if ctx.Cancelled() {
			return
		}

		p.Log().Warnf("%s was removed from the server for usage of third-party modifications (%s-%s).", p.IdentityDat.DisplayName, d.Type(), d.SubType())
		p.Disconnect(message)
		p.Close()
	}
}
