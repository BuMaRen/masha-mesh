# masha-mesh

A lightweight service mesh for learning purposes, featuring a Kubernetes in-cluster control plane application.

## Overview

`masha-mesh` is an educational service mesh project that demonstrates core concepts of service mesh architecture. The control-face component is a Kubernetes-native control plane application that performs service discovery and provides an API for mesh management.

## Components

### control-face

The control-face is a Kubernetes in-cluster application that acts as the control plane for the service mesh. It provides:

- **Service Discovery**: Automatically discovers and tracks Kubernetes services and their endpoints
- **Pod Monitoring**: Watches pod lifecycle events to maintain accurate endpoint information
- **REST API**: HTTP API for querying service mesh state
- **Health Checks**: Built-in health and readiness probes for Kubernetes integration

## Features

- ✅ In-cluster Kubernetes configuration
- ✅ Service and endpoint discovery
- ✅ Real-time event watching
- ✅ RESTful API for service information
- ✅ Health and readiness probes
- ✅ RBAC-compliant deployment
- ✅ Containerized deployment

## Prerequisites

- Go 1.20 or later
- Docker (for building container images)
- Kubernetes cluster (for deployment)
- kubectl configured to access your cluster

## Quick Start

### Building

```bash
# Build the binary
make build

# Build Docker image
make docker-build
```

### Deploying to Kubernetes

```bash
# Deploy control-face to your cluster
make deploy

# Check deployment status
kubectl get pods -n masha-mesh

# Check service
kubectl get svc -n masha-mesh
```

### Testing the API

Once deployed, you can access the API through the service:

```bash
# Port-forward to the service
kubectl port-forward -n masha-mesh svc/control-face 8080:8080

# Health check
curl http://localhost:8080/healthz

# List all discovered services
curl http://localhost:8080/api/v1/services

# Get specific service
curl http://localhost:8080/api/v1/services/default/kubernetes
```

## API Endpoints

- `GET /healthz` - Health check endpoint
- `GET /readyz` - Readiness check endpoint
- `GET /api/v1/services` - List all discovered services
- `GET /api/v1/services/{namespace}/{name}` - Get specific service details

## Architecture

```
┌─────────────────────────────────────┐
│         control-face                │
│  ┌──────────────────────────────┐  │
│  │   Service Controller         │  │
│  │   - Watches Services         │  │
│  │   - Watches Pods             │  │
│  │   - Tracks Endpoints         │  │
│  └──────────────────────────────┘  │
│  ┌──────────────────────────────┐  │
│  │   HTTP API Server            │  │
│  │   - /api/v1/services         │  │
│  │   - /healthz, /readyz        │  │
│  └──────────────────────────────┘  │
└─────────────────────────────────────┘
            │
            ▼
    ┌───────────────┐
    │  Kubernetes   │
    │  API Server   │
    └───────────────┘
```

## Development

### Running Locally

To run the control-face locally (requires valid kubeconfig):

```bash
# Run with default settings
make run

# Or run directly with custom flags
go run ./cmd/control-face -v=2 -port=8080
```

### Code Structure

```
.
├── cmd/
│   └── control-face/        # Main application entry point
├── pkg/
│   ├── controller/          # Kubernetes controllers
│   └── server/              # HTTP API server
├── deployments/             # Kubernetes manifests
├── Dockerfile               # Container image definition
├── Makefile                 # Build automation
└── README.md               # This file
```

### Testing

```bash
# Run tests
make test

# Format code
make fmt

# Run vet
make vet
```

## Cleanup

To remove the deployment from your cluster:

```bash
make undeploy
```

## Configuration

The control-face application supports the following command-line flags:

- `-port`: HTTP server port (default: 8080)
- `-namespace`: Namespace to watch (empty for all namespaces)
- `-v`: Log verbosity level

## RBAC Permissions

The control-face requires the following Kubernetes permissions:

- `get`, `list`, `watch` on `services`, `endpoints`, `pods`
- `get`, `list` on `namespaces`

These are defined in the ClusterRole in `deployments/control-face.yaml`.

## License

See LICENSE file for details.

## Contributing

This is an educational project. Contributions and improvements are welcome!

