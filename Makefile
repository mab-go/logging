# logging — Run 'make' or 'make help' to see available commands

.DEFAULT_GOAL := help

BIN         := ./bin
GOLANGCI    := $(BIN)/golangci-lint
GOIMPORTS   := $(BIN)/goimports
GOCYCLO     := $(BIN)/gocyclo
# Pinned golangci-lint release for reproducible `make lint`; bump if unsupported on current Go (see go.mod).
# v2.6.x binaries were built with Go 1.25 and reject go.mod go 1.26+; use v2.9+ for Go 1.26 toolchains.
GOLANGCI_LINT_VERSION ?= v2.11.3
# Pinned goimports (golang.org/x/tools); bump if `make fmt` fails or is incompatible with go.mod Go version.
GOIMPORTS_VERSION ?= v0.38.0
# Pinned gocyclo (github.com/fzipp/gocyclo); bump for `make cyclo` reproducibility.
GOCYCLO_VERSION ?= v0.6.0

# golangci-lint must be built with Go >= go.mod; auto follows deps' older go version, so pin to module Go.
# Prefer explicit `toolchain` directive (e.g. go1.26.1); `go 1.26` alone is not a valid GOTOOLCHAIN value.
GO_MOD_VERSION := $(shell grep -E '^go ' go.mod | head -1 | awk '{print $$2}')
TOOLCHAIN_FROM_MOD := $(shell grep -E '^toolchain ' go.mod | head -1 | awk '{print $$2}')
TOOLCHAIN_FOR_TOOLS ?= $(if $(TOOLCHAIN_FROM_MOD),$(TOOLCHAIN_FROM_MOD),go$(GO_MOD_VERSION))

RACE ?= 1
OPEN ?= $(shell command -v xdg-open 2>/dev/null || echo "open")

.PHONY: help \
        setup \
        test test\:cover \
        lint lint\:fix fmt vet cyclo \
        mod\:tidy mod\:verify \
        clean clean\:cache clean\:all \
        versions

#------------------------------------------------------------------------------
# Help
#------------------------------------------------------------------------------

help: ## Show available commands
	@awk '\
		/^#-+$$/ { next } \
		/^# [A-Za-z]/ { section = substr($$0, 3); next } \
		/^[a-zA-Z_:\\-]+:.*## / { \
			gsub(/\\:/, ":", $$0); \
			match($$0, /## /); \
			desc = substr($$0, RSTART + 3); \
			prefix = substr($$0, 1, RSTART - 1); \
			gsub(/: [^:]*$$/, "", prefix); \
			target = prefix; \
			targets[section] = targets[section] sprintf("  \033[36m%-22s\033[0m %s\n", target, desc); \
			order[section] = order[section] ? order[section] : ++count; \
		} \
		END { \
			for (i = 1; i <= count; i++) { \
				for (s in order) { \
					if (order[s] == i) { \
						if (i > 1) printf "\n"; \
						printf "\033[1m%s\033[0m\n", s; \
						printf "%s", targets[s]; \
					} \
				} \
			} \
		}' $(MAKEFILE_LIST)

#------------------------------------------------------------------------------
# Setup
#------------------------------------------------------------------------------

setup: ## Install required Go tools into ./bin (project-local)
	@mkdir -p $(BIN)
	GOTOOLCHAIN=$(TOOLCHAIN_FOR_TOOLS) GOBIN=$(abspath $(BIN)) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOTOOLCHAIN=$(TOOLCHAIN_FOR_TOOLS) GOBIN=$(abspath $(BIN)) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	GOTOOLCHAIN=$(TOOLCHAIN_FOR_TOOLS) GOBIN=$(abspath $(BIN)) go install github.com/fzipp/gocyclo/cmd/gocyclo@$(GOCYCLO_VERSION)
	@echo ""
	@echo "Setup complete: $(GOLANGCI), $(GOIMPORTS), $(GOCYCLO)"
	@echo ""

#------------------------------------------------------------------------------
# Test
#------------------------------------------------------------------------------

test: ## Run all tests (RACE=1 default; RACE=0 to disable -race)
	go test $(if $(filter 1,$(RACE)),-race,) ./...

test\:cover: ## Coverage report; opens HTML unless CI is set (override OPEN=...)
	go test $(if $(filter 1,$(RACE)),-race,) -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@if [ -z "$$CI" ]; then $(OPEN) coverage.html; else echo "Wrote coverage.html (CI set, skipping browser)"; fi

#------------------------------------------------------------------------------
# Lint and Format
#------------------------------------------------------------------------------

lint: ## Run golangci-lint
	@test -x $(GOLANGCI) || (echo "Run 'make setup' to install golangci-lint" && exit 1)
	$(GOLANGCI) run ./...

lint\:fix: ## Run golangci-lint with --fix
	@test -x $(GOLANGCI) || (echo "Run 'make setup' to install golangci-lint" && exit 1)
	$(GOLANGCI) run --fix ./...

fmt: ## Format with goimports (gofmt + import fixes; -l lists changed files, -w writes)
	@test -x $(GOIMPORTS) || (echo "Run 'make setup' to install goimports into ./bin" && exit 1)
	$(GOIMPORTS) -l -w .

vet: ## Run go vet
	go vet ./...

cyclo: ## Run gocyclo; run 'make setup' first
	@test -x $(GOCYCLO) || (echo "Run 'make setup' to install gocyclo" && exit 1)
	$(GOCYCLO) -over 10 .

#------------------------------------------------------------------------------
# Module
#------------------------------------------------------------------------------

mod\:tidy: ## Run go mod tidy
	go mod tidy

mod\:verify: ## Run go mod verify
	go mod verify

#------------------------------------------------------------------------------
# Clean
#------------------------------------------------------------------------------

clean: ## Remove coverage artifacts
	rm -f coverage.out coverage.html

clean\:cache: ## Clear Go test cache
	go clean -testcache

clean\:all: clean ## Run clean plus remove ./bin (Go tools)
	rm -rf $(BIN)

#------------------------------------------------------------------------------
# Utilities
#------------------------------------------------------------------------------

versions: ## Show Go and required tool versions
	@echo "Go: $$(go version)"
	@if test -x $(GOLANGCI); then $(GOLANGCI) version; else echo "golangci-lint: not installed (run make setup)"; fi
	@if test -x $(GOIMPORTS); then echo "goimports (module metadata):"; go version -m $(GOIMPORTS) 2>&1 | head -4; else echo "goimports: not installed (run make setup)"; fi
	@if test -x $(GOCYCLO); then echo "gocyclo (module metadata):"; go version -m $(GOCYCLO) 2>&1 | head -4; else echo "gocyclo: not installed (run make setup)"; fi
