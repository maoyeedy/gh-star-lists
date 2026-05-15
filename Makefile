SHELL := bash
GOEXE := $(shell go env GOEXE)
.PHONY: fmt test vet build lint ascii-check smoke check

fmt:
	goimports -w .

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p ./bin
	go build -o ./bin/gh-star-lists$(GOEXE) .

lint:
	golangci-lint run

ascii-check:
	@if LC_ALL=C grep -Pn '[^\x00-\x7F]' --include='*.go' -r . 2>/dev/null; then \
		echo "ERROR: non-ASCII characters found in Go source"; exit 1; \
	else \
		echo "ascii-check: clean"; \
	fi

smoke:
	bash scripts/smoke-local.sh

check:
	bash scripts/check.sh
