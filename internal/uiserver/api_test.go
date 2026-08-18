package server

import "testing"

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
