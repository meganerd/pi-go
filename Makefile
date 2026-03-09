# pi-go — cross-compilation Makefile
#
# Usage:
#   make                       Build for host (dynamic)
#   make static                Build for host (static, CGO_ENABLED=0)
#   make linux-arm64           Build for linux/arm64
#   make linux-arm64-static    Build for linux/arm64 (static)
#   make darwin-amd64          Build for darwin/amd64
#   make all                   Build all platforms (dynamic)
#   make all-static            Build all platforms (static)
#   make test                  Run tests
#   make clean                 Remove build artifacts
#   make help                  Show all targets

SHELL := /bin/bash

# ─── Variables ───────────────────────────────────────────────────────────────

VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE     := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
PKG      := github.com/meganerd/pi-go/internal/version
LDFLAGS  := -ldflags "-X $(PKG).version=$(VERSION) -X $(PKG).commit=$(COMMIT) -X $(PKG).date=$(DATE)"

BINARY   := pi-go

# Supported platforms (os/arch)
PLATFORMS := linux/amd64 linux/arm64 linux/riscv64 linux/ppc64 linux/ppc64le \
             darwin/amd64 darwin/arm64 \
             windows/amd64 windows/arm64

# Derive make-friendly target names (os-arch)
TARGETS   := $(subst /,-,$(PLATFORMS))

# ─── Internal build function ────────────────────────────────────────────────

# $(1) = GOOS, $(2) = GOARCH, $(3) = CGO_ENABLED (0 or 1)
define do-build
	@mkdir -p build/$(1)-$(2)
	@ext=""; \
	[ "$(1)" = "windows" ] && ext=".exe"; \
	echo "build  $(1)/$(2)  $(BINARY)$$ext  $(if $(filter 0,$(3)),[static],[dynamic])"; \
	CGO_ENABLED=$(3) GOOS=$(1) GOARCH=$(2) \
		go build $(LDFLAGS) -o build/$(1)-$(2)/$(BINARY)$$ext ./cmd/pi-go
endef

# ─── Host build (default) ───────────────────────────────────────────────────

.PHONY: build
build:
	@echo "build  $$(go env GOOS)/$$(go env GOARCH)  $(BINARY)  [dynamic]"
	@go build $(LDFLAGS) -o $(BINARY) ./cmd/pi-go

.PHONY: static
static:
	@echo "build  $$(go env GOOS)/$$(go env GOARCH)  $(BINARY)  [static]"
	@CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY) ./cmd/pi-go

# ─── Per-platform targets ──────────────────────────────────────────────────

# Generate targets: make linux-arm64, make linux-arm64-static, etc.
# $(1) = os, $(2) = arch
define platform-targets
.PHONY: $(1)-$(2) $(1)-$(2)-static
$(1)-$(2):
	$$(call do-build,$(1),$(2),1)
$(1)-$(2)-static:
	$$(call do-build,$(1),$(2),0)
endef

$(foreach p,$(PLATFORMS),$(eval $(call platform-targets,$(word 1,$(subst /, ,$(p))),$(word 2,$(subst /, ,$(p))))))

# ─── All platforms ─────────────────────────────────────────────────────────

.PHONY: all
all: $(TARGETS)

.PHONY: all-static
all-static: $(addsuffix -static,$(TARGETS))

# ─── Distribution (tarballs + checksums) ─────────────────────────────────────

.PHONY: dist
dist: all-static
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		slug=$${platform//\//-}; \
		tar -czf dist/$(BINARY)-$${slug}.tar.gz -C build $${slug}; \
	done
	@cd dist && sha256sum $(BINARY)-*.tar.gz > sha256sums.txt
	@echo ""
	@echo "checksums:"
	@cat dist/sha256sums.txt

# ─── Test targets ────────────────────────────────────────────────────────────

.PHONY: test
test:
	go test -race -cover ./...

.PHONY: cover
cover:
	go test -race -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html

.PHONY: integration
integration:
	go test -tags integration -v -run TestIntegration ./...

# ─── Lint ────────────────────────────────────────────────────────────────────

.PHONY: lint
lint:
	golangci-lint run ./...

# ─── Install ─────────────────────────────────────────────────────────────────

.PHONY: install
install:
	go install $(LDFLAGS) ./cmd/pi-go

# ─── Clean ───────────────────────────────────────────────────────────────────

.PHONY: clean
clean:
	rm -rf build dist $(BINARY)
	go clean -testcache

# ─── Help ────────────────────────────────────────────────────────────────────

.PHONY: help
help:
	@echo "pi-go build targets:"
	@echo ""
	@echo "  make                       Build for host (dynamic)"
	@echo "  make static                Build for host (static, CGO_ENABLED=0)"
	@echo ""
	@echo "  make <os>-<arch>           Build for specific platform"
	@echo "  make <os>-<arch>-static    Build for specific platform (static)"
	@echo ""
	@echo "  Platforms:"
	@for p in $(PLATFORMS); do \
		slug=$${p//\//-}; \
		echo "    make $$slug          make $${slug}-static"; \
	done
	@echo ""
	@echo "  make all                   Build all platforms (dynamic)"
	@echo "  make all-static            Build all platforms (static)"
	@echo "  make dist                  Build static + tarballs + checksums"
	@echo ""
	@echo "  make test                  Run tests"
	@echo "  make cover                 Run tests with coverage report"
	@echo "  make integration           Run integration tests (needs API key)"
	@echo "  make lint                  Run golangci-lint"
	@echo "  make install               Install to GOPATH/bin"
	@echo "  make clean                 Remove build/, dist/, and binary"
