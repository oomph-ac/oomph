package component

import "testing"

func TestJumpingIncludesHeldJumpState(t *testing.T) {
	movement := &AuthoritativeMovementComponent{pressingJump: true}
	if !movement.Jumping() {
		t.Fatal("expected held jump state to request a jump")
	}
}
