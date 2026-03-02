package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the project configuration from .sandbox.yml.
type Config struct {
	PostCheckout []string `yaml:"post-checkout"`
}

// Load reads .sandbox.yml from dir and returns the parsed config.
// If the file does not exist, it returns a zero Config and no error.
func Load(dir string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, ".sandbox.yml"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
