ROOT_DIR    = $(shell pwd)
GREEN := "\\033[0;32m"
NC := "\\033[0m"
define print
	echo $(GREEN)$1$(NC)
endef

.PHONY: test
test:
	@$(call print, "Running all unit tests...")
	@go test ./...

.PHONY: test-verbose
test-verbose:
	@$(call print, "Running all unit tests with verbose output...")
	@go test ./... -v

.PHONY: test-cover
test-cover:
	@$(call print, "Running unit tests with coverage...")
	@go test ./... -cover

.PHONY: test-cover-html
test-cover-html:
	@$(call print, "Running unit tests with coverage and generating HTML report...")
	@go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@$(call print, "Coverage report generated: coverage.html")

.PHONY: test-race
test-race:
	@$(call print, "Running unit tests with race detector...")
	@go test ./... -race

.PHONY: test-short
test-short:
	@$(call print, "Running short unit tests...")
	@go test ./... -short

.PHONY: test-skip-network
test-skip-network:
	@$(call print, "Running unit tests skipping network tests...")
	@go test ./... -short -run '^[^N]'

.PHONY: list-versions
list-versions:
	@$(call print, "Listing all available versions of github.com/steemit/steemgosdk...")
	@go list -m -versions github.com/steemit/steemgosdk
