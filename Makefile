.PHONY: build test run fmt

build:
	go build ./...

test:
	go test ./...

run:
	go run ./cmd/mymtr --help

fmt:
	gofmt -w .

