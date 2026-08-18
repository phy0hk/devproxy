package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Processes []ProcessConfig `yaml:"processes"`
	Proxy     ProxyConfig     `yaml:"proxy"`
}

type ServerConfig struct {
	Proxy ServerAddress `yaml:"proxy"`
	UI    ServerAddress `yaml:"ui"`
}

type ServerAddress struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type ProcessConfig struct {
	Name       string            `yaml:"name"`
	Command    string            `yaml:"command"`
	WorkingDir string            `yaml:"working_dir"`
	Env        map[string]string `yaml:"env"`
	Health     HealthConfig      `yaml:"health"`
}

type HealthConfig struct {
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout"`
}

type ProxyConfig struct {
	Targets []TargetConfig `yaml:"targets"`
}

type TargetConfig struct {
	Name    string `yaml:"name"`
	Path    string `yaml:"path"`
	URL     string `yaml:"url"`
	Rewrite bool   `yaml:"rewrite"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.setDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) setDefaults() {
	if c.Server.Proxy.Host == "" {
		c.Server.Proxy.Host = "127.0.0.1"
	}

	if c.Server.Proxy.Port == 0 {
		c.Server.Proxy.Port = 8080
	}

	if c.Server.UI.Host == "" {
		c.Server.UI.Host = "127.0.0.1"
	}

	if c.Server.UI.Port == 0 {
		c.Server.UI.Port = 8081
	}
}

func (c *Config) Validate() error {
	if err := validateServerAddress("server.proxy", c.Server.Proxy); err != nil {
		return err
	}

	if err := validateServerAddress("server.ui", c.Server.UI); err != nil {
		return err
	}

	processNames := make(map[string]struct{}, len(c.Processes))
	for i, process := range c.Processes {
		if process.Name == "" {
			return fmt.Errorf(
				"process %d: name cannot be empty",
				i,
			)
		}

		if _, exists := processNames[process.Name]; exists {
			return fmt.Errorf(
				"process %q: name must be unique",
				process.Name,
			)
		}
		processNames[process.Name] = struct{}{}

		if process.Command == "" {
			return fmt.Errorf(
				"process %q: command cannot be empty",
				process.Name,
			)
		}
	}

	targetNames := make(map[string]struct{}, len(c.Proxy.Targets))
	for i, target := range c.Proxy.Targets {
		if target.Name == "" {
			return fmt.Errorf(
				"proxy target %d: name cannot be empty",
				i,
			)
		}

		if _, exists := targetNames[target.Name]; exists {
			return fmt.Errorf(
				"proxy target %q: name must be unique",
				target.Name,
			)
		}
		targetNames[target.Name] = struct{}{}

		if target.Path == "" {
			return fmt.Errorf(
				"proxy target %q: path cannot be empty",
				target.Name,
			)
		}

		if !strings.HasPrefix(target.Path, "/") {
			return fmt.Errorf(
				"proxy target %q: path must start with /",
				target.Name,
			)
		}

		u, err := url.Parse(target.URL)
		if err != nil {
			return fmt.Errorf(
				"proxy target %q: invalid url: %w",
				target.Name,
				err,
			)
		}

		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf(
				"proxy target %q: url must use http or https",
				target.Name,
			)
		}

		if u.Host == "" {
			return fmt.Errorf(
				"proxy target %q: url must contain a host",
				target.Name,
			)
		}
	}

	return nil
}

func validateServerAddress(name string, address ServerAddress) error {
	if address.Port < 1 || address.Port > 65535 {
		return fmt.Errorf(
			"%s port must be between 1 and 65535",
			name,
		)
	}

	return nil
}
