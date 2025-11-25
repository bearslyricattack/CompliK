# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 🚀 Essential Commands

### Building and Development
```bash
# Build the main binary
go build -o procscan cmd/procscan/main.go

# Build for production (Linux, no CGO)
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o bin/manager cmd/procscan/main.go

# Run locally for testing
./procscan --config=config.yaml

# Install/update dependencies
go mod tidy
go mod download
```

### Local Testing and Debugging
```bash
# Run comprehensive local debug script
./scripts/local-debug.sh

# Quick test with config validation
./scripts/quick-test.sh

# Test Kubernetes labeling functionality
./scripts/test-labels.sh

# Metrics testing (if enabled)
./procscan --config=config.yaml &
curl http://localhost:8080/metrics
```

### Deployment
```bash
# Deploy to Kubernetes
./scripts/deploy.sh

# Individual deployment steps
kubectl create namespace block-system
kubectl apply -f deploy/
kubectl get pods -n block-system -o wide
```

## 🏗️ Architecture Overview

### Core Components Architecture

ProcScan follows a clean architecture with clear separation of concerns:

```
cmd/procscan/main.go           # Application entry point with config watching
├── internal/core/scanner/    # Main scanning engine
├── internal/core/processor/  # Process analysis logic
├── internal/core/k8s/        # Kubernetes client interface
├── internal/core/alert/      # Alert aggregation and formatting
├── internal/notification/    # Notification systems (Lark)
└── pkg/                       # Shared utilities
    ├── metrics/               # Prometheus metrics collection
    ├── models/                # Configuration and data models
    ├── config/                # Configuration management
    └── logger/                # Logging utilities
```

### Key Design Patterns

1. **Scanner-Processor Separation**: The `Scanner` orchestrates the overall scanning loop, while `Processor` handles the detailed process analysis. This allows independent testing and optimization.

2. **Interface-Based External Services**: K8s clients and notifiers use interfaces, enabling easy mocking and alternative implementations.

3. **Configuration Hot-Reload**: The main process monitors the config file via file hashing, enabling zero-downtime configuration updates.

4. **Prometheus Integration**: Metrics are collected throughout the scanning lifecycle, providing comprehensive monitoring capabilities.

### Critical Data Flow

1. **Configuration Loading**: `main.go` → `LoadConfig()` → `models.Config` → `Scanner.NewScanner()`
2. **Scanning Loop**: `Scanner.runScanLoop()` → `scanProcesses()` → `processor.AnalyzeProcess()` → threat detection
3. **Response Pipeline**: Threat detected → `handleGroupedActions()` → K8s labeling → notifications

### Configuration Architecture

The configuration system uses a hierarchical YAML structure:

- **Scanner Config**: Core scanning parameters (interval, log level, proc path)
- **Detection Rules**: Blacklist/whitelist with regex patterns for processes, commands, and namespaces
- **Actions Config**: Automated responses (K8s labeling)
- **Notifications Config**: External alerting (Lark webhooks)
- **Metrics Config**: Prometheus server settings (port, path, timeouts)

## 🔧 Development Workflow

### Local Development Setup

1. **Use the local debug script**: `./scripts/local-debug.sh` checks dependencies and sets up the environment
2. **Config-driven testing**: Modify `config.yaml` for different test scenarios
3. **Metrics available on port 8080**: When metrics are enabled, access at `http://localhost:8080/metrics`

### Configuration Management

- **Production config**: Use `deploy/configmap.yaml` for Kubernetes deployments
- **Local development**: Use `config.yaml` in the project root
- **Testing configs**: Use `examples/config-with-metrics.yaml` as a template

### Process Analysis Logic

The scanning engine works by:
1. Enumerating `/proc` directory (container needs `/host/proc` mount)
2. Extracting process metadata (PID, command line, container info)
3. Applying blacklist rules first, then whitelist exceptions
4. Grouping results by Kubernetes namespace
5. Executing automated responses (labeling, notifications)

### Metrics Integration

The metrics system provides 17 custom Prometheus metrics covering:
- Scanner lifecycle (running state, uptime, scan counts)
- Threat detection (by type, severity, namespace)
- Performance (scan duration, process analysis rate)
- Response actions (label operations, notifications)
- System resources (memory, CPU usage)

## 🧪 Testing Notes

### Current Test Infrastructure
- No Go unit tests currently exist
- Testing relies on shell scripts for integration testing
- Local debugging uses `scripts/local-debug.sh` for environment validation
- Metrics testing uses simple HTTP curl commands

### Testing Strategies
1. **Configuration Testing**: Use different config files to test various scenarios
2. **K8s Integration**: Use local Kubernetes (Docker Desktop, minikube) for full testing
3. **Process Simulation**: Create test processes to validate detection rules
4. **Metrics Validation**: Check `/metrics` endpoint for proper data collection

## 📝 Important Implementation Details

### K8s Integration Requirements
- Service account needs `get`, `list`, `watch`, `update`, `patch` permissions on `pods` and `namespaces`
- DaemonSet requires `hostPID: true` and `hostPath` mount for `/proc`
- Metrics port 8080 must be exposed for Prometheus scraping

### Process Detection Logic
- Uses regular expressions for process name matching in blacklist/whitelist
- Keyword matching is substring-based (not regex) for command line analysis
- Namespace and pod name matching supports regex patterns
- Blacklist takes precedence over whitelist for security

### Configuration Hot-Reload
- File hashing detects content changes (not just file modifications)
- Config updates are applied without service restart
- Scanner maintains state during hot-reload transitions

### Error Handling Strategy
- K8s client failures are logged but don't stop scanning
- Notification failures are counted in metrics but don't block operations
- Configuration errors prevent startup but are recoverable via hot-reload
- Process scanning errors are counted and logged per scan cycle