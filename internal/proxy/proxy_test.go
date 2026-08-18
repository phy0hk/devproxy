package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/phy0hk/devproxy/internal/config"
)

func TestMatchesPath(t *testing.T) {

	tests := []struct {
		name        string
		requestPath string
		routePath   string
		want        bool
	}{
		{
			name:        "exact match",
			requestPath: "/api",
			routePath:   "/api",
			want:        true,
		},
		{
			name:        "child path",
			requestPath: "/api/users",
			routePath:   "/api",
			want:        true,
		},
		{
			name:        "deep child path",
			requestPath: "/api/users/123",
			routePath:   "/api",
			want:        true,
		},
		{
			name:        "similar prefix",
			requestPath: "/apixxx",
			routePath:   "/api",
			want:        false,
		},
		{
			name:        "different path",
			requestPath: "/users",
			routePath:   "/api",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPath(
				tt.requestPath,
				tt.routePath,
			)
			if got != tt.want {
				t.Fatalf("matchesPath(%q, %q) = %v, want %v", tt.requestPath, tt.routePath, got, tt.want)
			}
		})
	}
}
func TestFindRouteLongestMatch(t *testing.T) {
	proxy := &Proxy{
		routes: []Route{
			{
				Path: "/api",
			},
			{
				Path: "/api/users",
			},
		},
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "api route",
			path: "/api/products",
			want: "/api",
		},
		{
			name: "users route",
			path: "/api/users/123",
			want: "/api/users",
		},
		{
			name: "no route",
			path: "/other",
			want: "",
		},
		{
			name: "similar prefix is not a match",
			path: "/apixxx",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := proxy.findRoute(tt.path)

			if tt.want == "" {
				if route != nil {
					t.Fatalf(
						"expected no route, got %q",
						route.Path,
					)
				}

				return
			}

			if route == nil {
				t.Fatalf(
					"expected route %q, got nil",
					tt.want,
				)
			}

			if route.Path != tt.want {
				t.Fatalf(
					"got route %q, want %q",
					route.Path,
					tt.want,
				)
			}
		})
	}
}
func TestProxy(t *testing.T) {
	tests := []struct {
		name         string
		rewrite      bool
		requestPath  string
		expectedPath string
	}{
		{
			name:         "rewrite path",
			rewrite:      true,
			requestPath:  "/api/users",
			expectedPath: "/users",
		},
		{
			name:         "preserve path",
			rewrite:      false,
			requestPath:  "/api/users",
			expectedPath: "/api/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(
				http.HandlerFunc(func(
					w http.ResponseWriter,
					r *http.Request,
				) {
					w.WriteHeader(http.StatusOK)

					_, _ = w.Write(
						[]byte(r.URL.Path),
					)
				}),
			)

			defer upstream.Close()

			cfg := &config.Config{
				Proxy: config.ProxyConfig{
					Targets: []config.TargetConfig{
						{
							Name:    "test",
							Path:    "/api",
							URL:     upstream.URL,
							Rewrite: tt.rewrite,
						},
					},
				},
			}

			proxy, err := New(cfg)
			if err != nil {
				t.Fatalf("create proxy: %v", err)
			}

			req := httptest.NewRequest(
				http.MethodGet,
				"http://localhost"+tt.requestPath,
				nil,
			)

			rec := httptest.NewRecorder()

			proxy.Handler(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf(
					"got status %d, want %d",
					rec.Code,
					http.StatusOK,
				)
			}

			if body := rec.Body.String(); body != tt.expectedPath {
				t.Fatalf(
					"got body %q, want %q",
					body,
					tt.expectedPath,
				)
			}
		})
	}
}
