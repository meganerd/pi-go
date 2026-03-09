BINARY := pi-go
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
PKG := github.com/meganerd/pi-go/internal/version
LDFLAGS := -ldflags "-X $(PKG).version=$(VERSION) -X $(PKG).commit=$(COMMIT) -X $(PKG).date=$(DATE)"

.PHONY: build test lint clean install

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/pi-go

test:
	go test -race -cover ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)
	go clean -testcache

install:
	go install $(LDFLAGS) ./cmd/pi-go

cover:
	go test -race -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html
