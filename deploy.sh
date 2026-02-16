#!/bin/bash

# deploy.sh - Build, push, and deploy masha-mesh application
# Usage: ./deploy.sh [VERSION]
# Example: ./deploy.sh v0.2.0
#          ./deploy.sh (defaults to latest)

set -e

VERSION=${1:-latest}

echo "=== Masha Mesh Deployment Script ==="
echo "Version: $VERSION"
echo ""

# Step 1: Build binaries
echo "Step 1: Building binaries..."
make build VERSION=$VERSION
echo "✓ Binaries built successfully"
echo ""

# Step 2: Push Docker images
echo "Step 2: Pushing Docker images..."
make push VERSION=$VERSION
echo "✓ Docker images pushed successfully"
echo ""

# Step 3: Deploy to Kubernetes
echo "Step 3: Deploying to Kubernetes..."
kubectl apply -f ./build/cli/deployment.yml
kubectl apply -f ./build/ctrl/deployment.yml
kubectl apply -f ./build/ctrl/service.yml
echo "✓ Kubernetes resources deployed successfully"
echo ""

# Step 4: Restart deployments to pull new images
echo "Step 4: Restarting deployments to pull new images..."
kubectl rollout restart deployment/mesh-cli
kubectl rollout restart deployment/mesh-ctrl
echo "✓ Deployments restarted successfully"
echo ""

# Step 5: Wait for rollout to complete
echo "Step 5: Waiting for rollout to complete..."
kubectl rollout status deployment/mesh-cli
kubectl rollout status deployment/mesh-ctrl
echo "✓ Rollout completed successfully"
echo ""

echo "=== Deployment Complete ==="
echo "To check status: kubectl get pods"
echo "To view logs: kubectl logs -f deployment/mesh-ctrl"
