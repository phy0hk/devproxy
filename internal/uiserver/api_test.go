package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/logstore"
	"github.com/phy0hk/devproxy/internal/process"
)

func TestParseProcessActionPath(t *testing.T) {
	name, action, ok := parseProcessActionPath("/api/processes/frontend/restart")
	if !ok {
		t.Fatal("expected path to parse")
	}

	if name != "frontend" {
		t.Fatalf("name = %q, want frontend", name)
	}

	if action != "restart" {
		t.Fatalf("action = %q, want restart", action)
	}
}

func TestParseProcessActionPathUnescapesName(t *testing.T) {
	name, action, ok := parseProcessActionPath("/api/processes/api%20server/stop")
	if !ok {
		t.Fatal("expected path to parse")
	}

	if name != "api server" {
		t.Fatalf("name = %q, want api server", name)
	}

	if action != "stop" {
		t.Fatalf("action = %q, want stop", action)
	}
}

func TestParseProcessActionPathRejectsInvalidPath(t *testing.T) {
	_, _, ok := parseProcessActionPath("/api/processes/frontend")
	if ok {
		t.Fatal("expected path to be rejected")
	}
}

func TestHandleProcessLogsPagesHistory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell command")
	}

	manager := process.NewManagerWithOptions(
		[]config.ProcessConfig{{
			Name:    "worker",
			Command: "printf 'one\\ntwo\\nthree\\nfour\\n'",
		}},
		nil,
		process.Options{LogCapacity: 100, MaxRuns: 2},
	)
	if err := manager.Start(context.Background(), "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer manager.StopAll(context.Background())
	waitForLogLines(t, manager, 4)

	handler := handleProcessAction(manager)

	first := httptest.NewRecorder()
	handler.ServeHTTP(
		first,
		httptest.NewRequest(http.MethodGet, "/api/processes/worker/logs?limit=2", nil),
	)
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", first.Code, first.Body.String())
	}

	var page logstore.Result
	if err := json.NewDecoder(first.Body).Decode(&page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(page.Lines))
	}
	if page.Lines[0].Message != "three" || page.Lines[1].Message != "four" {
		t.Fatalf("lines = %v, want newest page [three four]", page.Lines)
	}
	if !page.HasMore || page.NextBefore != page.Lines[0].Sequence {
		t.Fatalf("paging metadata = has_more %v next_before %d", page.HasMore, page.NextBefore)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(
		second,
		httptest.NewRequest(
			http.MethodGet,
			"/api/processes/worker/logs?limit=2&before="+uintString(page.NextBefore),
			nil,
		),
	)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200: %s", second.Code, second.Body.String())
	}

	var older logstore.Result
	if err := json.NewDecoder(second.Body).Decode(&older); err != nil {
		t.Fatalf("decode older: %v", err)
	}
	if len(older.Lines) != 2 || older.Lines[0].Message != "one" || older.Lines[1].Message != "two" {
		t.Fatalf("older lines = %v, want [one two]", older.Lines)
	}
	if older.HasMore {
		t.Fatal("expected has_more to be false on the first page")
	}
}

func TestHandleRoutesReturnsConfiguredTargets(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Targets: []config.TargetConfig{
			{Name: "api", Path: "/api", URL: "http://127.0.0.1:9000", Rewrite: true},
			{Name: "web", Path: "/", URL: "http://127.0.0.1:3000"},
		}},
	}

	response := httptest.NewRecorder()
	handleRoutes(cfg).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/routes", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}

	var routes []routeInfo
	if err := json.NewDecoder(response.Body).Decode(&routes); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("got %d routes, want 2", len(routes))
	}
	if routes[0].Name != "api" || !routes[0].Rewrite || routes[0].Upstream != "http://127.0.0.1:9000" {
		t.Fatalf("unexpected first route: %+v", routes[0])
	}
	if routes[1].Name != "web" || routes[1].Rewrite {
		t.Fatalf("unexpected second route: %+v", routes[1])
	}

	method := httptest.NewRecorder()
	handleRoutes(cfg).ServeHTTP(
		method,
		httptest.NewRequest(http.MethodPost, "/api/routes", nil),
	)
	if method.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status = %d, want 405", method.Code)
	}
}

func TestHandleProcessLogsRejectsUnknownProcess(t *testing.T) {
	manager := process.NewManager(nil, nil)
	handler := handleProcessAction(manager)

	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/processes/missing/logs", nil),
	)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestHandleProcessLogsRejectsInvalidBefore(t *testing.T) {
	manager := process.NewManager(
		[]config.ProcessConfig{{Name: "worker", Command: "true"}},
		nil,
	)
	handler := handleProcessAction(manager)

	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/processes/worker/logs?before=nope", nil),
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func waitForLogLines(t *testing.T, manager *process.Manager, count int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		result, err := manager.Logs("worker", "current", 50, 0)
		if err == nil && len(result.Lines) >= count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d log lines", count)
}

func uintString(value uint64) string {
	if value == 0 {
		return "0"
	}

	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
