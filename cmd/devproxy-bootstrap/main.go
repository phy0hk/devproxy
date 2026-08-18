package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	cacheDirName       = ".devproxy"
	binDirName         = "bin"
	defaultDownloadURL = "https://github.com/phy0hk/devproxy/releases/latest/download"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if err := ensureGitignoreEntry(); err != nil {
		return err
	}

	binaryPath, err := ensureBinary()
	if err != nil {
		return err
	}

	return execDevProxy(binaryPath, args)
}

func ensureBinary() (string, error) {
	binaryPath := cachedBinaryPath()
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check cached devproxy binary: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		return "", fmt.Errorf("create devproxy cache directory: %w", err)
	}

	if err := downloadBinary(downloadURL(), binaryPath); err != nil {
		return "", err
	}

	if err := verifyDownloadedBinary(binaryPath); err != nil {
		_ = os.Remove(binaryPath)
		return "", err
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(binaryPath, 0o755); err != nil {
			return "", fmt.Errorf("mark devproxy binary executable: %w", err)
		}
	}

	return binaryPath, nil
}

func cachedBinaryPath() string {
	return filepath.Join(cacheDirName, binDirName, assetName())
}

func assetName() string {
	name := fmt.Sprintf("devproxy-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	return name
}

func downloadURL() string {
	return downloadBaseURL() + "/" + assetName()
}

func checksumURL() string {
	return downloadBaseURL() + "/checksums.txt"
}

func downloadBaseURL() string {
	baseURL := strings.TrimRight(os.Getenv("DEVPROXY_DOWNLOAD_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = defaultDownloadURL
	}

	return baseURL
}

func downloadBinary(url string, destination string) error {
	fmt.Fprintf(os.Stderr, "devproxy binary not found, downloading %s\n", url)

	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download devproxy binary: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("download devproxy binary: %s", response.Status)
	}

	tmp := destination + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create temporary devproxy binary: %w", err)
	}

	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write devproxy binary: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close temporary devproxy binary: %w", closeErr)
	}

	if err := os.Rename(tmp, destination); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install devproxy binary: %w", err)
	}

	return nil
}

func verifyDownloadedBinary(path string) error {
	if os.Getenv("DEVPROXY_SKIP_CHECKSUM") == "1" {
		fmt.Fprintln(os.Stderr, "warning: checksum verification skipped")
		return nil
	}

	checksums, err := downloadText(checksumURL())
	if err != nil {
		return fmt.Errorf("verify devproxy binary: download checksums.txt: %w", err)
	}

	expected, ok := checksumForAsset(checksums, assetName())
	if !ok {
		return fmt.Errorf("verify devproxy binary: no checksum found for %s", assetName())
	}

	actual, err := sha256File(path)
	if err != nil {
		return err
	}

	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("verify devproxy binary: checksum mismatch for %s", assetName())
	}

	return nil
}

func downloadText(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("%s", response.Status)
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
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
