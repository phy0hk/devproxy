package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/phy0hk/devproxy/internal/process"
)

func handleProcessStatuses(manager *process.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		writeJSON(w, manager.Statuses())
	}
}

func handleProcessAction(manager *process.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name, action, ok := parseProcessActionPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var err error
		switch action {
		case "start":
			err = manager.Start(ctx, name)
		case "stop":
			err = manager.Stop(ctx, name)
		case "restart":
			err = manager.Restart(ctx, name)
		default:
			http.NotFound(w, r)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeJSON(w, manager.Statuses())
	}
}

func parseProcessActionPath(path string) (string, string, bool) {
	path = strings.TrimPrefix(path, "/api/processes/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	name, err := url.PathUnescape(parts[0])
	if err != nil {
		return "", "", false
	}

	return name, parts[1], true
}

func writeJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "encode json", http.StatusInternalServerError)
	}
}
