package api

import (
	"database/sql"
	"fmt"
	"kolektor/config"
	"log"
	"net/http"
)

type HTTPServer struct {
	db         *sql.DB
	cfg        *config.Config
	httpServer *http.Server
}

func NewHTTPServer(cfg *config.Config, db *sql.DB) HTTPServer {
	return HTTPServer{
		db:  db,
		cfg: cfg,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf("%s:%s", cfg.HTTP.Host, cfg.HTTP.Port),
			Handler: nil,
		},
	}
}

func (s *HTTPServer) Run() {
	endpoints := s.Endpoints()
	for _, endpoint := range endpoints {
		http.HandleFunc(endpoint.Pattern, endpoint.Handler)
	}

	log.Println(s.httpServer.ListenAndServe())
}

func (s *HTTPServer) Close() error {
	return s.httpServer.Close()
}
