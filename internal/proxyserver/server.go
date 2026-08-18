package proxyserver

import (
	"fmt"
	"log"
	"net/http"

	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/proxy"
)

type Server struct {
	httpServer *http.Server
}

func New(cfg *config.Config) (*Server, error) {
	proxyHandler, err := proxy.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create proxy: %w", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", proxyHandler.Handler)

	addr := fmt.Sprintf(
		"%s:%d",
		cfg.Server.Proxy.Host,
		cfg.Server.Proxy.Port,
	)

	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}, nil
}

func (s *Server) Start() error {
	log.Printf(
		"proxy server listening on http://%s",
		s.httpServer.Addr,
	)

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	return s.httpServer.Close()
}
