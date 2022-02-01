package utils

// HasFlag returns whether given flags include the given bitflag.
func HasFlag(flags uint64, flag uint64) bool {
	return flags&flag > 0
}
