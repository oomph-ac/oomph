package utils

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/chewxy/math32"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/oerror"
)

// HasFlag returns whether given flags include the given bitflag.
func HasFlag(flags uint64, flag uint64) bool {
	return flags&flag > 0
}

func AddFlag[T uint64 | int64](flags T, flag T) T {
	return flags | (1 << flag)
}

func RemoveFlag[T uint64 | int64](flags T, flag T) T {
	return flags &^ (1 << flag)
}

// HasDataFlag checks if the given flag includes the given data.
func HasDataFlag(flag uint64, data int64) bool {
	return (data & (1 << (flag % 64))) > 0
}

// RemoveDataFlag removes the specified flag from the flags.
func RemoveDataFlag(flags int64, flag uint64) int64 {
	return flags &^ (1 << (flag % 64))
}

// WriteLInt32 writes a 32-bit integer to the given bytes.
func WriteLInt32(b *bytes.Buffer, v int32) {
	_ = b.WriteByte(byte(v))
	_ = b.WriteByte(byte(v >> 8))
	_ = b.WriteByte(byte(v >> 16))
	_ = b.WriteByte(byte(v >> 24))
}

// LInt32 returns a 32-bit integer from the given bytes.
func LInt32(b []byte) int32 {
	_ = b[3]
	return int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16 | int32(b[3])<<24
}

// WriteLInt64 writes a 64-bit integer to the given bytes.
func WriteLInt64(b *bytes.Buffer, v int64) {
	_ = b.WriteByte(byte(v))
	_ = b.WriteByte(byte(v >> 8))
	_ = b.WriteByte(byte(v >> 16))
	_ = b.WriteByte(byte(v >> 24))
	_ = b.WriteByte(byte(v >> 32))
	_ = b.WriteByte(byte(v >> 40))
	_ = b.WriteByte(byte(v >> 48))
	_ = b.WriteByte(byte(v >> 56))
}

// LInt64 returns a 64-bit integer from the given bytes.
func LInt64(b []byte) int64 {
	return int64(b[0]) | int64(b[1])<<8 | int64(b[2])<<16 | int64(b[3])<<24 | int64(b[4])<<32 | int64(b[5])<<40 | int64(b[6])<<48 | int64(b[7])<<56
}

// WriteLFloat32 writes a 32-bit float to the given bytes.
func WriteLFloat32(b *bytes.Buffer, v float32) {
	bits := math32.Float32bits(v)
	binary.Write(b, binary.LittleEndian, bits)
}

// LFloat32 returns a 32-bit float from the given bytes.
func LFloat32(b []byte) float32 {
	bits := binary.LittleEndian.Uint32(b)
	return math32.Float32frombits(bits)
}

// WriteVec32 writes a 32-bit vector to the given bytes.
func WriteVec32(b *bytes.Buffer, v mgl32.Vec3) {
	WriteLFloat32(b, v[0])
	WriteLFloat32(b, v[1])
	WriteLFloat32(b, v[2])
}

// ReadVec32 returns a 32-bit vector from the given bytes.
func ReadVec32(b []byte) mgl32.Vec3 {
	return mgl32.Vec3{
		LFloat32(b[0:4]),
		LFloat32(b[4:8]),
		LFloat32(b[8:12]),
	}
}

// WriteLFloat64 writes a 64-bit float to the given bytes.
func WriteLFloat64(b *bytes.Buffer, v float64) {
	bits := math.Float64bits(v)
	binary.Write(b, binary.LittleEndian, bits)
}

// LFloat64 returns a 64-bit float from the given bytes.
func LFloat64(b []byte) float64 {
	bits := binary.LittleEndian.Uint64(b)
	return math.Float64frombits(bits)
}

// WriteBool writes a boolean to the given bytes.
func WriteBool(b *bytes.Buffer, v bool) {
	if v {
		b.WriteByte(1)
		return
	}

	b.WriteByte(0)
}

// Bool returns a boolean from the given bytes.
func Bool(b []byte) bool {
	v := b[0]
	if v == 0 {
		return false
	} else if v == 1 {
		return true
	} else {
		panic(oerror.New("unexpected non-boolean value in byte buffer %v", v))
	}
}
