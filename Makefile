APP      := koda
MODULE   := github.disney.com/SANCR225/koda
PUB_REPO := rsanchez-disney/Koda
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
RELEASE_KEY ?= $(STEER_RELEASE_KEY)
LDFLAGS  := -s -w -X main.version=$(VERSION) -X github.disney.com/SANCR225/koda/internal/ops.releaseKey=$(RELEASE_KEY)
BIN      := ./bin/$(APP)
YAX_REPO := github.disney.com-sancr225:QUINJ327/yax.git
YAX_SRC  ?= /tmp/yax

.PHONY: build run clean test lint fmt vet tidy install cross release help yax-fetch yax-cross

build: ## Build binary
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/koda/

run: build ## Build and launch TUI
	$(BIN) --steer-root ../steer-runtime

install: build ## Copy binary to ~/.local/bin
	mkdir -p ~/.local/bin
	cp $(BIN) ~/.local/bin/$(APP)
	-codesign -s - -f ~/.local/bin/$(APP) 2>/dev/null
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

cross: ## Cross-compile Koda for macOS, Linux, Windows
	CGO_ENABLED=1 GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-darwin-arm64  ./cmd/koda/
	CGO_ENABLED=1 GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-darwin-amd64  ./cmd/koda/
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-linux-amd64   ./cmd/koda/
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-linux-arm64   ./cmd/koda/
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-windows-amd64.exe ./cmd/koda/

yax-fetch: ## Clone or pull latest yax source
	@if [ -d "$(YAX_SRC)/.git" ]; then \
		echo "  Pulling yax..."; \
		cd $(YAX_SRC) && git pull --ff-only 2>/dev/null || \
		(echo "  Pull failed, re-cloning..."; rm -rf $(YAX_SRC); git clone --depth 1 git@$(YAX_REPO) $(YAX_SRC)); \
	else \
		echo "  Cloning yax..."; \
		rm -rf $(YAX_SRC); \
		git clone --depth 1 git@$(YAX_REPO) $(YAX_SRC); \
	fi

yax-cross: yax-fetch ## Fetch, test, and cross-compile yax
	@echo "  Testing yax..."
	cd $(YAX_SRC) && go test ./...
	@echo "  Building yax..."
	cd $(YAX_SRC) && CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags "-s -w" -o $(CURDIR)/bin/yax-darwin-arm64  ./cmd/yax/
	cd $(YAX_SRC) && CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags "-s -w" -o $(CURDIR)/bin/yax-darwin-amd64  ./cmd/yax/
	cd $(YAX_SRC) && CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w" -o $(CURDIR)/bin/yax-linux-amd64   ./cmd/yax/
	cd $(YAX_SRC) && CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags "-s -w" -o $(CURDIR)/bin/yax-linux-arm64   ./cmd/yax/
	cd $(YAX_SRC) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(CURDIR)/bin/yax-windows-amd64.exe ./cmd/yax/

release: ## Tag + build Koda + yax + publish (make release TAG=v0.1.0)
	@test -n "$(TAG)" || { echo "Usage: make release TAG=v0.1.0"; exit 1; }
	@which gh > /dev/null 2>&1 || { echo "Install GitHub CLI: brew install gh"; exit 1; }
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	$(MAKE) cross VERSION=$(TAG)
	-$(MAKE) yax-cross
	GH_HOST=github.com gh release create $(TAG) bin/$(APP)-* $$(ls bin/yax-* 2>/dev/null) --latest \
		--repo $(PUB_REPO) \
		--title "Koda $(TAG)" \
		--generate-notes
	@echo "Published $(TAG) to github.com/$(PUB_REPO)"

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

smoke-install: ## Test install script in Docker (downloads from GitHub releases)
	docker run --rm ubuntu:22.04 bash -c "\
		apt-get update -qq && apt-get install -y -qq curl git > /dev/null 2>&1 && \
		echo 'Testing install script...' && \
		curl -fsSL https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.sh | bash && \
		echo '' && \
		export PATH=\$$HOME/.local/bin:\$$PATH && \
		koda version && \
		koda --help | head -5 && \
		echo '' && echo 'Install test passed'"

