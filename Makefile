PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
BIN ?= fairway

.PHONY: build install test vet check

build:
	go build -o ./$(BIN) ./cmd/fairway

install:
	install -d "$(BINDIR)"
	go build -o "$(BINDIR)/$(BIN)" ./cmd/fairway

test:
	go test ./...

vet:
	go vet ./...

check: test vet
