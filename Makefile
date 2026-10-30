# tg-dlp Makefile

APP_NAME := tg-dlp
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE     ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  := -s -w \
	-X 'tg-dlp/pkg/runtime.Version=$(VERSION)' \
	-X 'tg-dlp/pkg/runtime.Commit=$(COMMIT)' \
	-X 'tg-dlp/pkg/runtime.BuildTime=$(DATE)'

GO       ?= go
GOFLAGS  ?=
BIN_DIR  := bin
DIST_DIR := dist

DOCKER_IMAGE ?= tg-dlp
DOCKER_TAG   ?= latest
PLATFORMS    ?= linux/amd64,linux/arm64

.PHONY: all build test test-race lint lint-fix vet fmt clean install tidy \
        build-all docker docker-multiarch help

all: fmt vet test build

## build: Compile the binary into bin/
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) .

## install: Install the binary into $GOPATH/bin
install:
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" .

## build-all: Build for all platforms (windows/linux/darwin x amd64/arm64/386/arm)
build-all:
	$(GO) run tools/build.go

## test: Run all tests
test:
	$(GO) test -count=1 ./...

## test-race: Run tests with the race detector
test-race:
	$(GO) test -count=1 -race ./...

## lint: Run golangci-lint
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found. Install: https://golangci-lint.run/welcome/install/"; \
		exit 1; \
	}
	golangci-lint run ./...

## lint-fix: Run golangci-lint with auto-fix
lint-fix:
	golangci-lint run --fix ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## fmt: Format all Go files
fmt:
	$(GO) fmt ./...

## tidy: Run go mod tidy
tidy:
	$(GO) mod tidy

## docker: Build Docker image (linux/amd64)
docker:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## docker-multiarch: Build multi-arch Docker image (linux/amd64 + linux/arm64)
docker-multiarch:
	docker buildx create --use --name multiarch 2>/dev/null || true
	docker buildx build \
		--platform $(PLATFORMS) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--load .

## clean: Remove build artifacts
clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

## help: Show this help
help:
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | sort