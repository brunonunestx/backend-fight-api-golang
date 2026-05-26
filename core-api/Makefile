BINARY=tmp/main
CMD=./cmd/internal

.PHONY: build run watch

build:
	go build -o $(BINARY) $(CMD)

run:
	go run $(CMD)

watch:
	air
