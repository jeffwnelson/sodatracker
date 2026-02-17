.PHONY: build build-server build-piagent test lint run clean docker-up docker-down

BINARY_DIR := bin
SERVER_BIN := $(BINARY_DIR)/server
PIAGENT_BIN := $(BINARY_DIR)/piagent
PIAGENT_ARM_BIN := $(BINARY_DIR)/piagent-arm

build: build-server build-piagent

build-server:
	go build -o $(SERVER_BIN) ./cmd/server

build-piagent:
	go build -o $(PIAGENT_BIN) ./cmd/piagent

build-piagent-arm:
	GOOS=linux GOARCH=arm GOARM=7 go build -tags rpi -o $(PIAGENT_ARM_BIN) ./cmd/piagent

test:
	go test ./...

lint:
	golangci-lint run ./...

run: build-server
	./$(SERVER_BIN)

clean:
	rm -rf $(BINARY_DIR)

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build
