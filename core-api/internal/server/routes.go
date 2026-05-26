package server

import (
	"net/http"

	"core-api/internal/modules/fraud"
)

func (s *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	fraudHandler := fraud.NewHandler()
	mux.HandleFunc("/fraud-score", fraudHandler.DetectFraud)

	return mux
}
