FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY core-api/go.mod ./
RUN go mod download

COPY core-api/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /api ./cmd/internal/main.go

FROM alpine:latest

COPY --from=builder /api /api
COPY resources/ /resources/

EXPOSE 8080

ENTRYPOINT ["/api"]
