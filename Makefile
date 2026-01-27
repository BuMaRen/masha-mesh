.PHONY: build clean docker-build docker-push deploy undeploy test fmt vet

# Variables
APP_NAME = control-face
IMAGE_NAME = control-face
IMAGE_TAG = latest
REGISTRY ?= localhost

# Build the binary
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p bin
	@go build -o bin/$(APP_NAME) ./cmd/$(APP_NAME)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Push Docker image (customize REGISTRY as needed)
docker-push: docker-build
	@echo "Pushing Docker image..."
	@docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
	@docker push $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

# Deploy to Kubernetes
deploy:
	@echo "Deploying to Kubernetes..."
	@kubectl apply -f deployments/control-face.yaml

# Remove from Kubernetes
undeploy:
	@echo "Removing from Kubernetes..."
	@kubectl delete -f deployments/control-face.yaml

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy

# Run locally (requires kubeconfig)
run:
	@echo "Running locally (requires valid kubeconfig)..."
	@go run ./cmd/$(APP_NAME) -v=2

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-push   - Push Docker image to registry"
	@echo "  deploy        - Deploy to Kubernetes"
	@echo "  undeploy      - Remove from Kubernetes"
	@echo "  test          - Run tests"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  tidy          - Tidy dependencies"
	@echo "  run           - Run locally"
