package utils

import (
	"sync/atomic"
	"unsafe"

	"github.com/sandertv/gophertunnel/minecraft"
)

type connWrapper struct {
	padding0 [1328]byte
	shieldID atomic.Int32
	padding1 [8]byte
}

func ConnShieldID(c *minecraft.Conn) int32 {
	return ((*connWrapper)(unsafe.Pointer(c))).shieldID.Load()
}
