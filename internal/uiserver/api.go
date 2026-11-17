package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/logstore"
	"github.com/phy0hk/devproxy/internal/process"
)

// routeInfo is the dashboard-safe view of one configured proxy target.
type routeInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Upstream string `json:"upstream"`
	Rewrite  bool   `json:"rewrite"`
}

func handleRoutes(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		routes := make([]routeInfo, 0, len(cfg.Proxy.Targets))
		for _, target := range cfg.Proxy.Targets {
			routes = append(routes, routeInfo{
				Name:     target.Name,
				Path:     target.Path,
				Upstream: target.URL,
				Rewrite:  target.Rewrite,
			})
		}

		writeJSON(w, routes)
	}
}

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
		name, action, ok := parseProcessActionPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}

		if action == "logs" {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			handleProcessLogs(w, r, manager, name)
			return
		}

		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

func handleProcessLogs(
	w http.ResponseWriter,
	r *http.Request,
	manager *process.Manager,
	name string,
) {
	query := r.URL.Query()
	run := query.Get("run")

	limit := 1000
	if rawLimit := query.Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 {
			http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	before := uint64(0)
	rawBefore := query.Get("before")
	if rawBefore == "" {
		rawBefore = query.Get("before-sequence")
	}
	if rawBefore != "" {
		parsed, err := strconv.ParseUint(rawBefore, 10, 64)
		if err != nil {
			http.Error(w, "before must be an unsigned integer", http.StatusBadRequest)
			return
		}
		before = parsed
	}

	result, err := manager.Logs(name, run, limit, before)
	if err != nil {
		if errors.Is(err, logstore.ErrRunNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, result)
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
