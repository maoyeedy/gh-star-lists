SHELL := bash
GOEXE := $(shell go env GOEXE)
.PHONY: fmt test vet build lint ascii-check smoke check

fmt:
	go tool goimports -w .

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -o ./gh-star-lists$(GOEXE) .

lint:
	go vet ./...
	golangci-lint run --fix

smoke:
	bash scripts/smoke-local.sh

check:
	bash scripts/check.sh
