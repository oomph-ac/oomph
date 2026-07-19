package main

import (
	"slices"
	"strings"
	"testing"
)

func TestLoadModerators(t *testing.T) {
	got, err := loadModerators(strings.NewReader(" Alice \n\nBob\r\nAlice\n"))
	if err != nil {
		t.Fatalf("load moderators: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("moderator count = %d, want 2", len(got))
	}
	for _, name := range []string{"Alice", "Bob"} {
		if _, ok := got[name]; !ok {
			t.Fatalf("moderator %q missing from %#v", name, got)
		}
	}
}

func TestBackendAddresses(t *testing.T) {
	if got, want := backendAddresses("primary:19132", "backup:19132"), []string{"primary:19132", "backup:19132"}; !slices.Equal(got, want) {
		t.Fatalf("backend addresses = %v, want %v", got, want)
	}
	if got, want := backendAddresses("primary:19132", "primary:19132"), []string{"primary:19132"}; !slices.Equal(got, want) {
		t.Fatalf("duplicate backend addresses = %v, want %v", got, want)
	}
}

func TestShutdownTarget(t *testing.T) {
	address, port, ok := shutdownTarget("play.example.com", 19132)
	if !ok || address != "play.example.com" || port != 19132 {
		t.Fatalf("shutdown target = %q, %d, %t", address, port, ok)
	}
	for _, test := range []struct {
		address string
		port    int
	}{
		{"", 19132},
		{"play.example.com", 0},
		{"play.example.com", 65536},
	} {
		if _, _, ok := shutdownTarget(test.address, test.port); ok {
			t.Fatalf("shutdown target %q:%d unexpectedly valid", test.address, test.port)
		}
	}
}
