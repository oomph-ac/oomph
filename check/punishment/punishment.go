package punishment

// Punishment specifies a variant of punishment that should be carried out for a detection.
type Punishment struct {
	punishment
}

type punishment string

// None will do nothing if the player reaches the max violations of a check.
func None() Punishment {
	return Punishment{"none"}
}

// Kick will kick the player if the player reaches the max violations of a check.
func Kick() Punishment {
	return Punishment{"kick"}
}

// Ban will ban the player if the player reaches the max violations of a check.
func Ban() Punishment {
	return Punishment{"ban"}
}
