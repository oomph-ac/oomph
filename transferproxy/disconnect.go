package proxy

import (
	"errors"
	"net"

	"github.com/sandertv/gophertunnel/minecraft"
)

func backendDisconnectMessage(err error) (string, bool) {
	var disconnect minecraft.DisconnectError
	if errors.As(err, &disconnect) && disconnect != "" {
		return disconnect.Error(), true
	}
	return legacyBackendDisconnectMessage(err)
}

func legacyBackendDisconnectMessage(err error) (string, bool) {
	if opErr, ok := err.(*net.OpError); ok {
		if cause, ok := opErr.Err.(*net.OpError); ok && errors.Is(cause.Err, net.ErrClosed) && playerFacingDisconnectOperation(cause.Op) {
			return cause.Op, true
		}
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, cause := range joined.Unwrap() {
			if message, ok := legacyBackendDisconnectMessage(cause); ok {
				return message, true
			}
		}
		return "", false
	}
	if cause := errors.Unwrap(err); cause != nil {
		return legacyBackendDisconnectMessage(cause)
	}
	return "", false
}

func playerFacingDisconnectOperation(operation string) bool {
	switch operation {
	case "", "accept", "close", "dial", "do spawn", "flush", "read", "read packet", "start game", "write", "write packet":
		return false
	default:
		return true
	}
}
