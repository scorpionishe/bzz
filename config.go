package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Enabled         bool     `yaml:"enabled"`
	PrimaryLanguage string   `yaml:"primary_language"`
	MinWordLength   int      `yaml:"min_word_length"`
	ExcludedApps    []string `yaml:"excluded_apps"`
}

func DefaultConfig() Config {
	return Config{
		Enabled:         true,
		PrimaryLanguage: "ru",
		MinWordLength:   2,
		ExcludedApps:    []string{"idea"},
	}
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "RuSwitch", "config.yaml")
}

func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

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
