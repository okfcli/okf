.PHONY: build test test-race cover cover-html vet check clean tidy install

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/okf ./cmd/okf

tidy:
	go mod tidy

test:
	go test ./...

test-race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

cover-html: cover
	go tool cover -html=coverage.out -o coverage.html

vet:
	go vet ./...

check: vet test-race

clean:
	rm -rf bin/ coverage.out coverage.html

install: build
	cp bin/okf $(GOPATH)/bin/okf
