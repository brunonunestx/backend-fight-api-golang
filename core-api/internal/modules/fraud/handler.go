package fraud

import (
	"io"
	"net/http"
	"sync"

	pkg "core-api/pkg"
)

var fraudResponses = [6][]byte{
	[]byte(`{"approved":true,"fraud_score":0}`),
	[]byte(`{"approved":true,"fraud_score":0.2}`),
	[]byte(`{"approved":true,"fraud_score":0.4}`),
	[]byte(`{"approved":false,"fraud_score":0.6}`),
	[]byte(`{"approved":false,"fraud_score":0.8}`),
	[]byte(`{"approved":false,"fraud_score":1}`),
}

var bodyPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 2048)
		return &b
	},
}

type Handler struct {
	service *Service
}

func NewHandler(idx *pkg.IVFIndex) *Handler {
	return &Handler{service: NewService(idx)}
}

func (h *Handler) DetectFraud(w http.ResponseWriter, r *http.Request) {
	bufp := bodyPool.Get().(*[]byte)
	body, err := readAll(r.Body, *bufp)
	if err != nil {
		*bufp = body[:0]
		bodyPool.Put(bufp)
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	result := h.service.DetectFraudRaw(body)

	*bufp = body[:0]
	bodyPool.Put(bufp)

	idx := int(result.Score * 5)
	if idx > 5 {
		idx = 5
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(fraudResponses[idx])
}

func readAll(r io.Reader, buf []byte) ([]byte, error) {
	buf = buf[:0]
	for {
		if len(buf) == cap(buf) {
			grown := make([]byte, cap(buf)*2)
			copy(grown, buf)
			buf = grown[:len(buf)]
		}
		n, err := r.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if err == io.EOF {
			return buf, nil
		}
		if err != nil {
			return buf, err
		}
	}
}
