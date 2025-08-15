package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig           `yaml:"server"`
	Capture CaptureConfig          `yaml:"capture"`
	Routes  map[string]RouteConfig `yaml:"routes"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Bind string `yaml:"bind"`
	Port int    `yaml:"port"`
}

// CaptureConfig holds capture-related configuration
type CaptureConfig struct {
	MaxBodyMB      int    `yaml:"max_body_mb"`
	Store          string `yaml:"store"`
	WorkerPoolSize int    `yaml:"worker_pool_size"`
}

// RouteConfig holds route-specific configuration
type RouteConfig struct {
	Mount    string `yaml:"mount"`
	Upstream string `yaml:"upstream"`
}

// Load loads configuration from file and applies environment overrides
func Load(configPath string) (*Config, error) {
	config := &Config{}
	if err := loadFromFile(config, configPath); err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	return config, nil
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(config *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, use defaults
		}
		return err
	}

	return yaml.Unmarshal(data, config)
}

// Address returns the server address in host:port format
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Bind, c.Server.Port)
}

// MaxBodyBytes returns the maximum body size in bytes
func (c *Config) MaxBodyBytes() int64 {
	return int64(c.Capture.MaxBodyMB) * 1024 * 1024
}

// GetRouteByMount returns the route config for a given mount path
func (c *Config) GetRouteByMount(mount string) (string, RouteConfig, bool) {
	mount = strings.TrimSuffix(mount, "/")
	for name, route := range c.Routes {
		if strings.TrimSuffix(route.Mount, "/") == mount {
			return name, route, true
		}
	}
	return "", RouteConfig{}, false
}
