package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractFlagsVersionForms(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantVersion string
		wantConfig  string
		wantRest    []string
	}{
		{
			name:        "no flags",
			args:        []string{"up", "--config", "config.yaml"},
			wantVersion: "",
			wantConfig:  "config.yaml",
			wantRest:    []string{"up", "--config", "config.yaml"},
		},
		{
			name:        "version flag with value",
			args:        []string{"--version", "v1.2.3", "up"},
			wantVersion: "v1.2.3",
			wantRest:    []string{"up"},
		},
		{
			name:        "version flag equals form",
			args:        []string{"up", "--version=v2.0.0"},
			wantVersion: "v2.0.0",
			wantRest:    []string{"up"},
		},
		{
			name:        "version flag without value passes through",
			args:        []string{"up", "--version"},
			wantVersion: "",
			wantRest:    []string{"up", "--version"},
		},
		{
			name:        "config flag is kept in pass-through args",
			args:        []string{"up", "--config", "custom.yaml"},
			wantVersion: "",
			wantConfig:  "custom.yaml",
			wantRest:    []string{"up", "--config", "custom.yaml"},
		},
		{
			name:        "config flag equals form",
			args:        []string{"up", "--config=custom.yaml"},
			wantVersion: "",
			wantConfig:  "custom.yaml",
			wantRest:    []string{"up", "--config=custom.yaml"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			version, configPath, rest := extractFlags(tc.args)
			if version != tc.wantVersion {
				t.Errorf("version = %q, want %q", version, tc.wantVersion)
			}
			if configPath != tc.wantConfig {
				t.Errorf("configPath = %q, want %q", configPath, tc.wantConfig)
			}
			if len(rest) != len(tc.wantRest) {
				t.Fatalf("rest = %v, want %v", rest, tc.wantRest)
			}
			for i := range rest {
				if rest[i] != tc.wantRest[i] {
					t.Fatalf("rest = %v, want %v", rest, tc.wantRest)
				}
			}
		})
	}
}

func TestValidVersion(t *testing.T) {
	valid := []string{"v1", "v1.2.3", "V2-beta", "1.0.0-rc.1"}
	for _, version := range valid {
		if !validVersion(version) {
			t.Errorf("validVersion(%q) = false, want true", version)
		}
	}

	invalid := []string{"", "../etc", "a/b", "v1;rm", "v 1", string(make([]byte, 65))}
	for _, version := range invalid {
		if validVersion(version) {
			t.Errorf("validVersion(%q) = true, want false", version)
		}
	}
}

func TestVersionFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if got := versionFromConfig(path); got != "" {
		t.Fatalf("versionFromConfig(missing) = %q, want empty", got)
	}

	content := "server:\n  proxy:\n    port: 8080\nversion: v1.4.2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := versionFromConfig(path); got != "v1.4.2" {
		t.Fatalf("versionFromConfig = %q, want v1.4.2", got)
	}

	if err := os.WriteFile(path, []byte("version: [broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := versionFromConfig(path); got != "" {
		t.Fatalf("versionFromConfig(invalid yaml) = %q, want empty fallback", got)
	}
}

func TestChecksumForAsset(t *testing.T) {
	checksums := "aaa111  devproxy-linux-amd64\nbbb222  devproxy-darwin-arm64\nccc332  *devproxy-windows-amd64.exe\n"

	if got, ok := checksumForAsset(checksums, "devproxy-linux-amd64"); !ok || got != "aaa111" {
		t.Errorf("checksumForAsset(linux) = %q, %v; want aaa111, true", got, ok)
	}
	if got, ok := checksumForAsset(checksums, "devproxy-windows-amd64.exe"); !ok || got != "ccc332" {
		t.Errorf("checksumForAsset(windows) = %q, %v; want ccc332, true", got, ok)
	}
	if _, ok := checksumForAsset(checksums, "devproxy-freebsd-amd64"); ok {
		t.Error("checksumForAsset(unknown) = true, want false")
	}
}

func TestIsRetryableDownloadError(t *testing.T) {
	retryable := []error{
		downloadStatusError(503),
		downloadStatusError(429),
		&netError{},
	}
	for _, err := range retryable {
		if !isRetryableDownloadError(err) {
			t.Errorf("isRetryableDownloadError(%v) = false, want true", err)
		}
	}

	notRetryable := []error{
		downloadStatusError(404),
		downloadStatusError(400),
		os.ErrPermission,
	}
	for _, err := range notRetryable {
		if isRetryableDownloadError(err) {
			t.Errorf("isRetryableDownloadError(%v) = true, want false", err)
		}
	}
}

type netError struct{}

func (e *netError) Error() string   { return "i/o timeout" }
func (e *netError) Timeout() bool   { return true }
func (e *netError) Temporary() bool { return true }
