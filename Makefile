.PHONY: fmt vet build generate test test-race bdd test-all docker snyk
.DEFAULT_GOAL := build

fmt:
	go fmt ./...

vet:
	go vet ./...

generate:
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml

build: generate
	go build -o bin/bucket-brigade ./cmd/bucket-brigade

test:
	go test ./...

test-race:
	go test -race ./...

bdd:
	go test ./bdd/... -v

test-all: test test-race bdd

docker:
	DOCKER_BUILDKIT=1 docker build -t bucket-brigade .

snyk:
	snyk test
