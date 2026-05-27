package fraud

import (
	"io"
	"net/http"
	"sync"

	"core-api/pkg"
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

func NewHandler(ivfIndex pkg.HKMTree) *Handler {
	return &Handler{service: NewService(ivfIndex)}
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

	fraudCount := h.service.DetectFraudRaw(body)

	*bufp = body[:0]
	bodyPool.Put(bufp)

	w.WriteHeader(http.StatusOK)
	w.Write(fraudResponses[fraudCount])
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
