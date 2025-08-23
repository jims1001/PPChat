.PHONY: lint lint-fix test build

# Run all linters
lint:
	golangci-lint run ./...

# Run linters and auto-fix issues (e.g., formatting)
lint-fix:
	golangci-lint run --fix ./...

# Run unit tests with race condition detector
test:
	go test -v -race ./...


build:
	go build -o  main.go