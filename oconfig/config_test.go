package oconfig

import "testing"

func TestDefaultConfigHasCurrentVersion(t *testing.T) {
	if DefaultConfig.Version != ConfigVersion {
		t.Fatalf("DefaultConfig.Version = %d, want %d", DefaultConfig.Version, ConfigVersion)
	}
	if DefaultConfig.Prefix == "" {
		t.Fatal("DefaultConfig.Prefix must not be empty")
	}
	if len(DefaultConfig.Detections) == 0 {
		t.Fatal("DefaultConfig.Detections must not be empty")
	}
}
