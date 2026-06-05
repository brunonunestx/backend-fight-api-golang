package server

import (
	"fmt"
	"net/http"
	"os"

	"core-api/internal/modules/fraud"
	pkg "core-api/pkg"
)

func (s *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	ivfPath := os.Getenv("IVF_PATH")
	if ivfPath == "" {
		ivfPath = "resources/index.ivf"
	}
	fmt.Printf("loading IVF index from %s\n", ivfPath)

	idx, err := pkg.LoadIVF(ivfPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load IVF index: %v", err))
	}
	fmt.Println("IVF index loaded")

	fraudHandler := fraud.NewHandler(idx)
	mux.HandleFunc("/fraud-score", fraudHandler.DetectFraud)

	return mux
}
