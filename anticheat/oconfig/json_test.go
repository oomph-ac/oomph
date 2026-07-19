package oconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRawJSONMigratesVersionSixCombatValidatorOptions(t *testing.T) {
	cfg, err := ParseRawJSON([]byte(`{
		version: 6
		combat_opts: {
			maximum_attack_angle: 72
			disable_block_occlusion_check: true
			maximum_reach: 3.25
		}
	}`))
	if !errors.Is(err, ErrConfigUpdated) {
		t.Fatalf("ParseRawJSON() error = %v, want ErrConfigUpdated", err)
	}
	if cfg.Version != ConfigVersion {
		t.Fatalf("Version = %d, want %d", cfg.Version, ConfigVersion)
	}
	if cfg.Combat.MaximumAttackAngle != 72 {
		t.Fatalf("MaximumAttackAngle = %v, want preserved value 72", cfg.Combat.MaximumAttackAngle)
	}
	if !cfg.Combat.DisableBlockOcclusionCheck || cfg.Combat.MaximumReach != 3.25 {
		t.Fatalf("existing combat validator values were not preserved: %#v", cfg.Combat)
	}
	if cfg.Combat.BBoxExpansion != 0.1 || cfg.Combat.LerpSteps != 10 || cfg.Combat.EntitySearchRadius != 6 {
		t.Fatalf("combat validator defaults not populated: %#v", cfg.Combat)
	}
}

func TestParseRawJSONMigratesVersionFiveWithoutLosingValues(t *testing.T) {
	defaultReach := DefaultConfig.Detections["Reach_A"]
	raw := []byte(`{
		version: 5
		spectrum_api_token: legacy-secret
		prefix: custom-prefix
		command_name: custom-command
		gc_percent: 37
		mem_threshold: 2048
		local_addr: ":19140"
		remote_addr: "backend.example:19132"
		movement_opts: {
			correction_threshold: 0.75
			limit_all_velocity: true
			limit_all_velocity_threshold: 12
		}
		detections: {
			Reach_A: {
				max_violations: 42
				flag_message: custom-message
				punishment_type: kick
			}
		}
	}`)

	cfg, err := ParseRawJSON(raw)
	if !errors.Is(err, ErrConfigUpdated) {
		t.Fatalf("ParseRawJSON() error = %v, want ErrConfigUpdated", err)
	}
	if cfg.Version != ConfigVersion {
		t.Fatalf("Version = %d, want %d", cfg.Version, ConfigVersion)
	}
	if cfg.Prefix != "custom-prefix" || cfg.CommandName != "custom-command" {
		t.Fatalf("string values were not preserved: %#v", cfg)
	}
	if cfg.GCPercent != 37 || cfg.MemThreshold != 2048 {
		t.Fatalf("runtime values were not preserved: GC=%d memory=%d", cfg.GCPercent, cfg.MemThreshold)
	}
	if cfg.Movement.CorrectionThreshold != 0.75 || cfg.Movement.LimitAllVelocityThreshold != 12 {
		t.Fatalf("movement values were not preserved: %#v", cfg.Movement)
	}
	if got := cfg.Detections["Reach_A"]; got.MaxVl != 42 || got.FlagMsg != "custom-message" || got.Punishment != PunishmentTypeKick {
		t.Fatalf("detection values were not preserved: %#v", got)
	}
	if got := DefaultConfig.Detections["Reach_A"]; got != defaultReach {
		t.Fatalf("ParseRawJSON mutated DefaultConfig.Detections: got %#v, want %#v", got, defaultReach)
	}
}

func TestParseRawJSONRejectsNewerConfigVersion(t *testing.T) {
	_, err := ParseRawJSON([]byte(`{
		version: 8
		prefix: future-prefix
	}`))
	if !errors.Is(err, ErrConfigTooNew) {
		t.Fatalf("ParseRawJSON() error = %v, want ErrConfigTooNew", err)
	}
}

func TestParseRawJSONPreservesVersionOneNetworkOptions(t *testing.T) {
	cfg, err := ParseRawJSON([]byte(`{
		version: 1
		network_opts: {
			max_ack_timeout: 91
			max_entity_rewind: 17
		}
	}`))
	if !errors.Is(err, ErrConfigUpdated) {
		t.Fatalf("ParseRawJSON() error = %v, want ErrConfigUpdated", err)
	}
	if cfg.Network.MaxACKTimeout != 91 || cfg.Network.MaxEntityRewind != 17 {
		t.Fatalf("version-one network values were not preserved: %#v", cfg.Network)
	}
}

