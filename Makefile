.PHONY: build run clean test lint

VERSION ?= dev

# Build
build:
	go build -ldflags "-X main.version=$(VERSION)" -o scinfra-bot ./cmd/bot

# Run locally
run:
	go run ./cmd/bot -config configs/config.yaml

# Build for Linux (cross-compile)
build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o dist/scinfra-bot-linux-amd64 ./cmd/bot
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o dist/scinfra-bot-linux-arm64 ./cmd/bot

# Clean
clean:
	rm -f scinfra-bot
	rm -rf dist/

# Test
test:
	go test -v ./...

# Lint
lint:
	golangci-lint run

# Dependencies
deps:
	go mod download
	go mod tidy

# Generate checksums
checksums:
	cd dist && sha256sum * > checksums.txt
