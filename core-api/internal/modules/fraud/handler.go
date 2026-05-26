package fraud

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler() *Handler {
	service := NewService()
	return &Handler{service: service}
}

func (h *Handler) DetectFraud(w http.ResponseWriter, r *http.Request) {
	var transaction Transaction
	if err := json.NewDecoder(r.Body).Decode(&transaction); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	h.service.DetectFraud(transaction)

	w.WriteHeader(http.StatusOK)
}
