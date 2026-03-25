APP      := koda
MODULE   := github.disney.com/SANCR225/koda
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
BIN      := ./bin/$(APP)

.PHONY: build run clean test lint fmt vet tidy install cross release help

build: ## Build binary
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/koda/

run: build ## Build and launch TUI
	$(BIN) --steer-root ../steer-runtime

install: build ## Copy binary to ~/go/bin
	cp $(BIN) $(GOPATH)/bin/$(APP) 2>/dev/null || cp $(BIN) ~/go/bin/$(APP)

test: ## Run tests
	go test ./... -v

lint: vet ## Run linters
	@which golangci-lint > /dev/null 2>&1 || { echo "Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run ./...

fmt: ## Format code
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy modules
	go mod tidy

clean: ## Remove build artifacts
	rm -rf bin/

cross: ## Cross-compile for macOS, Linux, Windows
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-darwin-arm64  ./cmd/koda/
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-darwin-amd64  ./cmd/koda/
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-linux-amd64   ./cmd/koda/
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-windows-amd64.exe ./cmd/koda/

release: ## Tag release and cross-compile
	@test -n "$(TAG)" || { echo "Usage: make release TAG=v0.1.0"; exit 1; }
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	$(MAKE) cross VERSION=$(TAG)
	@echo "\n✅ Release $(TAG) built in bin/"
	@ls -lh bin/$(APP)-*

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
