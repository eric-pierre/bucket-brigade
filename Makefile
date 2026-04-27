.PHONY: fmt vet build test test-race
.DEFAULT_GOAL := build

fmt:
	go fmt ./...

vet:
	go vet ./...

build:
	go build -o bin/main .

test:
	go test ./...

test-race:
	go test -race ./...
