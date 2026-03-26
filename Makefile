APP      := koda
MODULE   := github.disney.com/SANCR225/koda
GH_REPO  := SANCR225/Koda
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
BIN      := ./bin/$(APP)

.PHONY: build run clean test lint fmt vet tidy install cross release publish help

build: ## Build binary
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/koda/

run: build ## Build and launch TUI
	$(BIN) --steer-root ../steer-runtime

install: build ## Copy binary to ~/.local/bin
	mkdir -p ~/.local/bin
	cp $(BIN) ~/.local/bin/$(APP)
	@echo "Installed to ~/.local/bin/$(APP)"

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

release: ## Tag + cross-compile (make release TAG=v0.1.0)
	@test -n "$(TAG)" || { echo "Usage: make release TAG=v0.1.0"; exit 1; }
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	$(MAKE) cross VERSION=$(TAG)
	@echo "\n✅ Release $(TAG) built in bin/"
	@ls -lh bin/$(APP)-*

publish: ## Tag + build + upload to GitHub releases (make publish TAG=v0.1.0)
	@test -n "$(TAG)" || { echo "Usage: make publish TAG=v0.1.0"; exit 1; }
	@which gh > /dev/null 2>&1 || { echo "Install GitHub CLI: brew install gh"; exit 1; }
	$(MAKE) release TAG=$(TAG)
	gh release create $(TAG) bin/$(APP)-* --repo $(GH_REPO) --title "$(TAG)" --notes "Koda $(TAG)\n\nInstall: \`curl -fsSL https://github.disney.com/raw/$(GH_REPO)/main/install.sh | bash\`"
	@echo "\n✅ Published $(TAG) to GitHub releases"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
