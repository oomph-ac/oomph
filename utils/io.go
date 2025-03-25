package utils

import "io"

type ByteWritterWrapper struct {
	io.Writer
}

func NewByteWritterWrapper(w io.Writer) *ByteWritterWrapper {
	return &ByteWritterWrapper{w}
}

func (bw *ByteWritterWrapper) WriteByte(b byte) error {
	_, err := bw.Write([]byte{b})
	return err
}
