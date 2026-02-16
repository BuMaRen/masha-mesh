# masha-mesh
for service mesh learning

## Quick Start

### Build and Deploy

Use the provided `deploy.sh` script to build, push, and deploy the application:

```bash
# Deploy with latest tag
./deploy.sh

# Deploy with a specific version
./deploy.sh v0.2.0
```

The script will:
1. Build binaries (mesh-cli and mesh-ctrl)
2. Build and push Docker images
3. Deploy to Kubernetes
4. Restart deployments to pull new images
5. Wait for rollout to complete

### Manual Build

```bash
# Build binaries only
make build

# Build and push Docker images
make push VERSION=v0.2.0

# Deploy to Kubernetes
kubectl apply -f ./build/cli/deployment.yml
kubectl apply -f ./build/ctrl/deployment.yml
kubectl apply -f ./build/ctrl/service.yml
```
