# List available tasks
list:
    @just --list

# Build the application binary
build:
    @echo "Building tunecli..."
    @go build -o tunecli ./cmd/tunecli

# Run the application (builds it first)
run: build
    @echo "Running tunecli..."
    @./tunecli

# Format all Go source files
fmt:
    @echo "Formatting code..."
    @go fmt ./...

# Tidy the mod file
tidy:
    @echo "Tidying dependencies..."
    @go mod tidy

# Lint the codebase
lint:
    @echo "Linting code..."
    @golangci-lint run ./...
