package fraud

import (
	"encoding/json"
	"net/http"

	"core-api/pkg"
)

type Handler struct {
	service *Service
}

func NewHandler(ivfIndex pkg.IVFIndex) *Handler {
	service := NewService(ivfIndex)
	return &Handler{service: service}
}

func (h *Handler) DetectFraud(w http.ResponseWriter, r *http.Request) {
	var transaction Transaction
	if err := json.NewDecoder(r.Body).Decode(&transaction); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	response := h.service.DetectFraud(transaction)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
