package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultVersion   = "v1"
	cacheDirName     = ".devproxy"
	binDirName       = "bin"
	defaultBaseURL   = "https://github.com/phy0hk/devproxy/releases/download"
	downloadAttempts = 3
)

var (
	// binaryClient has a long overall timeout because release binaries can be
	// several megabytes on slow connections; the dial and header timeouts keep
	// dead connections from hanging the download.
	binaryClient = &http.Client{
		Timeout: 10 * time.Minute,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
	metaClient = &http.Client{Timeout: 30 * time.Second}
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "devproxy-bootstrap:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	explicitVersion, configPath, rest := extractFlags(args)

	version := explicitVersion
	if version == "" {
		version = os.Getenv("DEVPROXY_VERSION")
	}
	if version == "" {
		version = versionFromConfig(configPath)
	}
	if version == "" {
		version = defaultVersion
	}
	if !validVersion(version) {
		return fmt.Errorf("invalid devproxy version %q", version)
	}

	if err := ensureGitignoreEntry(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	binaryPath, err := ensureBinary(version)
	if err != nil {
		return err
	}

	return execDevProxy(binaryPath, rest)
}

// extractFlags pulls the bootstrap-only --version flag out of the argument
// list. --config is read (to locate config.yaml for version lookup) but kept
// in the pass-through arguments because the main app needs it too.
func extractFlags(args []string) (version string, configPath string, rest []string) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--version" || arg == "-version":
			if i+1 < len(args) {
				version = args[i+1]
				i++
			} else {
				// No value: pass through so the main app can handle it.
				rest = append(rest, arg)
			}
		case strings.HasPrefix(arg, "--version="):
			version = strings.TrimPrefix(arg, "--version=")
		case arg == "--config" || arg == "-config":
			rest = append(rest, arg)
			if i+1 < len(args) {
				configPath = args[i+1]
				rest = append(rest, args[i+1])
				i++
			}
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
			rest = append(rest, arg)
		default:
			rest = append(rest, arg)
		}
	}

	return version, configPath, rest
}

// versionFromConfig reads an optional top-level "version" key from the same
// config.yaml the main app uses. A missing or unreadable file falls back to
// the default version.
func validVersion(version string) bool {
	if version == "" || len(version) > 64 {
		return false
	}

	for _, r := range version {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '.':
		default:
			return false
		}
	}

	return true
}

func ensureBinary(version string) (string, error) {
	binaryPath := cachedBinaryPath(version)

	if _, err := os.Stat(binaryPath); err == nil {
		current, err := cachedBinaryIsCurrent(binaryPath)
		if err != nil {
			return "", err
		}
		if current {
			return binaryPath, nil
		}
		fmt.Fprintf(os.Stderr, "devproxy %s binary is missing or outdated, updating\n", version)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check cached devproxy binary: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		return "", fmt.Errorf("create devproxy cache directory: %w", err)
	}

	if err := downloadBinary(version, binaryPath); err != nil {
		return "", err
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(binaryPath, 0o755); err != nil {
			return "", fmt.Errorf("mark devproxy binary executable: %w", err)
		}
	}

	return binaryPath, nil
}

// cachedBinaryIsCurrent verifies the cached binary against the checksum that
// was recorded when it was installed. No network access is needed, so the
// bootstrap works offline once a binary has been downloaded and verified.
func cachedBinaryIsCurrent(path string) (bool, error) {
	expected, err := os.ReadFile(path + ".sha256")
	if err != nil {
		// No local checksum record: force a re-download and verify.
		return false, nil
	}

	actual, err := sha256File(path)
	if err != nil {
		return false, err
	}

	return strings.EqualFold(strings.TrimSpace(string(expected)), actual), nil
}

func downloadBinary(version, destination string) error {
	asset := assetName()
	releaseURL := downloadBaseURL() + "/" + version

	skipChecksum := os.Getenv("DEVPROXY_SKIP_CHECKSUM") == "1"
	if skipChecksum {
		fmt.Fprintln(os.Stderr, "warning: checksum verification skipped")
	}

	var expected string
	if !skipChecksum {
		var err error
		expected, err = fetchChecksum(releaseURL+"/checksums.txt", asset)
		if err != nil {
			return err
		}
	}

	binaryURL := releaseURL + "/" + asset
	fmt.Fprintf(os.Stderr, "downloading devproxy %s from %s\n", version, binaryURL)

	err := withDownloadRetry(func() error {
		tmp, err := downloadToFile(binaryURL, destination)
		if err != nil {
			return err
		}
		defer os.Remove(tmp)

		actual, err := sha256File(tmp)
		if err != nil {
			return err
		}

		if !skipChecksum && !strings.EqualFold(actual, expected) {
			return fmt.Errorf("checksum mismatch for %s", asset)
		}

		if err := installBinary(tmp, destination); err != nil {
			return err
		}

		return os.WriteFile(destination+".sha256", []byte(actual+"\n"), 0o644)
	})
	if err != nil {
		return fmt.Errorf("download devproxy %s binary: %w", version, err)
	}

	return nil
}

