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
	Path   string
	Target *httputil.ReverseProxy
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

		targetPath := target.Path

		proxy := &httputil.ReverseProxy{
			Rewrite: func(req *httputil.ProxyRequest) {
				req.SetURL(u)

				if target.Rewrite {
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
			Path:   targetPath,
			Target: proxy,
		})
	}

	return &Proxy{
		routes: routes,
	}, nil
}

func (p *Proxy) Handler(w http.ResponseWriter, r *http.Request) {
	route := p.findRoute(r.URL.Path)

	if route == nil {
		http.NotFound(w, r)
		return
	}

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
	return requestPath == routePath ||
		strings.HasPrefix(requestPath, routePath+"/")
}
