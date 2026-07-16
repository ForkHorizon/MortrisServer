.PHONY: fmt lint test build

fmt:
	gofmt -l -w .

lint:
	go vet ./...
	gofmt -l . | (! grep .)

test:
	go test ./...

build:
	go build -o bin/analytics-server ./cmd/analytics-server
