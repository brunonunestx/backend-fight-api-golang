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

	fmt.Println("Reading HKM index file")
	ivfIndex, cleanup, err := pkg.ReadHKM("../resources/index.ivf")
	if err != nil {
		panic(err)
	}
	_ = cleanup

	fmt.Printf("Loaded HKM tree: depth=%d branch=%d nodes=%d buckets=%d\n",
		ivfIndex.Depth, ivfIndex.Branch, len(ivfIndex.Centroids), len(ivfIndex.Buckets))
	fraudHandler := fraud.NewHandler(ivfIndex)
	mux.HandleFunc("/fraud-score", fraudHandler.DetectFraud)

	return mux
}
