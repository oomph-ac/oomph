package utils

// HasFlag returns whether given flags include the given bitflag.
func HasFlag(flags uint64, flag uint32) bool {
	return flags&(1<<flag) > 0
}
