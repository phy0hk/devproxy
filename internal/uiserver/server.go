package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/event"
	"github.com/phy0hk/devproxy/internal/logger"
	"github.com/phy0hk/devproxy/internal/proxy"
)

type Server struct {
	config *config.Config
	http   *http.Server
	bus    *event.Bus
}

func New(cfg *config.Config) (*Server, error) {
	bus := event.NewBus()

	requestLogger := logger.New(bus)
	proxyServer, err := proxy.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create proxy: %w", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc(
		"/events",
		func(w http.ResponseWriter, r *http.Request) {
			handleEvents(w, r, bus)
		},
	)

	mux.Handle(
		"/",
		requestLogger.Middleware(
			http.HandlerFunc(proxyServer.Handler),
		),
	)

	return &Server{
		config: cfg,
		http: &http.Server{
			Addr: fmt.Sprintf(
				"%s:%d",
				cfg.Server.Proxy.Host,
				cfg.Server.Proxy.Port,
			),
			Handler: mux,
		},
	}, nil
}

func (s *Server) Start() error {
	log.Printf(
		"server listening on http://%s",
		s.http.Addr,
	)

	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(
		ctx,
		5*time.Second,
	)
	defer cancel()

	return s.http.Shutdown(shutdownCtx)
}
