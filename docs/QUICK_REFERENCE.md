# Control-Face Quick Reference

## What is control-face?

The control-face is a Kubernetes in-cluster application that serves as the control plane for the masha-mesh service mesh. It performs service discovery and provides an API for mesh management.

## Core Components

### 1. Service Controller
- Watches Kubernetes Services and Pods
- Maintains real-time service topology
- Tracks endpoints for each service
- Updates automatically on cluster changes

### 2. HTTP API Server
- Provides REST API for service queries
- Health and readiness endpoints for Kubernetes
- JSON responses for easy integration

## API Endpoints

### Health & Status
- `GET /healthz` - Returns `{"status":"healthy"}`
- `GET /readyz` - Returns `{"status":"ready"}`

### Service Discovery
- `GET /api/v1/services` - List all discovered services
  ```json
  {
    "services": {
      "default/kubernetes": {
        "name": "kubernetes",
        "namespace": "default",
        "clusterIp": "10.96.0.1",
        "ports": [443],
        "endpoints": ["192.168.1.1"],
        "labels": {}
      }
    },
    "count": 1
  }
  ```

- `GET /api/v1/services/{namespace}/{name}` - Get specific service
  ```json
  {
    "name": "kubernetes",
    "namespace": "default",
    "clusterIp": "10.96.0.1",
    "ports": [443],
    "endpoints": ["192.168.1.1"],
    "labels": {}
  }
  ```

## Deployment

### Prerequisites
- Kubernetes cluster (v1.19+)
- kubectl configured
- Docker (for building images)

### Quick Deploy
```bash
# Build and deploy
make docker-build
make deploy

# Verify
kubectl get pods -n masha-mesh
kubectl get svc -n masha-mesh
```

### Access the API
```bash
# Port-forward to local machine
kubectl port-forward -n masha-mesh svc/control-face 8080:8080

# Test in another terminal
curl http://localhost:8080/api/v1/services
```

## Configuration

Command-line flags:
- `-port` - HTTP server port (default: 8080)
- `-namespace` - Namespace to watch (empty = all namespaces)
- `-v` - Log verbosity (0-4, default: 0)

## RBAC Permissions

The control-face requires minimal permissions:
- Read-only access to Services, Pods, Endpoints
- No write permissions required
- Cluster-scoped for multi-namespace discovery

## Troubleshooting

### Check pod logs
```bash
kubectl logs -n masha-mesh deployment/control-face
```

### Check RBAC
```bash
kubectl auth can-i list services --as=system:serviceaccount:masha-mesh:control-face
```

### Test connectivity
```bash
kubectl exec -it -n masha-mesh deployment/control-face -- wget -O- localhost:8080/healthz
```

## Development

### Run locally
Requires valid kubeconfig:
```bash
go run ./cmd/control-face -v=2 -port=8080
```

### Run tests
```bash
make test
```

### Build binary
```bash
make build
./bin/control-face --help
```

## Architecture

```
┌─────────────────────────────────────┐
│    Kubernetes API Server            │
│    - Services                       │
│    - Pods                           │
│    - Endpoints                      │
└─────────────────┬───────────────────┘
                  │
                  │ Watch API
                  ▼
┌─────────────────────────────────────┐
│         control-face                │
│  ┌──────────────────────────────┐  │
│  │   Service Controller         │  │
│  │   - Sync initial state       │  │
│  │   - Watch for changes        │  │
│  │   - Update service map       │  │
│  └──────────────────────────────┘  │
│  ┌──────────────────────────────┐  │
│  │   HTTP Server                │  │
│  │   - Handle API requests      │  │
│  │   - Return JSON responses    │  │
│  └──────────────────────────────┘  │
└─────────────────────────────────────┘
                  │
                  │ HTTP
                  ▼
          ┌───────────────┐
          │   API Clients │
          └───────────────┘
```

## Next Steps

This is a learning project. Potential enhancements:
1. Add gRPC xDS server for Envoy integration
2. Implement custom CRDs for mesh policies
3. Add admission webhooks for sidecar injection
4. Support for traffic routing rules
5. Integration with observability tools
6. Multi-cluster support
