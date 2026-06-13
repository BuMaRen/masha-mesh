# Proxy Module Implementation

## Overview

This proxy module implements a unified listener architecture similar to Envoy, handling all types of traffic (TCP, gRPC, HTTP/HTTPS) through a single entry point. Traffic is automatically detected and routed appropriately.

## Architecture

The proxy uses a unified listener design inspired by Envoy:

```
┌─────────────────────────────────────────────────────────────┐
│                      Unified Listener                        │
│                      (listener.go)                           │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  1. Accept Connection                                │   │
│  │  2. Peek at first bytes to detect protocol          │   │
│  │  3. Route to appropriate handler                    │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
│         ┌─────────────────┬──────────────────┐              │
│         │                 │                  │              │
│    ┌────▼─────┐     ┌────▼─────┐     ┌─────▼────┐         │
│    │   HTTP   │     │   TCP    │     │  gRPC    │         │
│    │ Handler  │     │ Handler  │     │ Handler  │         │
│    └──────────┘     └──────────┘     └──────────┘         │
│         │                 │                  │              │
│         │                 │                  │              │
│    ┌────▼─────┐     ┌────▼─────┐     ┌─────▼────┐         │
│    │HTTP Trans│     │TCP Trans-│     │gRPC Trans│         │
│    │  port    │     │  parent  │     │  parent  │         │
│    └──────────┘     └──────────┘     └──────────┘         │
└─────────────────────────────────────────────────────────────┘
```

### Components

1. **Listener** (`listener.go`): Unified TCP listener that accepts all connections
2. **HTTP Transport** (`http_transport.go`): HTTP request routing with load balancing
3. **Server** (`server.go`): Main proxy orchestration
4. **Configuration** (`proxyconfig/`): YAML-based configuration
5. **Circuit Breaker** (`breaker/`): Request failure tracking

## Traffic Handling

### TCP Traffic
- **Behavior**: Transparent pass-through using original destination
- **Detection**: Any non-HTTP traffic
- **Configurable**: No

### gRPC Traffic
- **Behavior**: Transparent pass-through (detected as non-HTTP)
- **Detection**: Treated as TCP traffic
- **Configurable**: No

### HTTP/HTTPS Traffic
- **Behavior**: Configurable via YAML
- **Detection**: Automatic by inspecting first bytes (GET, POST, etc.)
- **Configurable**: Yes

#### HTTP Proxy Rules

1. **IP-based requests** (e.g., `http://192.168.1.100:8080`)
   - Always passed through directly
   - No proxying or load balancing
   - Configuration ignored

2. **Domain-based requests** (e.g., `http://my-service:8080`)
   - Checked against proxy configuration
   - If configured with `loadBalance: true`, proxy to all backend IPs
   - If configured with `loadBalance: false`, pass through directly
   - If no configuration exists, pass through directly

## Usage

### Starting the Proxy

```bash
mesh-cli --listen-address :8081 --config /path/to/proxy-config.yaml
```

### Configuration File

See [build/cli/configs/proxy-example.yaml](../../../build/cli/configs/proxy-example.yaml) for a complete example.

```yaml
http:
  enabled: true
  proxies:
    my-service:
      loadBalance: true
      useBreaker: true
    
    another-service:
      loadBalance: true
      useBreaker: false
```

### Circuit Breaker Configuration

Circuit breaker parameters are configured via command-line flags:

```bash
--window-size 20                    # Window size in seconds
--half-open-allowed 10              # Requests allowed in half-open state
--min-request-count 20              # Minimum requests before considering open state
--failure-rate-threshold 0.5        # Failure rate threshold (0.0-1.0)
--half-open-max-duration 60         # Max half-open duration in seconds
--open-duration 60                  # Open state duration in seconds
```

## Implementation Details

### Module Structure

```
proxy/
├── server.go           # Main proxy server orchestration
├── listener.go         # Unified listener (accepts all connections)
├── http_transport.go   # HTTP RoundTripper with load balancing
├── options.go          # Command-line options
└── README.md           # This file

proxyconfig/
└── config.go           # YAML configuration structures
```

### Key Design Decisions

#### Why Unified Listener?

Like Envoy, we use a single listener that:
- Accepts all TCP connections on one port
- Automatically detects protocol by inspecting connection data
- Routes to appropriate handler based on protocol
- Simplifies deployment (single port, no complex routing)

#### Protocol Detection

HTTP detection is done by peeking at the first 64 bytes:
```go
func isHTTP(reader *bufio.Reader) bool {
    peek, _ := reader.Peek(64)
    return bytes.HasPrefix(peek, []byte("GET ")) ||
           bytes.HasPrefix(peek, []byte("POST ")) ||
           // ... other HTTP methods
}
```

#### Connection Handling

- **HTTP**: Wrapped in a single-connection HTTP server, processed by ReverseProxy
- **TCP**: Direct bidirectional copy between client and original destination
- **Original Destination**: Retrieved using `SO_ORIGINAL_DST` socket option (requires iptables REDIRECT)

### Load Balancing

When `loadBalance: true` for a service:
1. Resolve service name to all available IPs via mesh client
2. Try each IP in sequence until success
3. Record success/failure with circuit breaker (if enabled)
4. Return first successful response

### Circuit Breaker Integration

When `useBreaker: true` for a service:
1. Check if endpoint is allowed before sending request
2. Record outcome: success, network failure, timeout, or business failure
3. Breaker automatically opens after threshold failures
4. Breaker automatically transitions through half-open to closed states

### Error Handling

- **Network errors**: Connection refused, timeout → Record as network failure
- **HTTP 5xx errors**: Server errors → Record as business failure  
- **HTTP 4xx errors**: Client errors → Considered success (not backend's fault)
- **HTTP 2xx-3xx**: Success → Record as success

## Example Configuration

```yaml
http:
  enabled: true
  proxies:
    # Load balance with circuit breaker protection
    payment-service:
      loadBalance: true
      useBreaker: true
    
    # Load balance without circuit breaker
    cache-service:
      loadBalance: true
      useBreaker: false
    
    # Pass through (no proxying)
    legacy-service:
      loadBalance: false
      useBreaker: false
```

## Comparison with Previous Architecture

### Before (L4/L7 Split)
```
┌─────────┐     ┌─────────┐
│L4 Proxy │────▶│L7 Proxy │
│  :8081  │     │  :8080  │
└─────────┘     └─────────┘
   │                 │
   ▼                 ▼
 TCP/gRPC         HTTP
```

### After (Unified Listener)
```
┌──────────────────┐
│Unified Listener  │
│     :8081        │
└──────────────────┘
   │
   ├─▶ HTTP
   ├─▶ TCP
   └─▶ gRPC
```

**Benefits:**
- Single port, simpler deployment
- No internal forwarding overhead
- Cleaner architecture, easier to maintain
- More like Envoy's design philosophy

## Testing

To test the proxy:

1. Start the proxy with a configuration file
2. Send HTTP requests to domain-based services
3. Observe load balancing across backend IPs
4. Trigger failures to test circuit breaker behavior

## Future Enhancements

Potential improvements:
- Metrics/Prometheus integration for request statistics
- Advanced load balancing algorithms (round-robin, least-connections, etc.)
- Health check probes for backend endpoints
- Request retry with exponential backoff
- Protocol-specific optimizations for gRPC
- TLS/HTTPS support
- WebSocket support
