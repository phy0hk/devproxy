package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	path := writeTestConfig(t, `proxy:
  targets:
    - name: api
      path: /api
      url: http://127.0.0.1:4000
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Proxy.Host != "127.0.0.1" {
		t.Fatalf("proxy host = %q, want %q", cfg.Server.Proxy.Host, "127.0.0.1")
	}

	if cfg.Server.Proxy.Port != 8080 {
		t.Fatalf("proxy port = %d, want %d", cfg.Server.Proxy.Port, 8080)
	}

	if cfg.Server.UI.Host != "127.0.0.1" {
		t.Fatalf("ui host = %q, want %q", cfg.Server.UI.Host, "127.0.0.1")
	}

	if cfg.Server.UI.Port != 8081 {
		t.Fatalf("ui port = %d, want %d", cfg.Server.UI.Port, 8081)
	}
}

func TestLoadProcesses(t *testing.T) {
	path := writeTestConfig(t, `server:
  proxy:
    port: 9000
  ui:
    port: 9001
processes:
  - name: frontend
    command: pnpm dev
    working_dir: ./frontend
    env:
      PORT: "3000"
proxy:
  targets:
    - name: frontend
      path: /
      url: http://127.0.0.1:3000
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.Processes) != 1 {
		t.Fatalf("got %d processes, want 1", len(cfg.Processes))
	}

	process := cfg.Processes[0]
	if process.Name != "frontend" {
		t.Fatalf("process name = %q, want frontend", process.Name)
	}

	if process.Env["PORT"] != "3000" {
		t.Fatalf("PORT env = %q, want 3000", process.Env["PORT"])
	}
}

func TestValidateProcessRequiresCommand(t *testing.T) {
	cfg := validConfig()
	cfg.Processes = []ProcessConfig{
		{
			Name: "frontend",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "command cannot be empty") {
		t.Fatalf("error = %q, want command validation", err.Error())
	}
}

func TestValidateProcessNamesMustBeUnique(t *testing.T) {
	cfg := validConfig()
	cfg.Processes = []ProcessConfig{
		{
			Name:    "app",
			Command: "pnpm dev",
		},
		{
			Name:    "app",
			Command: "pnpm start:dev",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "name must be unique") {
		t.Fatalf("error = %q, want duplicate name validation", err.Error())
	}
}

func TestValidateProxyTargetNamesMustBeUnique(t *testing.T) {
	cfg := validConfig()
	cfg.Proxy.Targets = append(cfg.Proxy.Targets, cfg.Proxy.Targets[0])

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "name must be unique") {
		t.Fatalf("error = %q, want duplicate target validation", err.Error())
	}
}

func validConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Proxy: ServerAddress{
				Host: "127.0.0.1",
				Port: 8080,
			},
			UI: ServerAddress{
				Host: "127.0.0.1",
				Port: 8081,
			},
		},
		Proxy: ProxyConfig{
			Targets: []TargetConfig{
				{
					Name: "api",
					Path: "/api",
					URL:  "http://127.0.0.1:4000",
				},
			},
		},
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}
