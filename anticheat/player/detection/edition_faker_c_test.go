package detection

import (
	"log/slog"
	"testing"
	"time"

	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestEditionFakerCMobileClassificationUsesCurrentClientData(t *testing.T) {
	p := player.New(slog.Default(), player.MonitoringState{IsReplay: true, CurrentTime: time.Now()}, nil)
	d := New_EditionFakerC(p)
	p.ClientDat.DeviceOS = protocol.DeviceAndroid

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Android touch input was treated as non-mobile: %v", recovered)
		}
	}()
	d.Detect(&packet.PlayerAuthInput{InputMode: packet.InputModeTouch})
}
