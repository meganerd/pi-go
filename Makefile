BINARY := pi-go
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

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
