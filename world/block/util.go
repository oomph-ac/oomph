package block

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}
