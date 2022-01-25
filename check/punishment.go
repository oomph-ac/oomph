package check

type Punishment struct {
	punishment
}

type punishment uint8

// PunishmentNone will do nothing if the player reaches the max violations of a check.
func PunishmentNone() Punishment {
	return Punishment{0}
}

// PunishmentKick will kick the player if the player reaches the max violations of a check.
func PunishmentKick() Punishment {
	return Punishment{1}
}

// PunishmentBan will ban the player if the player reaches the max violations of a check.
func PunishmentBan() Punishment {
	return Punishment{2}
}
