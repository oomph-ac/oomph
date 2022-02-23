package settings

import (
	"errors"
	"fmt"
	"github.com/justtaldevelops/oomph/check/punishment"
	"github.com/pelletier/go-toml"
	"io/ioutil"
	"os"
)

type settings struct {
	AutoClicker struct {
		A struct {
			BaseSettings
			MaxCPS int
		}
		B BaseSettings
		C BaseSettings
		D BaseSettings
	}
	Velocity struct {
		A struct {
			BaseSettings
			MinY float64
			MaxY float64
		}
		B struct {
			BaseSettings
			MinXZ float64
			MaxXZ float64
		}
	}
	Reach struct {
		A struct {
			BaseSettings
			MinDist    float64
			MinRaycast float64
		}
	}
	Timer struct {
		A BaseSettings
	}
	AimAssist struct {
		A BaseSettings
	}
	KillAura struct {
		A BaseSettings
		B BaseSettings
	}
	OSSpoofer struct {
		A BaseSettings
	}
}

type BaseSettings struct {
	// Enabled is whether the check should be enabled or not.
	Enabled bool
	// MaxViolations is the amount of violations until a punishment is issued for the check.
	MaxViolations uint32
	// Punishment is the type of punishment that should be issued for the check
	Punishment punishment.Punishment
}

var Settings = defaultSettings()

func defaultSettings() settings {
	s := settings{}
	base := BaseSettings{Enabled: true, Punishment: punishment.Ban()}
	// AutoClicker
	s.AutoClicker.A.BaseSettings = base
	s.AutoClicker.A.MaxViolations = 15
	s.AutoClicker.A.MaxCPS = 22

	s.AutoClicker.B = base
	s.AutoClicker.B.MaxViolations = 15
	s.AutoClicker.C = base
	s.AutoClicker.C.MaxViolations = 15
	s.AutoClicker.D = base
	s.AutoClicker.D.MaxViolations = 15
	// Velocity
	s.Velocity.A.BaseSettings = base
	s.Velocity.A.MaxViolations = 15
	s.Velocity.A.MinY = 0.9999
	s.Velocity.A.MaxY = 1.1

	s.Velocity.B.BaseSettings = base
	s.Velocity.B.MaxViolations = 15
	s.Velocity.B.MinXZ = 0.9999
	s.Velocity.B.MaxXZ = 1.5
	// Reach
	s.Reach.A.BaseSettings = base
	s.Reach.A.MaxViolations = 15
	s.Reach.A.MinDist = 3.15
	s.Reach.A.MinRaycast = 3.1
	// Timer
	s.Timer.A = base
	s.Timer.A.MaxViolations = 10
	// AimAssist
	s.AimAssist.A = base
	s.AimAssist.A.MaxViolations = 15
	// KillAura
	s.KillAura.A = base
	s.KillAura.A.MaxViolations = 15
	s.KillAura.B = base
	s.KillAura.B.MaxViolations = 15
	// OS Spoofer
	s.OSSpoofer.A = base
	s.OSSpoofer.A.MaxViolations = 1
	return s
}

// SaveDefault will create and save the default settings file. If the file already exists, it will return an error.
func SaveDefault(path string) error {
	s := defaultSettings()
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
func Load(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return errors.New("settings file doesn't exist")
	}
	if data, err := ioutil.ReadFile(path); err != nil {
		return fmt.Errorf("error reading config: %v", err)
	} else if err := toml.Unmarshal(data, &Settings); err != nil {
		return fmt.Errorf("error decoding config: %v", err)
	}
	return nil
}
