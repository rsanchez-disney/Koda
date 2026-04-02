APP      := koda
MODULE   := github.disney.com/SANCR225/koda
PUB_REPO := rsanchez-disney/Koda
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
RELEASE_KEY ?= $(STEER_RELEASE_KEY)
LDFLAGS  := -s -w -X main.version=$(VERSION) -X github.disney.com/SANCR225/koda/internal/ops.releaseKey=$(RELEASE_KEY)
BIN      := ./bin/$(APP)

.PHONY: build run clean test lint fmt vet tidy install cross release release help

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
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-linux-arm64   ./cmd/koda/
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-windows-amd64.exe ./cmd/koda/


release: ## Tag + build + release to github.com (make release TAG=v0.1.0)
	@test -n "$(TAG)" || { echo "Usage: make release TAG=v0.1.0"; exit 1; }
	@which gh > /dev/null 2>&1 || { echo "Install GitHub CLI: brew install gh"; exit 1; }
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	$(MAKE) cross VERSION=$(TAG)
	GH_HOST=github.com gh release create $(TAG) bin/$(APP)-* --latest \
		--repo $(PUB_REPO) \
		--title "Koda $(TAG)" \
		--generate-notes
	@echo "\n✅ Published $(TAG) to github.com/$(PUB_REPO)"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help

smoke: cross ## Run smoke tests in Docker
	docker build -t koda-test -f test/Dockerfile .
	docker run --rm koda-test

publish: ## Tag + build + upload to GitHub releases (make publish TAG=v0.1.0)
	@test -n "$(TAG)" || { echo "Usage: make publish TAG=v0.1.0"; exit 1; }
	@which gh > /dev/null 2>&1 || { echo "Install GitHub CLI: brew install gh"; exit 1; }
	$(MAKE) release TAG=$(TAG)
	@echo "\n✅ Published $(TAG) to GitHub releases"

smoke-install: ## Test install script in Docker (downloads from GitHub releases)
	docker run --rm ubuntu:22.04 bash -c "\
		apt-get update -qq && apt-get install -y -qq curl git > /dev/null 2>&1 && \
		echo '🐾 Testing install script...' && \
		curl -fsSL https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.sh | bash && \
		echo '' && \
		export PATH=\$$HOME/.local/bin:\$$PATH && \
		koda version && \
		koda --help | head -5 && \
		echo '' && echo '✅ Install test passed'"

pack-steer: ## Create steer-runtime tarball for release (requires STEER_ROOT and optionally STEER_RELEASE_KEY)
	@test -n "$(STEER_ROOT)" || { echo "Usage: make pack-steer STEER_ROOT=../steer-runtime"; exit 1; }
	@echo "📦 Packing steer-runtime from $(STEER_ROOT)..."
	tar czf bin/steer-runtime.tar.gz -C "$(STEER_ROOT)" \
		--exclude='.git' --exclude='node_modules' --exclude='.DS_Store' --exclude='tests/runs' .
	@ls -lh bin/steer-runtime.tar.gz
	@if [ -n "$(STEER_RELEASE_KEY)" ]; then \
		echo "🔒 Encrypting..."; \
		openssl enc -aes-256-cbc -pbkdf2 -salt -in bin/steer-runtime.tar.gz -out bin/steer-runtime.tar.gz.enc -pass pass:$(STEER_RELEASE_KEY); \
		ls -lh bin/steer-runtime.tar.gz.enc; \
		echo "✅ Encrypted tarball ready"; \
	else \
		echo "⚠ No STEER_RELEASE_KEY — tarball is unencrypted"; \
	fi

publish-steer: pack-steer ## Upload steer-runtime tarball to public repo (make publish-steer TAG=v0.1.4 STEER_ROOT=../steer-runtime)
	@test -n "$(TAG)" || { echo "Usage: make publish-steer TAG=v0.1.4 STEER_ROOT=../steer-runtime"; exit 1; }
	GH_HOST=github.com gh release create $(TAG) --repo rsanchez-disney/steer-runtime --title "$(TAG)" --notes "steer-runtime $(TAG)" 2>/dev/null || true
	@if [ -f bin/steer-runtime.tar.gz.enc ]; then \
		GH_HOST=github.com gh release upload $(TAG) bin/steer-runtime.tar.gz.enc --repo rsanchez-disney/steer-runtime --clobber; \
	else \
		GH_HOST=github.com gh release upload $(TAG) bin/steer-runtime.tar.gz --repo rsanchez-disney/steer-runtime --clobber; \
	fi
	@echo "✅ Uploaded steer-runtime tarball to rsanchez-disney/steer-runtime $(TAG)"
