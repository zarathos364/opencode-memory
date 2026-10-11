BINDIR  := $(CURDIR)/bin
BINNAME ?= opencode-memory
VERSION ?= 0.0.1

# GO OPTIONS
LDFLAGS     := -w -s -X main.version=$(VERSION)
GOFLAGS     ?=
CGO_ENABLED ?= 0

SRC := $(shell find . -type f -name '*.go' -print) go.mod go.sum

# GO ---------------------------------------------------------------------------

.PHONY: build clean install run fmt lint tidy test vet vuln-check

build: tidy $(BINDIR)/$(BINNAME)
$(BINDIR)/$(BINNAME): $(SRC)
	CGO_ENABLED=$(CGO_ENABLED) go build -trimpath -ldflags '$(LDFLAGS)' -o '$(BINDIR)/$(BINNAME)' ./cmd/opencode-memory

install:
	go install $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cmd/opencode-memory

clean:
	rm -f '$(BINDIR)/$(BINNAME)'

run:
	go run -ldflags '$(LDFLAGS)' ./cmd/opencode-memory $(ARGS)

tidy:
	go mod tidy

fmt:
	golangci-lint fmt

lint:
	golangci-lint run

vet:
	go vet ./...

test:
	go test -race ./...

vuln-check:
	govulncheck ./...

# DEV TOOLS --------------------------------------------------------------------

-include .github/env

.PHONY: check-ci-env install-tools

check-ci-env:
	@test -n "$(GOLANGCI_LINT_VERSION)" || (echo "GOLANGCI_LINT_VERSION not set (.github/env missing?)"; exit 1)
	@test -n "$(GOVULNCHECK_VERSION)" || (echo "GOVULNCHECK_VERSION not set (.github/env missing?)"; exit 1)

install-tools: check-ci-env
	@echo "==> Installing golangci-lint@v$(GOLANGCI_LINT_VERSION)..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION)

	@echo "==> Installing govulncheck@v$(GOVULNCHECK_VERSION)..."
	go install golang.org/x/vuln/cmd/govulncheck@v$(GOVULNCHECK_VERSION)

.NOTPARALLEL:
