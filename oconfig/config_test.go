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

func TestGlobalStartsWithIndependentDefaults(t *testing.T) {
	if Global.Network.MaxACKTimeout != DefaultConfig.Network.MaxACKTimeout {
		t.Fatalf("Global.Network.MaxACKTimeout = %d, want default %d", Global.Network.MaxACKTimeout, DefaultConfig.Network.MaxACKTimeout)
	}

	defaultReach := DefaultConfig.Detections["Reach_A"]
	Global.Detections["Reach_A"] = Detection{MaxVl: 999}
	t.Cleanup(func() { Global.Detections["Reach_A"] = defaultReach })
	if got := DefaultConfig.Detections["Reach_A"]; got != defaultReach {
		t.Fatalf("Global.Detections shares DefaultConfig.Detections: got %#v, want %#v", got, defaultReach)
	}
}

func TestDefaultCombatValidatorOptions(t *testing.T) {
	combat := DefaultConfig.Combat
	if combat.DisableFullAuthoritative {
		t.Fatal("DisableFullAuthoritative must default to false")
	}
	if combat.DisableBlockOcclusionCheck {
		t.Fatal("DisableBlockOcclusionCheck must default to false")
	}
	if combat.RawDistanceFallback {
		t.Fatal("RawDistanceFallback must default to false")
	}
	if combat.BBoxExpansion != 0.1 {
		t.Fatalf("BBoxExpansion = %v, want 0.1", combat.BBoxExpansion)
	}
	if combat.MaximumReach != 2.9 {
		t.Fatalf("MaximumReach = %v, want 2.9", combat.MaximumReach)
	}
	if combat.ReachLeniency != 0 {
		t.Fatalf("ReachLeniency = %v, want 0", combat.ReachLeniency)
	}
	if combat.LerpSteps != 10 {
		t.Fatalf("LerpSteps = %v, want 10", combat.LerpSteps)
	}
	if combat.EntitySearchRadius != 6 {
		t.Fatalf("EntitySearchRadius = %v, want 6", combat.EntitySearchRadius)
	}
}
