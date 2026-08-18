package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/process"
	server "github.com/phy0hk/devproxy/internal/uiserver"
)

var version = "dev"

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	command := "up"
	if len(args) > 0 && !isFlag(args[0]) {
		command = args[0]
		args = args[1:]
	}

	switch command {
	case "up", "run":
		return runUp(args)
	case "process", "processes", "ps", "status":
		return runProcess(args)
	case "validate", "check":
		return runValidate(args)
	case "version", "--version", "-version":
		fmt.Printf("devproxy %s\n", version)
		return nil
	case "help", "--help", "-help", "-h":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func runUp(args []string) error {
	flags := flag.NewFlagSet("up", flag.ExitOnError)
	configPath := flags.String(
		"config",
		"config.yaml",
		"path to configuration file",
	)
	tui := flags.Bool(
		"tui",
		false,
		"enable terminal user interface",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *tui {
		log.Println("tui mode is not implemented yet; streaming logs to the terminal")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	srv, err := server.New(cfg)
	if err != nil {
		return err
	}

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- srv.Start()
	}()

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	select {
	case err := <-serverErr:
		if isNormalServerClose(err) {
			return nil
		}

		return err

	case <-signalCtx.Done():
		if err := srv.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}

		return nil
	}
}

func runProcess(args []string) error {
	command := "list"
	if len(args) > 0 && !isFlag(args[0]) {
		command = args[0]
		args = args[1:]
	}

	flags := flag.NewFlagSet("process", flag.ExitOnError)
	configPath := flags.String(
		"config",
		"config.yaml",
		"path to configuration file used to discover the UI server URL",
	)
	uiURL := flags.String(
		"ui-url",
		"",
		"DevProxy UI/API base URL, for example http://127.0.0.1:8081",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}

	baseURL, err := resolveUIURL(*uiURL, *configPath)
	if err != nil {
		return err
	}

	switch command {
	case "list", "ls", "status":
		statuses, err := fetchProcessStatuses(baseURL)
		if err != nil {
			return err
		}
		printProcessStatuses(statuses)
		return nil
	case "start", "stop", "restart":
		if flags.NArg() != 1 {
			return fmt.Errorf("usage: devproxy process %s <name> [--config config.yaml] [--ui-url url]", command)
		}

		statuses, err := runProcessAction(baseURL, flags.Arg(0), command)
		if err != nil {
			return err
		}
		printProcessStatuses(statuses)
		return nil
	default:
		return fmt.Errorf("unknown process command %q", command)
	}
}

func runValidate(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := flags.String(
		"config",
		"config.yaml",
		"path to configuration file",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}

	if _, err := config.Load(*configPath); err != nil {
		return err
	}

	fmt.Printf("%s is valid\n", *configPath)
	return nil
}

func resolveUIURL(explicitURL string, configPath string) (string, error) {
	if explicitURL != "" {
		return strings.TrimRight(explicitURL, "/"), nil
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("http://%s:%d", cfg.Server.UI.Host, cfg.Server.UI.Port), nil
}

func fetchProcessStatuses(baseURL string) ([]process.Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/processes", nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connect to devproxy UI API at %s: %w", baseURL, err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("fetch process statuses: %s", response.Status)
	}

	var statuses []process.Status
	if err := json.NewDecoder(response.Body).Decode(&statuses); err != nil {
		return nil, fmt.Errorf("decode process statuses: %w", err)
	}

	return statuses, nil
}

func runProcessAction(baseURL string, name string, action string) ([]process.Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("%s/api/processes/%s/%s", baseURL, url.PathEscape(name), action)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connect to devproxy UI API at %s: %w", baseURL, err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("%s process %q: %s", action, name, response.Status)
	}

	var statuses []process.Status
	if err := json.NewDecoder(response.Body).Decode(&statuses); err != nil {
		return nil, fmt.Errorf("decode process statuses: %w", err)
	}

	return statuses, nil
}

func printProcessStatuses(statuses []process.Status) {
	if len(statuses) == 0 {
		fmt.Println("no processes configured")
		return
	}

	fmt.Printf("%-20s %-12s %-8s %s\n", "NAME", "STATE", "PID", "COMMAND")
	for _, status := range statuses {
		pid := "-"
		if status.PID != 0 {
			pid = fmt.Sprintf("%d", status.PID)
		}

		fmt.Printf(
			"%-20s %-12s %-8s %s\n",
			status.Name,
			status.State,
			pid,
			status.Command,
		)
	}
}

func isNormalServerClose(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, os.ErrClosed) ||
		errors.Is(err, http.ErrServerClosed)
}

func isFlag(arg string) bool {
	return len(arg) > 0 && arg[0] == '-'
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `DevProxy - reverse proxy and process runner for local development

Usage:
  devproxy [up] [--config config.yaml] [--tui]
  devproxy process [list] [--config config.yaml] [--ui-url url]
  devproxy process start <name> [--config config.yaml] [--ui-url url]
  devproxy process stop <name> [--config config.yaml] [--ui-url url]
  devproxy process restart <name> [--config config.yaml] [--ui-url url]
  devproxy validate [--config config.yaml]
  devproxy version
  devproxy help

Commands:
  up        Start configured processes, proxy server, and web UI. Default command.
  process   List, start, stop, or restart processes through the running UI API.
  validate  Validate the configuration file and exit.
  version   Print version information.
  help      Show this help message.

Flags:
  --config  Path to config file. Default: config.yaml
  --ui-url  UI/API base URL for process commands. Defaults to server.ui from config.
  --tui     Reserved for terminal UI mode. Logs stream to terminal for now.

`)
}
