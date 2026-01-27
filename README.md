# Masha-Mesh Service Mesh

A lightweight service mesh learning project implementing Layer 4 (TCP) and Layer 7 (HTTP) proxies with a control plane for service discovery and configuration distribution.

## Architecture

```
┌─────────────────┐
│  Control Plane  │
│   :8080/:9090   │
└────────┬────────┘
         │ gRPC
         │
    ┌────┴────┐
    │         │
┌───▼───┐ ┌──▼────┐
│Sidecar│ │Sidecar│
│ :8000 │ │ :8001 │
└───┬───┘ └───┬───┘
    │         │
┌───▼───┐ ┌──▼────┐
│Backend│ │Backend│
│ :9001 │ │ :9002 │
└───────┘ └───────┘
```

## Components

### 1. Control Plane

The control plane manages service discovery and publishes configurations to sidecars.

**Features:**
- Service registry for tracking service endpoints
- gRPC API for sidecars to fetch configurations
- HTTP API for service management
- Streaming configuration updates to sidecars

**Ports:**
- `:8080` - HTTP API (REST)
- `:9090` - gRPC API

**Endpoints:**
- `GET /services` - List all registered services
- `GET /health` - Health check

### 2. Sidecar

The sidecar proxy intercepts traffic and forwards it to backend services.

**Features:**
- L4 (TCP) proxy for transparent TCP forwarding
- L7 (HTTP) proxy with load balancing
- Control plane integration for dynamic configuration
- Round-robin load balancing