pack-steer: ## Create steer-runtime tarball for release (requires STEER_ROOT and optionally STEER_RELEASE_KEY)
	@test -n "$(STEER_ROOT)" || { echo "Usage: make pack-steer STEER_ROOT=../steer-runtime"; exit 1; }
	@echo "Packing steer-runtime from $(STEER_ROOT)..."
	tar czf bin/steer-runtime.tar.gz -C "$(STEER_ROOT)" \
		--exclude='.git' --exclude='node_modules' --exclude='.DS_Store' --exclude='tests/runs' \
		--exclude='shared/tools/mcp-servers/*/src' \
		--exclude='shared/tools/mcp-servers/*/package.json' \
		--exclude='shared/tools/mcp-servers/*/package-lock.json' \
		--exclude='shared/tools/mcp-servers/*/tsconfig.json' .
	@ls -lh bin/steer-runtime.tar.gz
	@if [ -n "$(STEER_RELEASE_KEY)" ]; then \
		echo "Encrypting..."; \
		openssl enc -aes-256-cbc -pbkdf2 -salt -in bin/steer-runtime.tar.gz -out bin/steer-runtime.tar.gz.enc -pass pass:$(STEER_RELEASE_KEY); \
		ls -lh bin/steer-runtime.tar.gz.enc; \
		echo "Encrypted tarball ready"; \
	else \
		echo "No STEER_RELEASE_KEY — tarball is unencrypted"; \
	fi

publish-steer: pack-steer ## Upload steer-runtime tarball to public repo (make publish-steer TAG=v0.1.4 STEER_ROOT=../steer-runtime)
	@test -n "$(TAG)" || { echo "Usage: make publish-steer TAG=v0.1.4 STEER_ROOT=../steer-runtime"; exit 1; }
	@echo "$(TAG)" > "$(STEER_ROOT)/VERSION"
	$(MAKE) pack-steer STEER_ROOT=$(STEER_ROOT)
	GH_HOST=github.com gh release create $(TAG) --repo rsanchez-disney/steer-runtime --title "$(TAG)" --notes "steer-runtime $(TAG)" 2>/dev/null || true
	@if [ -f bin/steer-runtime.tar.gz.enc ]; then \
		GH_HOST=github.com gh release upload $(TAG) bin/steer-runtime.tar.gz.enc --repo rsanchez-disney/steer-runtime --clobber; \
	else \
		GH_HOST=github.com gh release upload $(TAG) bin/steer-runtime.tar.gz --repo rsanchez-disney/steer-runtime --clobber; \
	fi
	@echo "Uploaded steer-runtime tarball to rsanchez-disney/steer-runtime $(TAG)"

STEER_ROOT     ?= ../steer-runtime
AUTOPILOT_ROOT ?= ../steer-autopilot
KITESTREAM_ROOT ?= ../KiteStream

