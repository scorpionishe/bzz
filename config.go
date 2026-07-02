package main

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Enabled         bool     `yaml:"enabled"`
	PrimaryLanguage string   `yaml:"primary_language"`
	MinWordLength   int      `yaml:"min_word_length"`
	ExcludedApps    []string `yaml:"excluded_apps"`
	// Hotkey for manual selection conversion. Examples: "cmd+shift+x",
	// "ctrl+space", "f18". A single dedicated key like f18 (synthesized from a
	// Caps Lock tap via Karabiner) avoids modifier/character leaks.
	Hotkey string `yaml:"hotkey"`
	// SwitchLayout, when true, also switches the macOS system input source to
	// match a correction (classic Punto "switch" mode). Default false keeps Bzz
	// layout-neutral: it only rewrites the text and leaves the active layout alone.
	SwitchLayout bool `yaml:"switch_layout"`
	// ContextAware, when true, uses recent-word language context plus an
	// impossible-in-English letter-combo check to catch wrong-layout words that
	// are not in the dictionary (e.g. "ddj" → "вво"). Default true.
	ContextAware bool `yaml:"context_aware"`
}

func DefaultConfig() Config {
	return Config{
		Enabled:         true,
		PrimaryLanguage: "ru",
		MinWordLength:   2,
		ExcludedApps:    []string{"idea"},
		Hotkey:          "f18",
		SwitchLayout:    false,
		ContextAware:    true,
	}
}

func configPath() string {
	dir, err := defaultConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Library", "Application Support", "Bzz")
	}
	return filepath.Join(dir, "config.yaml")
}

func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	// configPath() triggers the RuSwitch→Bzz config dir migration via
	// defaultConfigDir(). migrateConfigDir handles the "new dir exists but
	// is empty" edge case, so we don't need a second legacy-path lookup here.
	data, err := os.ReadFile(configPath())
	if err != nil {
		// No config file — save defaults and return
		_ = SaveConfig(&cfg)
		return &cfg, nil
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &cfg, err
	}
	return &cfg, nil
}

// IsAppExcluded checks if the given app bundle ID (or substring) matches the excluded list.
// Matching is case-insensitive substring — e.g. "idea" matches "com.jetbrains.intellij.idea.ce".
func (c *Config) IsAppExcluded(bundleID string) bool {
	if bundleID == "" || len(c.ExcludedApps) == 0 {
		return false
	}
	lower := strings.ToLower(bundleID)
	for _, ex := range c.ExcludedApps {
		if ex == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(ex)) {
			return true
		}
	}
	return false
}

func SaveConfig(cfg *Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
