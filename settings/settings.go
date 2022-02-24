package settings

import (
	"errors"
	"fmt"
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/pelletier/go-toml"
	"io/ioutil"
	"os"
)

// Settings contains all settings that can be configured for each check and Oomph.
type Settings struct {
	Oomph struct {
		ViewDistance int32
	}
	AutoClicker struct {
		A struct {
			Basics
			MaxCPS int
		}
		B Basics
		C Basics
		D Basics
	}
	Velocity struct {
		A struct {
			Basics
			MinY float64
			MaxY float64
		}
		B struct {
			Basics
			MinXZ float64
			MaxXZ float64
		}
	}
	Reach struct {
		A struct {
			Basics
			MinDist    float64
			MinRaycast float64
		}
	}
	Timer struct {
		A Basics
	}
	AimAssist struct {
		A Basics
	}
	KillAura struct {
		A Basics
		B Basics
	}
	InvalidMovement struct {
		A Basics
		B Basics
		C Basics
	}
	OSSpoofer struct {
		A Basics
	}
}

// Basics are the basic settings for a check.
type Basics struct {
	// Enabled is whether the check should be enabled or not.
	Enabled bool
	// MaxViolations is the amount of violations until a punishment is issued for the check.
	MaxViolations uint32
	// Punishment is the type of punishment that should be issued for the check
	Punishment punishment.Punishment
}

// DefaultSettings returns the default settings for all checks.
func DefaultSettings() Settings {
	settings := Settings{}
	settings.Oomph.ViewDistance = 8

	basics := Basics{Enabled: true, Punishment: punishment.Ban()}
	settings.AutoClicker.A.Basics = basics
	settings.AutoClicker.A.MaxViolations = 15
	settings.AutoClicker.A.MaxCPS = 22

	settings.AutoClicker.B = basics
	settings.AutoClicker.B.MaxViolations = 15
	settings.AutoClicker.C = basics
	settings.AutoClicker.C.MaxViolations = 15
	settings.AutoClicker.D = basics
	settings.AutoClicker.D.MaxViolations = 15

	settings.Velocity.A.Basics = basics
	settings.Velocity.A.MaxViolations = 15
	settings.Velocity.A.MinY = 0.9999
	settings.Velocity.A.MaxY = 1.1

	settings.Velocity.B.Basics = basics
	settings.Velocity.B.MaxViolations = 15
	settings.Velocity.B.MinXZ = 0.9999
	settings.Velocity.B.MaxXZ = 1.5

	settings.Reach.A.Basics = basics
	settings.Reach.A.MaxViolations = 15
	settings.Reach.A.MinDist = 3.15
	settings.Reach.A.MinRaycast = 3.1

	settings.Timer.A = basics
	settings.Timer.A.MaxViolations = 10

	settings.AimAssist.A = basics
	settings.AimAssist.A.MaxViolations = 15

	settings.KillAura.A = basics
	settings.KillAura.A.MaxViolations = 15
	settings.KillAura.B = basics
	settings.KillAura.B.MaxViolations = 15

	settings.InvalidMovement.A = basics
	settings.InvalidMovement.A.MaxViolations = 50
	settings.InvalidMovement.B = basics
	settings.InvalidMovement.B.MaxViolations = 50
	settings.InvalidMovement.C = basics
	settings.InvalidMovement.C.MaxViolations = 10

	settings.OSSpoofer.A = basics
	settings.OSSpoofer.A.MaxViolations = 1
	return settings
}

// SaveDefault will create and save the default settings file. If the file already exists, it will return an error.
func SaveDefault(path string) error {
	s := DefaultSettings()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if data, err := toml.Marshal(s); err != nil {
			return fmt.Errorf("failed encoding default settings: %v", err)
		} else if err := ioutil.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("failed creating settings file: %v", err)
		}
		return nil
	}
	return errors.New("settings file already exists")
}

// Load will load the settings from your settings file, and return an error if the file does not exist.
func Load(path string) (Settings, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Settings{}, errors.New("settings file doesn't exist")
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return Settings{}, fmt.Errorf("error reading config: %v", err)
	}

	var settings Settings
	if err = toml.Unmarshal(data, &settings); err != nil {
		return Settings{}, fmt.Errorf("error decoding config: %v", err)
	}
	return settings, nil
}
