package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Proxy  ProxyConfig  `yaml:"proxy"`
}

type ServerConfig struct {
	Proxy ServerAddress `yaml:"proxy"`
	UI    ServerAddress `yaml:"ui"`
}

type ServerAddress struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
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
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}

	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
}

func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf(
			"server port must be between 1 and 65535",
		)
	}

	for i, target := range c.Proxy.Targets {
		if target.Name == "" {
			return fmt.Errorf(
				"proxy target %d: name cannot be empty",
				i,
			)
		}

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
