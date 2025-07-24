package utils

import (
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ethaniccc/float32-cube/cube"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/go-raknet"
	"github.com/sandertv/gophertunnel/minecraft"
)

type security struct {
	conf raknet.ListenConfig

	blockCount atomic.Uint32

	mu     sync.Mutex
	blocks map[[16]byte]time.Time
}

// updatePrivateField sets a private field of a session to the value passed.
func updatePrivateField[T any](v any, name string, value T) {
	reflectedValue := reflect.ValueOf(v).Elem()
	privateFieldValue := reflectedValue.FieldByName(name)

	privateFieldValue = reflect.NewAt(privateFieldValue.Type(), unsafe.Pointer(privateFieldValue.UnsafeAddr())).Elem()

	privateFieldValue.Set(reflect.ValueOf(value))
}

// fetchPrivateField fetches a private field of a session.
func fetchPrivateField[T any](s any, name string) T {
	reflectedValue := reflect.ValueOf(s).Elem()
	privateFieldValue := reflectedValue.FieldByName(name)
	privateFieldValue = reflect.NewAt(privateFieldValue.Type(), unsafe.Pointer(privateFieldValue.UnsafeAddr())).Elem()

	return privateFieldValue.Interface().(T)
}

func GetRaknetListener(l *minecraft.Listener) (*raknet.Listener, bool) {
	listener := fetchPrivateField[minecraft.NetworkListener](l, "listener")
	rkListener, ok := listener.(*raknet.Listener)
	return rkListener, ok
}

func unsafeCast[T any](s any) *T {
	v := reflect.ValueOf(s)
	if v.Kind() != reflect.Ptr {
		return (*T)(unsafe.Pointer(&s))
	}
	return (*T)(unsafe.Pointer(v.Pointer()))
}

func getRaknetSecurity(l *raknet.Listener) *security {
	sec := fetchPrivateField[any](l, "sec")
	t := unsafeCast[security](sec)
	return t
}

func BlockAddress(l *raknet.Listener, addr net.IP, duration time.Duration) {
	sec := getRaknetSecurity(l)
	sec.mu.Lock()
	defer sec.mu.Unlock()
	sec.blockCount.Add(1)
	sec.blocks[[16]byte(addr.To16())] = time.Now().Add(duration)
}

func ModifyBBoxResult(hitResult *trace.BBoxResult, bb cube.BBox, pos mgl32.Vec3, face cube.Face) {
	type bboxResultWrapper struct {
		bbox cube.BBox
		pos  mgl32.Vec3
		face cube.Face
	}

	if hitResult == nil {
		return
	}
	hitResultWrapper := (*bboxResultWrapper)(unsafe.Pointer(hitResult))
	hitResultWrapper.bbox = bb
	hitResultWrapper.pos = pos
	hitResultWrapper.face = face
}
