# Performance Dashboard Backend

This is a Go (Golang) backend project scaffolded with a standard structure.

## Structure
- `cmd/` - Main applications for this project
- `pkg/` - Library code that's ok to use by external applications
- `internal/` - Private application and library code

## Getting Started

1. Make sure you have Go installed: https://golang.org/dl/
2. Initialize the Go module (if not already):
   ```sh
   go mod init performance-dashboard-backend
   ```
3. Build and run:
   ```sh
   go run ./cmd
   ```
4. For deployment build and run
    ```sh
   go build -tags netgo -ldflags "-s -w" -o main ./cmd
   ./main
   ```

## Project Status
- [ ] Initial scaffolding
- [ ] Add business logic
- [ ] Add tests
