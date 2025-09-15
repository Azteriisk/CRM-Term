package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Store manages the runtime configuration for the CRM.
type Store struct {
	path   string
	Config Data
}

// Data represents persisted user preferences.
type Data struct {
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

// Load retrieves the config from disk, creating defaults if needed.
func Load() (*Store, error) {
	cfgPath, err := resolvePath()
	if err != nil {
		return nil, err
	}

	cfg := Data{}
	if _, err := os.Stat(cfgPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat config: %w", err)
		}
		cfg = defaultConfig()
		if err := writeConfig(cfgPath, cfg); err != nil {
			return nil, err
		}
	} else {
		bytes, err := os.ReadFile(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if err := json.Unmarshal(bytes, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	if cfg.Timezone == "" {
		cfg.Timezone = defaultTimezone()
	}
	if cfg.Name == "" {
		cfg.Name = defaultName()
	}

	return &Store{path: cfgPath, Config: cfg}, nil
}

// Save writes the current config values to disk.
func (s *Store) Save() error {
	if s == nil {
		return errors.New("nil config store")
	}
	return writeConfig(s.path, s.Config)
}

func resolvePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.Getenv("HOME")
		if base == "" {
			return "", fmt.Errorf("cannot resolve config directory: %w", err)
		}
	}
	dir := filepath.Join(base, "crmterm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return filepath.Join(dir, "config.json"), nil
}

func writeConfig(path string, cfg Data) error {
	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func defaultConfig() Data {
	return Data{
		Name:     defaultName(),
		Timezone: defaultTimezone(),
	}
}

func defaultName() string {
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	if runtime.GOOS == "windows" {
		if name := os.Getenv("USERNAME"); name != "" {
			return name
		}
	}
	return "CRM User"
}

func defaultTimezone() string {
	if locName := time.Now().Location().String(); locName != "Local" && locName != "" {
		return locName
	}
	return "UTC"
}

// Location returns the configured timezone Location, defaulting to UTC on error.
func (s *Store) Location() *time.Location {
	if s == nil {
		return time.UTC
	}
	if loc, err := time.LoadLocation(s.Config.Timezone); err == nil {
		return loc
	}
	return time.UTC
}
