BINARY = goggle

.PHONY: build test lint vet fmt check clean deps

build:
	go build -o $(BINARY) .

test:
	go test ./...

lint:
	golangci-lint run

vet:
	go vet ./...

fmt:
	gofmt -d .

check: vet lint test

clean:
	rm -f $(BINARY)

deps:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint
