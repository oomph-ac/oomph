package punishment

// Punishment specifies a variant of punishment that should be carried out for a detection.
type Punishment struct {
	punishment
}

type punishment uint8

// None will do nothing if the player reaches the max violations of a check.
func None() Punishment {
	return Punishment{0}
}

// Kick will kick the player if the player reaches the max violations of a check.
func Kick() Punishment {
	return Punishment{1}
}

// Ban will ban the player if the player reaches the max violations of a check.
func Ban() Punishment {
	return Punishment{2}
}
