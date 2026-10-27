# Makefile
TOOLS := $(CURDIR)/.tools

.PHONY: install api docs build test check tools lint vuln openapi generate generate-onyx schema-validate schema-diff schema-validate-remote schema-publish live-smoke
install:
	go mod download
api:
	go run ./cmd/api
docs:
	go run ./cmd/docs
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o bin/api ./cmd/api
test:
	go test -race ./...
check: test
	go vet ./...
	$(MAKE) build

# Optional development tools; none are needed to start the API or build its image.
tools: $(TOOLS)/onyx $(TOOLS)/golangci-lint $(TOOLS)/govulncheck
$(TOOLS)/onyx:
	mkdir -p $(TOOLS)
	GOBIN=$(TOOLS) go install github.com/OnyxDevTools/onyx-cli/cmd/onyx@v0.11.14
$(TOOLS)/golangci-lint:
	mkdir -p $(TOOLS)
	GOBIN=$(TOOLS) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
$(TOOLS)/govulncheck:
	mkdir -p $(TOOLS)
	GOBIN=$(TOOLS) go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
lint: $(TOOLS)/golangci-lint
	$(TOOLS)/golangci-lint run
vuln: $(TOOLS)/govulncheck
	$(TOOLS)/govulncheck ./...
openapi:
	go run ./cmd/openapi
generate-onyx: $(TOOLS)/onyx
	go run ./cmd/schema generate
generate: generate-onyx openapi
schema-validate:
	go run ./cmd/schema validate
schema-diff: $(TOOLS)/onyx
	go run ./cmd/schema diff
schema-validate-remote: $(TOOLS)/onyx
	go run ./cmd/schema validate-remote
schema-publish: $(TOOLS)/onyx
	go run ./cmd/schema publish
live-smoke:
	@test "$$LIVE_SMOKE_CONFIRM" = development || (echo 'Set LIVE_SMOKE_CONFIRM=development and ONYX_DATABASE_ID first'; exit 1)
	go test ./internal/httpapi -run '^TestLiveCRUD$$' -count=1 -v
