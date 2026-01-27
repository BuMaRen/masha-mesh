#!/bin/bash

# Demo script for Masha-Mesh Service Mesh
# This script demonstrates the key features of the service mesh

set -e

echo "==================================="
echo "Masha-Mesh Service Mesh Demo"
echo "==================================="
echo ""

# Function to check if a port is in use
check_port() {
    if lsof -Pi :$1 -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "Warning: Port $1 is already in use"
        return 1
    fi
    return 0
}

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    jobs -p | xargs -r kill 2>/dev/null || true
    wait 2>/dev/null || true
    echo "Done!"
}

trap cleanup EXIT

# Start backend services
echo "Starting backend services..."
./bin/backend -port 9001 -name backend-1 > /dev/null 2>&1 &
./bin/backend -port 9002 -name backend-2 > /dev/null 2>&1 &
sleep 1

# Start control plane
echo "Starting control plane..."
./bin/control-plane > /dev/null 2>&1 &
sleep 2

# Start L7 sidecar with control plane integration
echo "Starting L7 sidecar with control plane integration..."
./bin/sidecar -mode l7 -listen :8000 -enable-cp -service example-service -control-plane localhost:9090 > /dev/null 2>&1 &
sleep 2

echo ""
echo "==================================="
echo "All services started successfully!"
echo "==================================="
echo ""

# Test the setup
echo "Testing the service mesh..."
echo ""

echo "1. Control Plane API (GET /services):"
curl -s http://localhost:8080/services | jq
echo ""

echo "2. Making requests through L7 proxy (demonstrates load balancing):"
for i in {1..4}; do
    echo -n "   Request $i: "
    curl -s http://localhost:8000/ | jq -r '.service + " on port " + .port'
done
echo ""

echo "==================================="
echo "Demo completed!"
echo "==================================="
echo ""
echo "Services are running. You can test them with:"
echo "  - Control Plane API:    curl http://localhost:8080/services"
echo "  - Health Check:         curl http://localhost:8080/health"
echo "  - L7 Proxy (port 8000): curl http://localhost:8000/"
echo "  - Backend 1 (port 9001): curl http://localhost:9001/"
echo "  - Backend 2 (port 9002): curl http://localhost:9002/"
echo ""
echo "Press Ctrl+C to stop all services."

# Keep script running
wait
