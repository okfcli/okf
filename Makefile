.PHONY: build test test-race cover cover-html vet check clean tidy install

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/okf ./cmd/okf

test:
	go test ./...

test-race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...

cover-html: cover
	go tool cover -html=coverage.out -o coverage.html

vet:
	go vet ./...

check: vet test

clean:
	rm -rf bin/ coverage.out coverage.html

tidy:
	go mod tidy

install: build
	cp bin/okf $(GOPATH)/bin/okf
