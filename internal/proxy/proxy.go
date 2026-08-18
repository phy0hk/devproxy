package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/phy0hk/devproxy/internal/config"
)

type Route struct {
	Name     string
	Path     string
	Upstream string
	Target   *httputil.ReverseProxy
}

type Proxy struct {
	routes []Route
}

func New(cfg *config.Config) (*Proxy, error) {
	routes := make([]Route, 0, len(cfg.Proxy.Targets))

	for _, target := range cfg.Proxy.Targets {
		u, err := url.Parse(target.URL)
		if err != nil {
			return nil, fmt.Errorf(
				"parse target %q: %w",
				target.Name,
				err,
			)
		}

		targetName := target.Name
		targetPath := target.Path
		targetURL := target.URL
		targetRewrite := target.Rewrite

		proxy := &httputil.ReverseProxy{
			Rewrite: func(req *httputil.ProxyRequest) {
				req.SetURL(u)

				if targetRewrite {
					req.Out.URL.Path = strings.TrimPrefix(
						req.In.URL.Path,
						targetPath,
					)

					if req.Out.URL.Path == "" {
						req.Out.URL.Path = "/"
					}
				} else {
					req.Out.URL.Path = req.In.URL.Path
				}

				req.Out.URL.RawQuery = req.In.URL.RawQuery
			},
		}

		routes = append(routes, Route{
			Name:     targetName,
			Path:     targetPath,
			Upstream: targetURL,
			Target:   proxy,
		})
	}

	return &Proxy{
		routes: routes,
	}, nil
}

func (p *Proxy) Handler(w http.ResponseWriter, r *http.Request) {
	route := p.findRoute(r.URL.Path)

	if route == nil {
		http.Error(
			w,
			fmt.Sprintf("devproxy: no proxy route matched %s", r.URL.Path),
			http.StatusNotFound,
		)
		return
	}

	w.Header().Set("X-DevProxy-Route", route.Name)
	w.Header().Set("X-DevProxy-Upstream", route.Upstream)
	route.Target.ServeHTTP(w, r)
}
func (p *Proxy) findRoute(path string) *Route {
	var matched *Route

	for i := range p.routes {
		route := &p.routes[i]

		if !matchesPath(path, route.Path) {
			continue
		}

		if matched == nil ||
			len(route.Path) > len(matched.Path) {
			matched = route
		}
	}

	return matched
}

func matchesPath(requestPath, routePath string) bool {
	if routePath == "/" {
		return strings.HasPrefix(requestPath, "/")
	}

	return requestPath == routePath ||
		strings.HasPrefix(requestPath, routePath+"/")
}
