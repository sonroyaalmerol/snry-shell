BINARY    := snry-shell
CMD       := ./cmd/snry-shell
GOFLAGS   :=
PKGS      := ./...

.PHONY: all build run test vet clean install help

all: build

## build: Compile the binary
build:
	go build $(GOFLAGS) -o $(BINARY) $(CMD)

## run: Build and run (development)
run: build
	./$(BINARY)

## test: Run all tests
test:
	go test $(PKGS)

## test-race: Run tests with race detector
test-race:
	go test -race $(PKGS)

## vet: Run go vet
vet:
	go vet $(PKGS)

## fmt: Format code
fmt:
	gofmt -w .
	goimports -w . 2>/dev/null || true

## clean: Remove build artifacts
clean:
	rm -f $(BINARY)

## install: Install to $GOPATH/bin
install:
	go install $(GOFLAGS) $(CMD)

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'

# System dependencies (informational, not a target)
#   Arch:  pacman -S gtk4 gtk4-layer-shell pkg-config swww matugen wireplumber
#   Fedora: dnf install gtk4-devel gtk4-layer-shell-devel pkg-config swww matugen wireplumber
#
# Fonts required:
#   Google Sans Flex, Material Symbols Rounded, JetBrains Mono NF
#
# Build requirements:
#   Go 1.26+, pkg-config, gtk4-layer-shell development headers
