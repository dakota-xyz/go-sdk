.PHONY: generate generate-client test test-race vet fmt

generate: generate-client

generate-client:
	./scripts/generate-client.sh

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')
