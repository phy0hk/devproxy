package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/phy0hk/devproxy/internal/config"
)

type Server struct {
	config *config.Config
}

func New(cfg *config.Config) *Server {
	return &Server{
		config: cfg,
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(
		"%s:%d",
		s.config.Server.Host,
		s.config.Server.Port,
	)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("devproxy 🚀"))
	})

	log.Printf("server listening on http://%s", addr)

	return http.ListenAndServe(addr, mux)
}
