# Copilot Instructions for BSS Metrics API Server

## Project Overview
This is a Go-based API server that bridges Linux kernel scheduler metrics (BSS) with Kubernetes orchestration. The core purpose is to collect scheduling metrics from eBPF programs and provide intelligent scheduling strategies back to the kernel based on Kubernetes pod labels and process patterns.

## Architecture & Key Components

### Core Data Flow
1. **Metrics Collection**: eBPF programs send BSS scheduler metrics to `/api/v1/metrics`
2. **Process Discovery**: Server scans `/proc` filesystem to map processes to Kubernetes pods via cgroup parsing
3. **Strategy Generation**: Combines user-defined strategies with live pod label data to generate scheduling decisions
4. **Strategy Delivery**: Returns concrete PID-based scheduling strategies to the kernel scheduler

### Critical File Structure
- `main.go`: Single-file monolith containing all HTTP handlers and core logic
- `kubernetes.go`: K8s client with caching layer for pod metadata
- `config.go`: Configuration with default scheduling strategies
- `options.go`: CLI argument parsing with dual-mode support (in-cluster vs external)

## Development Patterns

### Data Structure Conventions
All API responses follow this pattern:
```go
type Response struct {
    Success   bool   `json:"success"`
    Message   string `json:"message"`
    Timestamp string `json:"timestamp"`
    // ... specific data fields
}
```

### Scheduling Strategy Resolution
The system uses a two-phase approach:
1. **Template Strategies**: Defined with label selectors and regex patterns
2. **Concrete Strategies**: Resolved to specific PIDs for kernel consumption

Example template to concrete transformation:
```go
// Template (from config/API)
{
    "selectors": [{"key": "nf", "value": "upf"}],
    "command_regex": "nr-gnb|ping",
    "execution_time": 20000000
}

// Becomes multiple concrete strategies
[
    {"pid": 12345, "execution_time": 20000000},
    {"pid": 12346, "execution_time": 20000000}
]
```

### Kubernetes Integration Patterns
- **Dual Mode Support**: Always handle both in-cluster (`--in-cluster=true`) and external kubeconfig modes
- **Graceful Degradation**: If K8s client fails, continue with empty pod labels rather than crashing
- **Caching Strategy**: 30-second TTL on pod label lookups to reduce API pressure
- **RBAC Requirements**: Needs `pods` and `namespaces` read access (see `k8s/deployment.yaml`)

### Error Handling Approach
- Return structured JSON errors, never plain text
- Log errors but don't expose internal details in API responses
- Use `log.Printf()` for all logging (no structured logging framework)

## Key Development Workflows

### Running & Testing
```bash
# Local development with external K8s
make run
# or with specific kubeconfig
go run main.go --kubeconfig=/path/to/config

# Testing strategy APIs
make test-strategies

# Container deployment
make docker-build && make k8s-deploy
```

### Adding New Scheduling Logic
1. Extend `SchedulingStrategy` struct in `main.go`
2. Update `findPIDsByStrategy()` function for new matching logic
3. Modify template-to-concrete resolution in `GetSchedulingStrategiesHandler`
4. Update default config in `config.go`

### Process-to-Pod Mapping Logic
The system parses `/proc/<pid>/cgroup` looking for kubepods patterns:
- Format: `/kubepods/burstable/pod<uid>/<container-id>`
- Extracts pod UID, then queries K8s API for labels
- Critical for linking kernel processes to pod scheduling policies

## Integration Points

### External Dependencies
- **Gorilla Mux**: HTTP routing (`github.com/gorilla/mux`)
- **Kubernetes Client**: Official Go client for pod metadata
- **Linux /proc filesystem**: Direct parsing for process discovery

### API Contract with eBPF Clients
- BSS metrics use specific field names (e.g., `nr_queued`, `usersched_last_run_at`)
- Scheduling strategies return nanosecond `execution_time` values
- All timestamps in RFC3339 format

### Deployment Considerations
- Requires privileged access to `/proc` filesystem
- Needs K8s RBAC permissions for pod/namespace reads
- Typically deployed in `kube-system` namespace
- Health checks on `/health` endpoint

## Common Gotchas
- Pod UIDs from cgroup paths need underscore-to-dash conversion
- Strategy resolution happens on every GET request (no caching)
- Kubernetes client initialization is lazy and retried
- Configuration file is optional; defaults are comprehensive
- All scheduling times are in nanoseconds, not milliseconds
