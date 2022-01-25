package check

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Timer struct{}

func (Timer) Name() (string, string) {
	return "Timer", "A"
}

func (Timer) Description() string {
	return "This checks if the player is sending movement packets too fast."
}

func (Timer) Punishment() Punishment {
	return PunishmentBan()
}

func (Timer) Violations() uint32 {
	return 15
}

func (Timer) Process(pk packet.Packet) {
	if _, ok := pk.(*packet.PlayerAuthInput); ok {
		// todo
	}
}
