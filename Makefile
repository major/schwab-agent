.PHONY: build test lint clean install

build:
	go build -o schwab-agent ./cmd/schwab-agent/

test:
	go test -v ./...

lint:
	golangci-lint run ./...

clean:
	go clean
	rm -f schwab-agent
	rm -rf dist/

install:
	go install ./cmd/schwab-agent/
