package server

import (
	"log"
	"net/http"

	"openailogger/internal/api"
	"openailogger/internal/config"
	"openailogger/internal/proxy"
	"openailogger/storage"
)

// Server represents the main HTTP server
type Server struct {
	config  *config.Config
	gateway *proxy.Gateway
	api     *api.Handler
}

// New creates a new server instance
func New(cfg *config.Config, store storage.Store) *Server {
	return &Server{
		config:  cfg,
		gateway: proxy.New(cfg, store),
		api:     api.New(store),
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register API routes first
	s.api.RegisterRoutes(mux)

	// Register provider proxy routes before the catch-all static handler
	for _, route := range s.config.Routes {
		pattern := route.Mount + "/"
		mux.Handle(pattern, s.gateway)
		log.Printf("Registered proxy route: %s -> %s", pattern, route.Upstream)
	}

	// Serve static UI files (this should be last as it's a catch-all)
	staticHandler := http.FileServer(http.Dir("ui/"))
	mux.Handle("/", staticHandler)

	log.Printf("Starting server on %s", s.config.Address())
	log.Printf("UI available at: http://%s", s.config.Address())
	log.Printf("API available at: http://%s/api", s.config.Address())

	return http.ListenAndServe(s.config.Address(), mux)
}

// Close shuts down the server
func (s *Server) Close() error {
	return s.gateway.Close()
}
