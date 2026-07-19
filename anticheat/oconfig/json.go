package oconfig

import (
	"errors"
	"fmt"
	"maps"
	"os"

	"github.com/hjson/hjson-go/v4"
)

var (
	ErrConfigCreated = errors.New("config file created - please fill in required fields")
	ErrConfigUpdated = errors.New("config file updated - please fill in required fields")
	ErrConfigTooNew  = errors.New("config file was created by a newer Oomph version")
)

// ParseRawJSON parses a raw JSON string and returns a Config struct.
func ParseRawJSON(data []byte) (Config, error) {
	parsedCfg := DefaultConfig
	parsedCfg.Detections = maps.Clone(DefaultConfig.Detections)
	if err := hjson.Unmarshal(data, &parsedCfg); err != nil {
		return Config{}, fmt.Errorf("unable to parse config: %w", err)
	}

	if parsedCfg.Version > ConfigVersion {
		return Config{}, fmt.Errorf("%w: got version %d, support up to %d", ErrConfigTooNew, parsedCfg.Version, ConfigVersion)
	}

	if parsedCfg.Version < ConfigVersion {
		newCfg := parsedCfg
		switch parsedCfg.Version {
		case 0: // No version set.
			newCfg.Prefix = DefaultConfig.Prefix
			newCfg.GCPercent = DefaultConfig.GCPercent
			newCfg.MemThreshold = DefaultConfig.MemThreshold
			newCfg.Detections = maps.Clone(DefaultConfig.Detections)
		case 2:
			newCfg.Network = DefaultConfig.Network
		case 3:
			// For version 4, we added two new detections to the configuration.
			if newCfg.Detections == nil {
				newCfg.Detections = make(map[string]Detection)
			}
			newCfg.Detections["Proxy_A"] = DefaultConfig.Detections["Proxy_A"]
			newCfg.Detections["Proxy_B"] = DefaultConfig.Detections["Proxy_B"]
		case 4:
			newCfg.Movement.LimitAllVelocity = DefaultConfig.Movement.LimitAllVelocity
			newCfg.Movement.LimitAllVelocityThreshold = DefaultConfig.Movement.LimitAllVelocityThreshold
		}
		newCfg.Version = ConfigVersion
		return newCfg, ErrConfigUpdated
	}

	return parsedCfg, nil
}

// ParseJSON parses a JSON file and returns a Config struct.
func ParseJSON(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("unable to read config file: %w", err)
		}
		if err = CreateJSON(file); err != nil {
			return err
		}

		return ErrConfigCreated
	}

	parsedCfg, err := ParseRawJSON(data)
	if err != nil && !errors.Is(err, ErrConfigUpdated) {
		return fmt.Errorf("unable to parse config file: %w", err)
	}

	if errors.Is(err, ErrConfigUpdated) {
		if writeErr := writeMigratedJSON(file, data, parsedCfg); writeErr != nil {
			return fmt.Errorf("unable to update config file: %w", writeErr)
		}
		return ErrConfigUpdated
	}

	Global = parsedCfg
	return nil
}

// writeMigratedJSON rewrites known configuration values while retaining fields
// owned by wrappers or future versions. Spectrum's obsolete token is explicitly
// removed during the version-5 to version-6 migration.
func writeMigratedJSON(file string, original []byte, cfg Config) error {
	var existing hjson.Node
	if err := hjson.Unmarshal(original, &existing); err != nil {
		return fmt.Errorf("unable to preserve existing config fields: %w", err)
	}
	knownData, err := hjson.MarshalWithOptions(cfg, configEncoderOptions())
	if err != nil {
		return fmt.Errorf("unable to marshal migrated config: %w", err)
	}
	var known hjson.Node
	if err := hjson.Unmarshal(knownData, &known); err != nil {
		return fmt.Errorf("unable to prepare migrated config: %w", err)
	}
	if err := mergeConfigNodes(&existing, &known); err != nil {
		return fmt.Errorf("unable to merge migrated config: %w", err)
	}
	if _, _, err := existing.DeleteKey("spectrum_api_token"); err != nil {
		return fmt.Errorf("unable to remove Spectrum config: %w", err)
	}
	return writeValue(file, existing)
}

func mergeConfigNodes(dst, src *hjson.Node) error {
	dstMap, dstOK := dst.Value.(*hjson.OrderedMap)
	srcMap, srcOK := src.Value.(*hjson.OrderedMap)
	if !dstOK || !srcOK {
		return fmt.Errorf("expected config objects, got %T and %T", dst.Value, src.Value)
	}
	for _, key := range srcMap.Keys {
		srcChild, ok := srcMap.Map[key].(*hjson.Node)
		if !ok {
			return fmt.Errorf("expected source node for %q, got %T", key, srcMap.Map[key])
		}
		dstValue, exists := dstMap.AtKey(key)
		if !exists {
			dstMap.Set(key, srcChild)
			continue
		}
		dstChild, ok := dstValue.(*hjson.Node)
		if !ok {
			return fmt.Errorf("expected destination node for %q, got %T", key, dstValue)
		}
		_, dstNested := dstChild.Value.(*hjson.OrderedMap)
		_, srcNested := srcChild.Value.(*hjson.OrderedMap)
		if dstNested && srcNested {
			if err := mergeConfigNodes(dstChild, srcChild); err != nil {
				return err
			}
			continue
		}
		dstChild.Value = srcChild.Value
	}
	return nil
}

// CreateJSON creates a new JSON file with default config.
func CreateJSON(file string) error {
	// Write default config to file.
	dat, err := hjson.MarshalWithOptions(DefaultConfig, configEncoderOptions())
	if err != nil {
		return fmt.Errorf("unable to write default config to file: %v", err)
	}

	if err := os.WriteFile(file, dat, 0644); err != nil {
		return fmt.Errorf("unable to write default config to file: %v", err)
	}
	return nil
}

func WriteJSON(file string, cfg Config) error {
	return writeValue(file, cfg)
}

func writeValue(file string, value any) error {
	dat, err := hjson.MarshalWithOptions(value, hjson.EncoderOptions{
		IndentBy:              "    ",
		EmitRootBraces:        true,
		QuoteAlways:           false,
		QuoteAmbiguousStrings: false,
		Eol:                   "\n",
		Comments:              true,
	})
	if err != nil {
		return fmt.Errorf("unable to write config to file: %v", err)
	}

	if err := os.WriteFile(file, dat, 0644); err != nil {
		return fmt.Errorf("unable to write config to file: %v", err)
	}
	return nil
}

func configEncoderOptions() hjson.EncoderOptions {
	return hjson.EncoderOptions{
		IndentBy:              "    ",
		EmitRootBraces:        true,
		QuoteAlways:           false,
		QuoteAmbiguousStrings: false,
		Eol:                   "\n",
		Comments:              true,
	}
}
