.PHONY: all build clean proto build-control-plane build-sidecar build-backend

# Variables
GO=go
PROTOC=protoc
BIN_DIR=bin
PROTO_DIR=pkg/api

# Default target
all: build

# Install dependencies
deps:
	$(GO) mod download
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate protobuf code
proto:
	$(PROTOC) --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/control_plane.proto

# Build all components
build: build-control-plane build-sidecar build-backend

# Build control-plane
build-control-plane:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/control-plane ./cmd/control-plane

# Build sidecar
build-sidecar:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/sidecar ./cmd/sidecar

# Build example backend
build-backend:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/backend ./examples/backend

# Run control-plane
run-control-plane: build-control-plane
	./$(BIN_DIR)/control-plane

# Run sidecar in L7 mode
run-sidecar-l7: build-sidecar
	./$(BIN_DIR)/sidecar -mode l7 -listen :8000 -target localhost:9001

# Run sidecar in L4 mode
run-sidecar-l4: build-sidecar
	./$(BIN_DIR)/sidecar -mode l4 -listen :8000 -target localhost:9001

# Run example backend
run-backend: build-backend
	./$(BIN_DIR)/backend -port 9001 -name backend-1

# Clean build artifacts
clean:
	rm -rf $(BIN_DIR)

# Run tests
test:
	$(GO) test -v ./...

# Format code
fmt:
	$(GO) fmt ./...

# Run go mod tidy
tidy:
	$(GO) mod tidy

# Build for multiple platforms
build-all:
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BIN_DIR)/control-plane-linux-amd64 ./cmd/control-plane
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BIN_DIR)/sidecar-linux-amd64 ./cmd/sidecar
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BIN_DIR)/control-plane-darwin-amd64 ./cmd/control-plane
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BIN_DIR)/sidecar-darwin-amd64 ./cmd/sidecar
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BIN_DIR)/control-plane-windows-amd64.exe ./cmd/control-plane
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BIN_DIR)/sidecar-windows-amd64.exe ./cmd/sidecar
