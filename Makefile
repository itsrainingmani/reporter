VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"
GOCACHE := $(shell pwd)/.cache/go-build

.PHONY: all build clean test install release-local

all: build

build:
	GOCACHE=$(GOCACHE) go build $(LDFLAGS) -o reporter ./cmd/reporter

test:
	GOCACHE=$(GOCACHE) go test -v ./cmd/reporter

clean:
	rm -f reporter
	rm -rf dist/

install: build
	mkdir -p $(HOME)/.local/bin $(HOME)/.local/share/reporter
	cp reporter $(HOME)/.local/bin/
	cp shell/reporter-auto.sh $(HOME)/.local/share/reporter/
	@echo ""
	@echo "Installed reporter to ~/.local/bin/reporter"
	@echo "Add this to your shell rc file:"
	@echo ""
	@echo '  source $$HOME/.local/share/reporter/reporter-auto.sh'
	@echo ""

# Build for all platforms (for local testing before goreleaser)
release-local:
	mkdir -p dist
	GOCACHE=$(GOCACHE) GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/reporter-darwin-arm64 ./cmd/reporter
	GOCACHE=$(GOCACHE) GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/reporter-darwin-amd64 ./cmd/reporter
	GOCACHE=$(GOCACHE) GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/reporter-linux-amd64 ./cmd/reporter
	GOCACHE=$(GOCACHE) GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/reporter-linux-arm64 ./cmd/reporter

