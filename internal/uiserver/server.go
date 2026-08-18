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
	"github.com/phy0hk/devproxy/internal/process"
	"github.com/phy0hk/devproxy/internal/proxy"
)

type Server struct {
	config         *config.Config
	proxyHTTP      *http.Server
	uiHTTP         *http.Server
	bus            *event.Bus
	processManager *process.Manager
}

func New(cfg *config.Config) (*Server, error) {
	bus := event.NewBus()

	requestLogger := logger.New(bus)
	proxyServer, err := proxy.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create proxy: %w", err)
	}

	proxyMux := http.NewServeMux()
	proxyMux.Handle(
		"/",
		requestLogger.Middleware(
			http.HandlerFunc(proxyServer.Handler),
		),
	)

	processManager := process.NewManager(cfg.Processes, bus)

	uiMux := http.NewServeMux()
	uiMux.Handle("/static/", handleStaticAssets())
	uiMux.HandleFunc("/", handleDashboard)
	uiMux.HandleFunc("/api/processes", handleProcessStatuses(processManager))
	uiMux.HandleFunc("/api/processes/", handleProcessAction(processManager))
	uiMux.HandleFunc(
		"/events",
		func(w http.ResponseWriter, r *http.Request) {
			handleEvents(w, r, bus)
		},
	)

	return &Server{
		config:         cfg,
		bus:            bus,
		processManager: processManager,
		proxyHTTP: &http.Server{
			Addr: fmt.Sprintf(
				"%s:%d",
				cfg.Server.Proxy.Host,
				cfg.Server.Proxy.Port,
			),
			Handler: proxyMux,
		},
		uiHTTP: &http.Server{
			Addr: fmt.Sprintf(
				"%s:%d",
				cfg.Server.UI.Host,
				cfg.Server.UI.Port,
			),
			Handler: uiMux,
		},
	}, nil
}

func (s *Server) Start() error {
	if err := s.processManager.StartAll(context.Background()); err != nil {
		return fmt.Errorf("start processes: %w", err)
	}

	errCh := make(chan error, 2)

	go func() {
		log.Printf(
			"ui server listening on http://%s",
			s.uiHTTP.Addr,
		)
		errCh <- s.uiHTTP.ListenAndServe()
	}()

	go func() {
		log.Printf(
			"proxy server listening on http://%s",
			s.proxyHTTP.Addr,
		)
		errCh <- s.proxyHTTP.ListenAndServe()
	}()

	return <-errCh
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(
		ctx,
		5*time.Second,
	)
	defer cancel()

	processErr := s.processManager.StopAll(shutdownCtx)
	proxyErr := s.proxyHTTP.Shutdown(shutdownCtx)
	uiErr := s.uiHTTP.Shutdown(shutdownCtx)

	if processErr != nil {
		return processErr
	}

	if proxyErr != nil {
		return proxyErr
	}

	return uiErr
}
