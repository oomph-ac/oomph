package internal

import (
	"io"
	"unsafe"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type writerOutput interface {
	io.Writer
	io.ByteWriter
}

type writerWrapper struct {
	w   writerOutput
	sID int32
}

func ModifyWriterOutput(w *protocol.Writer, newOut writerOutput) {
	wr := (*writerWrapper)(unsafe.Pointer(w))
	wr.w = newOut
}