func TestParseRawJSONVersionZeroDoesNotShareDefaultDetections(t *testing.T) {
	cfg, err := ParseRawJSON([]byte(`{
		version: 0
		prefix: legacy
	}`))
	if !errors.Is(err, ErrConfigUpdated) {
		t.Fatalf("ParseRawJSON() error = %v, want ErrConfigUpdated", err)
	}
	defaultReach := DefaultConfig.Detections["Reach_A"]
	cfg.Detections["Reach_A"] = Detection{MaxVl: 999}
	if got := DefaultConfig.Detections["Reach_A"]; got != defaultReach {
		t.Fatalf("version-zero config shares DefaultConfig.Detections: got %#v, want %#v", got, defaultReach)
	}
}

func TestParseJSONDoesNotRewriteNewerConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oomph.hjson")
	original := []byte(`{
		version: 8
		future_setting: keep-me
	}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	err := ParseJSON(path)
	if !errors.Is(err, ErrConfigTooNew) {
		t.Fatalf("ParseJSON() error = %v, want ErrConfigTooNew", err)
	}
	written, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(written) != string(original) {
		t.Fatalf("newer config was rewritten:\ngot:  %s\nwant: %s", written, original)
	}
}

func TestParseJSONRewritesLegacyConfigWithoutSpectrumSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oomph.hjson")
	if err := os.WriteFile(path, []byte(`{
		version: 5
		spectrum_api_token: legacy-secret
		prefix: custom-prefix
		third_party_setting: keep-me
		# wrapper-owned comment
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	err := ParseJSON(path)
	if !errors.Is(err, ErrConfigUpdated) {
		t.Fatalf("ParseJSON() error = %v, want ErrConfigUpdated", err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(written), "spectrum") || strings.Contains(string(written), "legacy-secret") {
		t.Fatalf("rewritten config retained Spectrum data:\n%s", written)
	}
	if !strings.Contains(string(written), "custom-prefix") {
		t.Fatalf("rewritten config lost custom prefix:\n%s", written)
	}
	if !strings.Contains(string(written), "third_party_setting") || !strings.Contains(string(written), "keep-me") {
		t.Fatalf("rewritten config lost an unknown setting:\n%s", written)
	}
	if !strings.Contains(string(written), "# wrapper-owned comment") {
		t.Fatalf("rewritten config lost an existing comment:\n%s", written)
	}
	if !strings.Contains(string(written), "The maximum amount of CPS that Oomph will allow") {
		t.Fatalf("rewritten config omitted documentation for migrated defaults:\n%s", written)
	}
}

func TestParseJSONSetsGlobalForCurrentConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oomph.hjson")
	cfg := DefaultConfig
	cfg.Prefix = "global-prefix"
	if err := WriteJSON(path, cfg); err != nil {
		t.Fatal(err)
	}

	Global = Config{}
	if err := ParseJSON(path); err != nil {
		t.Fatalf("ParseJSON() error = %v", err)
	}
	if Global.Prefix != "global-prefix" {
		t.Fatalf("Global.Prefix = %q, want global-prefix", Global.Prefix)
	}
}

func TestParseJSONLeavesCurrentConfigFileUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oomph.hjson")
	original := []byte(`{
		version: 7
		prefix: current-prefix
		third_party_setting: keep-me
		# preserve this comment and formatting
	}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ParseJSON(path); err != nil {
		t.Fatalf("ParseJSON() error = %v", err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(original) {
		t.Fatalf("current config was rewritten:\ngot:  %s\nwant: %s", written, original)
	}
}

func TestParseJSONReturnsNonNotExistReadErrors(t *testing.T) {
	err := ParseJSON(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "unable to read config file") {
		t.Fatalf("ParseJSON() error = %v, want read error", err)
	}
}

func TestParseJSONCreatesMissingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oomph.hjson")
	if err := ParseJSON(path); !errors.Is(err, ErrConfigCreated) {
		t.Fatalf("ParseJSON() error = %v, want ErrConfigCreated", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("created config: %v", err)
	}
}

func TestCreateJSONWritesReadableCombatDecimals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oomph.hjson")
	if err := CreateJSON(path); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(written)
	if !strings.Contains(text, "bbox_expansion: 0.1\n") {
		t.Fatalf("generated config does not contain readable bbox expansion:\n%s", text)
	}
	if !strings.Contains(text, "maximum_reach: 2.9\n") {
		t.Fatalf("generated config does not contain readable maximum reach:\n%s", text)
	}
}
