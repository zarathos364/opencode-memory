BINDIR  := $(CURDIR)/bin
BINNAME ?= opencode-memory
VERSION ?= 0.0.4

# GO OPTIONS
LDFLAGS     := -w -s -X main.version=$(VERSION)
GOFLAGS     ?=
CGO_ENABLED ?= 0
GOCMD       ?= CGO_ENABLED=$(CGO_ENABLED) go

# PACKAGE
DIST          ?= $(CURDIR)/dist
BINARY        := $(BINDIR)/$(BINNAME)
PACKAGE_FILES ?= LICENSE README.md

SRC := $(shell find . -type f -name '*.go' -print) go.mod go.sum

# GO ---------------------------------------------------------------------------

.PHONY: build clean install run fmt lint tidy test vet vuln-check package

build: tidy $(BINDIR)/$(BINNAME)
$(BINDIR)/$(BINNAME): $(SRC)
	$(GOCMD) build -trimpath -ldflags '$(LDFLAGS)' -o '$(BINDIR)/$(BINNAME)' ./cmd/opencode-memory

install:
	$(GOCMD) install $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cmd/opencode-memory

clean:
	rm -f '$(BINDIR)/$(BINNAME)'

run:
	$(GOCMD) run -ldflags '$(LDFLAGS)' ./cmd/opencode-memory $(ARGS)

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

package: build
	@os=$$(go env GOOS); arch=$$(go env GOARCH); \
	exe=""; [ "$$os" = windows ] && exe=".exe"; \
	name=$(BINNAME)_$(VERSION)_$${os}_$${arch}; \
	rm -rf $(DIST)/$$name; \
	mkdir -p $(DIST)/$$name; \
	cp $(BINARY) $(DIST)/$$name/$(BINNAME)$$exe; \
	cp $(PACKAGE_FILES) $(DIST)/$$name/; \
	tar --sort=name --owner=0 --group=0 --numeric-owner --mtime=@0 \
		-cf - -C $(DIST) $$name | gzip -9n > $(DIST)/$$name.tar.gz; \
	rm -rf $(DIST)/$$name; \
	echo "wrote $(DIST)/$$name.tar.gz"; \
	tar -tzvf $(DIST)/$$name.tar.gz

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
