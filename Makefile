.PHONY: all build test test-integration swagger docker-up docker-down

BINARY  := activation-service
CMD_DIR := ./cmd/server

all: build

build:
	go build -o $(BINARY) $(CMD_DIR)

test:
	go test -v ./...

test-integration:
	go test -v -count=1 -timeout 120s ./test/integration/

swagger:
	swag init -g $(CMD_DIR)/main.go -o docs

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down -v
