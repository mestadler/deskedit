BINARY := deskedit
PREFIX ?= $(HOME)/.local
INSTALL_DIR := $(PREFIX)/bin

# Pull version from git if available, else dev.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all build install uninstall clean test tidy run lint check

all: build

build:
	go build $(LDFLAGS) -o $(BINARY) .

install: build
	install -D -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)"
	@echo "Ensure $(INSTALL_DIR) is on your PATH."

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./...

tidy:
	go mod tidy

run: build
	./$(BINARY)

lint:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then printf '%s\n' "$$out"; exit 1; fi
	go vet ./...

check: lint test