publish-all: ## Pull, detect changes, auto-version, publish all repos with changes
	@echo "=== Pulling latest ==="
	@git pull --ff-only 2>/dev/null || true
	@git -C $(STEER_ROOT) checkout main 2>/dev/null && git -C $(STEER_ROOT) pull --ff-only 2>/dev/null || true
	@echo ""
	@# --- Koda ---
	@KODA_LAST=$$(git tag --sort=-v:refname | head -1); \
	KODA_COMMITS=$$(git log $$KODA_LAST..HEAD --oneline 2>/dev/null | wc -l | tr -d ' '); \
	if [ "$$KODA_COMMITS" -gt 0 ]; then \
		MAJOR=$$(echo $$KODA_LAST | sed 's/v//' | cut -d. -f1); \
		MINOR=$$(echo $$KODA_LAST | sed 's/v//' | cut -d. -f2); \
		PATCH=$$(echo $$KODA_LAST | sed 's/v//' | cut -d. -f3); \
		NEXT="v$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
		echo "Koda: $$KODA_COMMITS commits since $$KODA_LAST → $$NEXT"; \
		$(MAKE) release TAG=$$NEXT; \
		echo "  Cleaning old Koda releases (keeping last 3)..."; \
		sleep 3; \
		GH_HOST=github.com gh release list --repo $(PUB_REPO) --limit 50 --json tagName --jq '.[].tagName' 2>/dev/null | \
			sort -t. -k1,1rn -k2,2rn -k3,3rn | tail -n +4 | \
			while read old; do echo "    removing $$old"; GH_HOST=github.com gh release delete "$$old" --repo $(PUB_REPO) --yes --cleanup-tag 2>/dev/null || true; done; \
	else \
		echo "Koda: up to date ($$KODA_LAST)"; \
	fi
	@echo ""
	@# --- steer-runtime ---
	@STEER_LAST=$$(GH_HOST=github.com gh release list --repo rsanchez-disney/steer-runtime --limit 1 --json tagName --jq '.[0].tagName' 2>/dev/null); \
	git -C $(STEER_ROOT) fetch --tags 2>/dev/null; \
	if git -C $(STEER_ROOT) rev-parse "$$STEER_LAST" >/dev/null 2>&1; then \
		STEER_COMMITS=$$(git -C $(STEER_ROOT) log $$STEER_LAST..HEAD --oneline 2>/dev/null | wc -l | tr -d ' '); \
	else \
		STEER_COMMITS=999; \
	fi; \
	if [ "$$STEER_COMMITS" -gt 0 ]; then \
		MAJOR=$$(echo $$STEER_LAST | sed 's/v//' | cut -d. -f1); \
		MINOR=$$(echo $$STEER_LAST | sed 's/v//' | cut -d. -f2); \
		PATCH=$$(echo $$STEER_LAST | sed 's/v//' | cut -d. -f3); \
		NEXT="v$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
		echo "steer-runtime: $$STEER_COMMITS commits since $$STEER_LAST → $$NEXT"; \
		git -C $(STEER_ROOT) log $$STEER_LAST..HEAD --oneline 2>/dev/null | head -5; \
		echo "  Rebuilding MCP bundles..."; \
		$(MAKE) -C $(STEER_ROOT) mcp-build 2>&1 | grep -E "✅|⚠"; \
		git -C $(STEER_ROOT) add -A 2>/dev/null; \
		git -C $(STEER_ROOT) diff --cached --quiet || git -C $(STEER_ROOT) commit -m "chore: rebuild MCP bundles" 2>/dev/null; \
		git -C $(STEER_ROOT) tag -a $$NEXT -m "Release $$NEXT" 2>/dev/null; \
		git -C $(STEER_ROOT) push origin $$NEXT 2>/dev/null; \
		$(MAKE) publish-steer TAG=$$NEXT STEER_ROOT=$(STEER_ROOT); \
		echo "  Cleaning old steer-runtime releases (keeping last 3)..."; \
		sleep 3; \
		GH_HOST=github.com gh release list --repo rsanchez-disney/steer-runtime --limit 50 --json tagName --jq '.[].tagName' 2>/dev/null | \
			sort -t. -k1,1rn -k2,2rn -k3,3rn | tail -n +4 | \
			while read old; do echo "    removing $$old"; GH_HOST=github.com gh release delete "$$old" --repo rsanchez-disney/steer-runtime --yes --cleanup-tag 2>/dev/null || true; done; \
	else \
		echo "steer-runtime: up to date ($$STEER_LAST)"; \
	fi
	@echo ""
	@# --- steer-autopilot ---
	@if [ -d "$(AUTOPILOT_ROOT)" ]; then \
		AP_LAST=$$(git -C $(AUTOPILOT_ROOT) tag --sort=-v:refname | head -1); \
		if [ -n "$$AP_LAST" ]; then \
			AP_COMMITS=$$(git -C $(AUTOPILOT_ROOT) log $$AP_LAST..HEAD --oneline 2>/dev/null | wc -l | tr -d ' '); \
		else \
			AP_COMMITS=999; \
		fi; \
		if [ "$$AP_COMMITS" -gt 0 ]; then \
			MAJOR=$$(echo $$AP_LAST | sed 's/v//' | cut -d. -f1); \
			MINOR=$$(echo $$AP_LAST | sed 's/v//' | cut -d. -f2); \
			PATCH=$$(echo $$AP_LAST | sed 's/v//' | cut -d. -f3); \
			NEXT="v$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
			echo "steer-autopilot: $$AP_COMMITS commits since $$AP_LAST → $$NEXT"; \
			git -C $(AUTOPILOT_ROOT) log $$AP_LAST..HEAD --oneline 2>/dev/null | head -5; \
			git -C $(AUTOPILOT_ROOT) tag -a $$NEXT -m "Release $$NEXT" 2>/dev/null; \
			git -C $(AUTOPILOT_ROOT) push origin $$NEXT 2>/dev/null; \
			$(MAKE) -C $(AUTOPILOT_ROOT) release TAG=$$NEXT; \
			echo "  Cleaning old autopilot releases (keeping last 3)..."; \
			sleep 3; \
			GH_HOST=github.com gh release list --repo rsanchez-disney/steer-autopilot --limit 50 --json tagName --jq '.[].tagName' 2>/dev/null | \
				sort -t. -k1,1rn -k2,2rn -k3,3rn | tail -n +4 | \
				while read old; do echo "    removing $$old"; GH_HOST=github.com gh release delete "$$old" --repo rsanchez-disney/steer-autopilot --yes --cleanup-tag 2>/dev/null || true; done; \
		else \
			echo "steer-autopilot: up to date ($$AP_LAST)"; \
		fi; \
	else \
		echo "steer-autopilot: not found at $(AUTOPILOT_ROOT) — skipping"; \
	fi
	@echo ""
	@# --- KiteStream ---
	@if [ -d "$(KITESTREAM_ROOT)" ]; then \
		git -C $(KITESTREAM_ROOT) checkout main 2>/dev/null && git -C $(KITESTREAM_ROOT) pull --ff-only 2>/dev/null || true; \
		KS_LAST=$$(GH_HOST=github.com gh release list --repo rsanchez-disney/KiteStream --limit 1 --json tagName --jq '.[0].tagName' 2>/dev/null); \
		git -C $(KITESTREAM_ROOT) fetch --tags 2>/dev/null; \
		if [ -n "$$KS_LAST" ] && git -C $(KITESTREAM_ROOT) rev-parse "$$KS_LAST" >/dev/null 2>&1; then \
			KS_COMMITS=$$(git -C $(KITESTREAM_ROOT) log $$KS_LAST..HEAD --oneline 2>/dev/null | wc -l | tr -d ' '); \
		else \
			KS_COMMITS=999; \
		fi; \
		if [ "$$KS_COMMITS" -gt 0 ]; then \
			if [ -n "$$KS_LAST" ]; then \
				MAJOR=$$(echo $$KS_LAST | sed 's/v//' | cut -d. -f1); \
				MINOR=$$(echo $$KS_LAST | sed 's/v//' | cut -d. -f2); \
				PATCH=$$(echo $$KS_LAST | sed 's/v//' | cut -d. -f3); \
				NEXT="v$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
			else \
				NEXT="v0.1.0"; \
			fi; \
			echo "KiteStream: $$KS_COMMITS commits since $${KS_LAST:-none} → $$NEXT"; \
			git -C $(KITESTREAM_ROOT) log $${KS_LAST:+$$KS_LAST..}HEAD --oneline 2>/dev/null | head -5; \
			git -C $(KITESTREAM_ROOT) tag -a $$NEXT -m "Release $$NEXT" 2>/dev/null; \
			git -C $(KITESTREAM_ROOT) push origin $$NEXT 2>/dev/null; \
			if [ -f "$(KITESTREAM_ROOT)/Makefile" ]; then \
				$(MAKE) -C $(KITESTREAM_ROOT) release TAG=$$NEXT 2>/dev/null || true; \
			fi; \
			echo "  Cleaning old KiteStream releases (keeping last 3)..."; \
			sleep 3; \
			GH_HOST=github.com gh release list --repo rsanchez-disney/KiteStream --limit 50 --json tagName --jq '.[].tagName' 2>/dev/null | \
				sort -t. -k1,1rn -k2,2rn -k3,3rn | tail -n +4 | \
				while read old; do echo "    removing $$old"; GH_HOST=github.com gh release delete "$$old" --repo rsanchez-disney/KiteStream --yes --cleanup-tag 2>/dev/null || true; done; \
		else \
			echo "KiteStream: up to date ($$KS_LAST)"; \
		fi; \
	else \
		echo "KiteStream: not found at $(KITESTREAM_ROOT) — skipping"; \
	fi
	@echo ""
	@echo "Done."