**Modes:**
- `l4` - Layer 4 TCP proxy
- `l7` - Layer 7 HTTP proxy

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Protocol Buffers compiler (`protoc`)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/BuMaRen/masha-mesh.git
cd masha-mesh
```

2. Install dependencies:
```bash
go mod download
```

3. Generate gRPC code (if modified):
```bash
make proto
```

### Building

Build all components:
```bash
make build
```

Or build individually:
```bash
make build-control-plane
make build-sidecar
make build-backend
```

## Usage

### Running the Control Plane

```bash
./bin/control-plane
```

The control plane will start on:
- HTTP API: `localhost:8080`
- gRPC API: `localhost:9090`

### Running a Sidecar

**L4 Mode (TCP Proxy):**
```bash
./bin/sidecar -mode l4 -listen :8000 -target localhost:9001
```

**L7 Mode (HTTP Proxy) with Static Target:**
```bash
./bin/sidecar -mode l7 -listen :8000 -target localhost:9001
```

**L7 Mode with Control Plane Integration:**
```bash
./bin/sidecar -mode l7 -listen :8000 -enable-cp -service example-service -control-plane localhost:9090
```

### Running Example Backend Services

```bash
./bin/backend -port 9001 -name backend-1
./bin/backend -port 9002 -name backend-2
```

## Example Scenarios

### Scenario 1: Simple L7 Proxy

1. Start a backend service:
```bash
go run examples/backend/main.go -port 9001 -name backend-1
```

2. Start sidecar proxy:
```bash
go run cmd/sidecar/main.go -mode l7 -listen :8000 -target localhost:9001
```

3. Test the proxy:
```bash
curl http://localhost:8000/
```

### Scenario 2: L7 Proxy with Control Plane

1. Start the control plane:
```bash
go run cmd/control-plane/main.go
```

2. Start multiple backend services:
```bash
go run examples/backend/main.go -port 9001 -name backend-1
go run examples/backend/main.go -port 9002 -name backend-2
```

3. Register services with control plane:
```bash
# The control plane automatically registers example-service with localhost:8001 and localhost:8002
# You can view registered services:
curl http://localhost:8080/services
```

4. Start sidecar with control plane integration:
```bash
go run cmd/sidecar/main.go -mode l7 -listen :8000 -enable-cp -service example-service -control-plane localhost:9090
```

5. Test load balancing:
```bash
curl http://localhost:8000/
curl http://localhost:8000/
curl http://localhost:8000/
```

Each request will be load-balanced across the registered backends.

### Scenario 3: L4 TCP Proxy

1. Start a TCP service (e.g., a simple HTTP server):
```bash
go run examples/backend/main.go -port 9001 -name backend-1
```

2. Start L4 sidecar:
```bash
go run cmd/sidecar/main.go -mode l4 -listen :8000 -target localhost:9001
```

3. Test the TCP proxy:
```bash
curl http://localhost:8000/
```

## Configuration

### Sidecar Flags

- `-mode` - Proxy mode: `l4` or `l7` (default: `l7`)
- `-listen` - Listen address (default: `:8000`)
- `-target` - Target address for static routing (default: `localhost:8001`)
- `-control-plane` - Control plane address (default: `localhost:9090`)
- `-service` - Service name for control plane lookup (default: `example-service`)
- `-enable-cp` - Enable control plane integration (default: `false`)

### Backend Flags

- `-port` - Port to listen on (default: `8001`)
- `-name` - Service name (default: `backend-1`)

## API Reference

### Control Plane gRPC API

#### GetServiceConfig
Get configuration for a service:
```protobuf
rpc GetServiceConfig(ServiceConfigRequest) returns (ServiceConfigResponse);
```

#### RegisterService
Register a new service endpoint:
```protobuf
rpc RegisterService(ServiceRegistration) returns (RegistrationResponse);
```

#### StreamConfig
Stream configuration updates:
```protobuf
rpc StreamConfig(ServiceConfigRequest) returns (stream ServiceConfigResponse);
```

### Control Plane HTTP API

#### GET /services
Returns all registered services and their endpoints.

Response:
```json
{
  "example-service": [
    "localhost:8001",
    "localhost:8002"
  ]
}
```

#### GET /health
Health check endpoint.

## Development

### Project Structure

```
masha-mesh/
├── cmd/
│   ├── control-plane/    # Control plane service
│   │   └── main.go
│   └── sidecar/          # Sidecar proxy
│       └── main.go
├── pkg/
│   ├── api/              # gRPC API definitions
│   │   ├── control_plane.proto
│   │   ├── control_plane.pb.go
│   │   └── control_plane_grpc.pb.go
│   └── proxy/            # Proxy implementations
│       ├── l4_proxy.go   # Layer 4 TCP proxy
│       └── l7_proxy.go   # Layer 7 HTTP proxy
├── examples/
│   └── backend/          # Example backend service
│       └── main.go
├── Makefile
├── go.mod
└── README.md
```

### Building from Source

```bash
# Clean build artifacts
make clean

# Build all binaries
make build

# Run tests (when available)
make test
```

## Features

### Implemented

- ✅ Layer 4 (TCP) transparent proxy
- ✅ Layer 7 (HTTP) reverse proxy
- ✅ Control plane service discovery
- ✅ gRPC-based control plane to sidecar communication
- ✅ Round-robin load balancing
- ✅ Dynamic configuration updates via streaming
- ✅ Service registration and discovery

### Future Enhancements

- Circuit breaking
- Retry logic
- Request tracing
- Metrics and observability
- mTLS support
- Advanced load balancing algorithms (least connections, weighted round-robin)
- Health checking

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Learning Resources

This project demonstrates core service mesh concepts:

1. **Service Discovery**: Control plane maintains a registry of services
2. **Traffic Management**: Sidecars intercept and route traffic
3. **Load Balancing**: Distribute requests across multiple backends
4. **Control Plane Pattern**: Centralized configuration management
5. **Data Plane Pattern**: Distributed proxies handling actual traffic

## Contributing

Contributions are welcome! This is a learning project, so feel free to:
- Add new features
- Improve existing implementations
- Add tests
- Enhance documentation

## Acknowledgments

This project is created for learning purposes to understand service mesh concepts and implementations.
