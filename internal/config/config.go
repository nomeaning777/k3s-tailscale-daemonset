// Package config handles configuration loading and validation.
package config

import (
	"errors"
	"fmt"
	"net"
	"os"

	"gopkg.in/yaml.v3"
)

// ErrNoRoutesSpecified is returned when no routes are configured.
var ErrNoRoutesSpecified = errors.New("no routes specified")

// Config represents the application configuration.
type Config struct {
	Routes []string `yaml:"routes"`
}

// Load reads and parses a configuration file from the specified path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is from trusted source (config)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if len(c.Routes) == 0 {
		return ErrNoRoutesSpecified
	}

	for _, route := range c.Routes {
		_, _, err := net.ParseCIDR(route)
		if err != nil {
			return fmt.Errorf("invalid CIDR format for route %s: %w", route, err)
		}
	}

	return nil
}
