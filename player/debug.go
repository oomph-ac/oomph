package player

type Debugger struct {
	LogLatency  bool
	LogCombat   bool
	LogMovement bool

	UseServerKnockback bool
	UsePacketBuffer    bool
}
