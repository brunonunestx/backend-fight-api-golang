package server

import (
	"fmt"
	"net/http"

	"core-api/internal/modules/fraud"
	"core-api/pkg"
)

func (s *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	fmt.Println("Reading IVF file")
	ivfIndex, cleanup, err := pkg.ReadIVF("../resources/index.ivf")
	if err != nil {
		panic(err)
	}
	_ = cleanup

	fmt.Printf("Loaded IVF index with %d centroids and %d clusters\n", len(ivfIndex.Centroids), len(ivfIndex.Clusters))
	fraudHandler := fraud.NewHandler(ivfIndex)
	mux.HandleFunc("/fraud-score", fraudHandler.DetectFraud)

	return mux
}
