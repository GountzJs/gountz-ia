BINARY_NAME=gz-ia
BUILD_DIR=bin
VERSION?=0.3.0
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X gz-ia/internal/version.Version=$(VERSION) -X gz-ia/internal/version.Commit=$(COMMIT) -X gz-ia/internal/version.Date=$(DATE)"

.PHONY: all build run test clean docs-dev docs-build

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/gz-ia

run:
	go run ./cmd/gz-ia start

test:
	go test -v ./...

docs-dev:
	cd docs && npm run docs:dev

docs-build:
	cd docs && npm run docs:build

clean:
	rm -rf $(BUILD_DIR)

