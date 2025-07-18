.PHONY: build clean test install

# Build the crawler
build:
	go build -o crawler main.go

# Clean build artifacts
clean:
	rm -f crawler
	rm -rf responses/
	rm -rf output/
	rm -rf results/

# Install dependencies
deps:
	go mod tidy
	go mod download

# Run tests (if any)
test:
	go test ./...

# Install the crawler
install: build
	cp crawler /usr/local/bin/

# Development build with race detection
dev: clean
	go build -race -o crawler main.go

# Build for different platforms
build-linux:
	GOOS=linux GOARCH=amd64 go build -o crawler-linux main.go

build-windows:
	GOOS=windows GOARCH=amd64 go build -o crawler.exe main.go

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o crawler-darwin main.go

# Build all platforms
build-all: build-linux build-windows build-darwin

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the crawler"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Install dependencies"
	@echo "  test         - Run tests"
	@echo "  install      - Install to /usr/local/bin"
	@echo "  dev          - Development build with race detection"
	@echo "  build-linux  - Build for Linux"
	@echo "  build-windows- Build for Windows"
	@echo "  build-darwin - Build for macOS"
	@echo "  build-all    - Build for all platforms"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code" 