func downloadToFile(url, destination string) (string, error) {
	response, err := binaryClient.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", downloadStatusError(response.StatusCode)
	}

	tmp := fmt.Sprintf("%s.tmp.%d", destination, os.Getpid())
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", fmt.Errorf("create temporary devproxy binary: %w", err)
	}

	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("write devproxy binary: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("close temporary devproxy binary: %w", closeErr)
	}

	return tmp, nil
}

func installBinary(tmp, destination string) error {
	if runtime.GOOS == "windows" {
		// Windows cannot rename over an existing file, so an outdated binary
		// must be removed before the new one is moved into place.
		if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove outdated devproxy binary: %w", err)
		}
	}

	if err := os.Rename(tmp, destination); err != nil {
		return fmt.Errorf("install devproxy binary: %w", err)
	}

	return nil
}

func fetchChecksum(url, asset string) (string, error) {
	var expected string

	err := withDownloadRetry(func() error {
		response, err := metaClient.Get(url)
		if err != nil {
			return err
		}
		defer response.Body.Close()

		if response.StatusCode < 200 || response.StatusCode > 299 {
			return downloadStatusError(response.StatusCode)
		}

		data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if err != nil {
			return err
		}

		hash, ok := checksumForAsset(string(data), asset)
		if !ok {
			return fmt.Errorf("no checksum found for %s in checksums.txt", asset)
		}

		expected = hash
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("fetch devproxy checksums: %w", err)
	}

	return expected, nil
}

func withDownloadRetry(download func() error) error {
	var lastErr error

	for attempt := 1; attempt <= downloadAttempts; attempt++ {
		lastErr = download()
		if lastErr == nil {
			return nil
		}
		if attempt == downloadAttempts || !isRetryableDownloadError(lastErr) {
			break
		}

		fmt.Fprintf(os.Stderr, "download attempt %d/%d failed: %v — retrying\n", attempt, downloadAttempts, lastErr)
		time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
	}

	return lastErr
}

type downloadStatusError int

func (e downloadStatusError) Error() string {
	if int(e) == http.StatusNotFound {
		return "asset not found (404): check that the release tag and asset name are published"
	}

	return fmt.Sprintf("server returned %s (%d)", http.StatusText(int(e)), int(e))
}

func isRetryableDownloadError(err error) bool {
	var statusErr downloadStatusError
	if errors.As(err, &statusErr) {
		status := int(statusErr)
		return status == http.StatusTooManyRequests || status >= 500
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	message := strings.ToLower(err.Error())
	for _, pattern := range []string{
		"connection reset",
		"reset by peer",
		"unexpected eof",
		"broken pipe",
		"temporary",
	} {
		if strings.Contains(message, pattern) {
			return true
		}
	}

	return false
}

func cachedBinaryPath(version string) string {
	return filepath.Join(cacheDirName, version, binDirName, assetName())
}

func assetName() string {
	name := fmt.Sprintf("devproxy-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	return name
}

func downloadBaseURL() string {
	baseURL := strings.TrimRight(os.Getenv("DEVPROXY_DOWNLOAD_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return baseURL
}

func checksumForAsset(checksums string, asset string) (string, bool) {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name := strings.TrimPrefix(fields[1], "*")
		if filepath.Base(name) == asset {
			return fields[0], true
		}
	}

	return "", false
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open downloaded devproxy binary: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash downloaded devproxy binary: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func ensureGitignoreEntry() error {
	const entry = ".devproxy/"
	path := ".gitignore"

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return os.WriteFile(path, []byte(entry+"\n"), 0o644)
	}
	if err != nil {
		return fmt.Errorf("read .gitignore: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == entry || strings.TrimSpace(line) == cacheDirName {
			return nil
		}
	}

	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry + "\n"

	return os.WriteFile(path, []byte(content), 0o644)
}

func execDevProxy(binaryPath string, args []string) error {
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return cmd.Run()
}

// versionFromConfig reads an optional top-level version key from config.yaml.
// It intentionally uses a small parser so the bootstrapper has no YAML dependency.
func versionFromConfig(path string) string {
	if path == "" {
		path = "config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "version:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
			return strings.TrimSpace(strings.Trim(value, "\"'"))
		}
	}
	return ""
}
