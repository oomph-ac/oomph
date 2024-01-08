package utils

// HasFlag returns whether given flags include the given bitflag.
func HasFlag(flags uint64, flag uint64) bool {
	return flags&flag > 0
}

// HasDataFlag checks if the given flag includes the given data.
func HasDataFlag(flag uint64, data int64) bool {
	return (data & (1 << (flag % 64))) > 0
}
