SHELL := /bin/bash

# Project
MODULE   := github.com/emergingrobotics/gopicar
BINARY   := picarctl
CMD      := ./examples/picarctl
BIN_DIR  := bin
COVERAGE := coverage.out

# Toolchain
GO       := go
GOFLAGS  :=

# Cross-compilation for the Raspberry Pi on the Picar-X
# Pi 4/5 (64-bit OS): arm64  |  Pi Zero/3 (32-bit OS): arm/GOARM=7
# Deploy user defaults to whoever runs `make deploy` (override with PI_USER=...)
PI_USER  ?= $(shell id -un)
PI_HOST  ?= rpi1
PI_DIR   ?= /home/$(PI_USER)
PI_TARGET := $(PI_USER)@$(PI_HOST)

.PHONY: help build build-arm64 build-arm run test test-race test-hw cover cover-html \
        fmt vet lint tidy deploy clean

# --------------------------------------------------------------------------
# Help
# --------------------------------------------------------------------------
help:
	@echo "gopicar - Go driver for the Picar-X robot"
	@echo ""
	@echo "Build:"
	@echo "  make build       Build $(BINARY) for the host into $(BIN_DIR)/"
	@echo "  make build-arm64 Cross-compile for Raspberry Pi 64-bit OS (arm64)"
	@echo "  make build-arm   Cross-compile for Raspberry Pi 32-bit OS (armv7)"
	@echo "  make run         go run $(CMD) (add ARGS=...)"
	@echo ""
	@echo "Quality:"
	@echo "  make test        Run all tests (hardware-free)"
	@echo "  make test-race   Run all tests with the race detector"
	@echo "  make test-hw     Run hardware integration tests (on the Pi, -tags hardware)"
	@echo "  make cover       Run tests and report coverage"
	@echo "  make cover-html  Generate and open an HTML coverage report"
	@echo "  make fmt         gofmt all packages"
	@echo "  make vet         go vet all packages"
	@echo "  make lint        Run golangci-lint (if installed)"
	@echo "  make tidy        go mod tidy"
	@echo ""
	@echo "Deploy:"
	@echo "  make deploy      Build arm64 and scp $(BINARY) to $(PI_TARGET):$(PI_DIR)"
	@echo "                   (user defaults to local \$$(id -un); override:"
	@echo "                    make deploy PI_USER=pi PI_HOST=1.2.3.4 PI_DIR=/tmp)"
	@echo ""
	@echo "  make clean       Remove build and coverage artifacts"

# --------------------------------------------------------------------------
# Build
# --------------------------------------------------------------------------
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD)

build-arm64:
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY)-arm64 $(CMD)

build-arm:
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=arm GOARM=7 $(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY)-armv7 $(CMD)

run:
	$(GO) run $(CMD)

# --------------------------------------------------------------------------
# Quality
# --------------------------------------------------------------------------
test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

# Hardware integration tests (run ON the Pi, with the HAT attached).
# Movement tests additionally require GOPICAR_HW_MOVE=1.
test-hw:
	$(GO) test -tags hardware ./...

cover:
	$(GO) test -coverprofile=$(COVERAGE) ./...
	$(GO) tool cover -func=$(COVERAGE)

cover-html: cover
	$(GO) tool cover -html=$(COVERAGE)

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
	    golangci-lint run; \
	else \
	    echo "golangci-lint not installed; skipping."; \
	    echo "Install: https://golangci-lint.run/usage/install/"; \
	fi

tidy:
	$(GO) mod tidy

# --------------------------------------------------------------------------
# Deploy
# --------------------------------------------------------------------------
deploy: build-arm64
	scp $(BIN_DIR)/$(BINARY)-arm64 $(PI_TARGET):$(PI_DIR)/$(BINARY)
	@echo "Copied $(BINARY) to $(PI_TARGET):$(PI_DIR)/$(BINARY)"

# --------------------------------------------------------------------------
# Clean
# --------------------------------------------------------------------------
clean:
	rm -rf $(BIN_DIR) $(COVERAGE) coverage.html
	@echo "Cleaned build and coverage artifacts."
