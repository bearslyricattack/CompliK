# CompliK Logging System Documentation

## Overview

CompliK uses a unified structured logging system that supports multiple log levels, performance monitoring, log rotation, and other features.

## Log Levels

The system supports the following log levels (from low to high):

- **DEBUG**: Detailed debugging information
- **INFO**: General information
- **WARN**: Warning information
- **ERROR**: Error information
- **FATAL**: Fatal errors (will cause program exit)

## Environment Variable Configuration

Log behavior can be flexibly configured through environment variables:

```bash
# Log level (DEBUG, INFO, WARN, ERROR, FATAL)
export COMPLIK_LOG_LEVEL=INFO

# Log format (text or json)
export COMPLIK_LOG_FORMAT=text

# Whether to display colors (true or false)
export COMPLIK_LOG_COLORED=true

# Whether to display caller location (true or false)
export COMPLIK_LOG_CALLER=true

# Log file path (outputs to console if not set)
export COMPLIK_LOG_FILE=/var/log/complik/app.log

# Maximum log file size (bytes)
export COMPLIK_LOG_MAX_SIZE=104857600  # 100MB

# Number of backup files to retain
export COMPLIK_LOG_MAX_BACKUPS=10

# Log file retention days
export COMPLIK_LOG_MAX_AGE=30
```

## Usage Examples

### Basic Usage

```go
import "github.com/bearslyricattack/CompliK/pkg/logger"

// Initialize the logging system
logger.Init()

// Get logger instance
log := logger.GetLogger()

// Log messages at different levels
log.Debug("This is a debug message")
log.Info("Application started")
log.Warn("Low memory warning")
log.Error("Failed to connect to database")
log.Fatal("Critical error, shutting down")
```

### Structured Logging

```go
// Add fields
log.Info("User login", logger.Fields{
    "user_id": 12345,
    "ip": "192.168.1.1",
    "method": "OAuth",
})

// Chain calls
log.WithField("request_id", "abc-123").
    WithField("user", "john").
    Info("Processing request")

// Add error information
err := doSomething()
if err != nil {
    log.WithError(err).Error("Operation failed")
}
```

### Context Logging

```go
// Create logger with context
ctx := context.WithValue(context.Background(), "request_id", "xyz-789")
contextLog := log.WithContext(ctx)

// Automatically includes context information
contextLog.Info("Processing request")
// Output: 2024-01-01 12:00:00 [INFO] Processing request | request_id=xyz-789
```

### Performance Tracing

```go
// Trace operation execution time
err := logger.TraceOperation(ctx, "database_query", func() error {
    // Execute database query
    return db.Query()
})

// Function tracing
func ProcessData() {
    defer logger.TraceFunc("ProcessData")()
    // Function logic
}
```

### Performance Monitoring

```go
// Initialize performance monitoring (report every minute)
logger.InitMetrics(1 * time.Minute)

// Stop monitoring when program exits
defer logger.StopMetrics()
```

## Log Output Formats

### Text Format (Default)

```
2024-01-01 12:00:00.123 [INFO ] [main.go:42] Application started | version=1.0.0, env=production
2024-01-01 12:00:01.456 [ERROR] [db.go:156] Database connection failed | error=connection timeout, retry=3
```

### JSON Format

```json
{
  "time": "2024-01-01 12:00:00.123",
  "level": "INFO",
  "caller": "main.go:42",
  "func": "main",
  "msg": "Application started",
  "version": "1.0.0",
  "env": "production"
}
```

## Log Rotation

When log file output is configured, the system automatically performs log rotation:

1. **Rotation by Size**: When the log file exceeds the specified size
2. **Rotation by Time**: Automatic daily rotation
3. **Automatic Cleanup**: Delete old log files exceeding the retention period

Rotated file naming format:
```
app.log                    # Current log file
app-20240101-120000.log    # Rotated historical files
app-20240101-000000.log
```

## Performance Metrics

The system automatically collects and logs the following performance metrics:

### System Metrics
- Memory usage
- Goroutine count
- GC pause time
- Uptime

### Operation Metrics
- Operation execution count
- Average execution time
- Minimum/maximum execution time
- Error rate
- Success rate

Example output:
```
2024-01-01 12:00:00 [INFO] System metrics | memory_mb=45, goroutines=25, gc_pause_ms=2, uptime_minutes=60
2024-01-01 12:00:00 [INFO] Operation metrics | operation=db_query, count=1000, avg_ms=5, success_rate=99.5%
```

## Best Practices

### 1. Log Level Usage Guidelines

- **DEBUG**: Use only during development and debugging
- **INFO**: Log normal business flow
- **WARN**: Log potential issues that don't affect functionality
- **ERROR**: Log errors but the program can continue running
- **FATAL**: Use only when the program must be terminated

### 2. Structured Logging

Always use structured fields instead of string concatenation:

```go
// ✅ Recommended
log.Info("User action", logger.Fields{
    "user_id": userID,
    "action": "login",
})

// ❌ Not recommended
log.Info(fmt.Sprintf("User %d performed login", userID))
```

### 3. Error Handling

Always log the complete context of errors:

```go
if err := service.Process(); err != nil {
    log.WithError(err).
        WithField("service", "payment").
        WithField("transaction_id", txID).
        Error("Service processing failed")
    return err
}
```

### 4. Performance Considerations

- Set appropriate log levels in production environments (usually INFO)
- Use asynchronous log writing to avoid blocking the main flow
- Regularly clean up old log files
- Consider using JSON format for easier processing by log analysis tools

### 5. Security Considerations

- Do not log sensitive information (passwords, keys, personal information, etc.)
- Use field filtering for sensitive data
- Ensure log files have appropriate access permissions

## Integration Examples

### Docker Configuration

```dockerfile
ENV COMPLIK_LOG_LEVEL=INFO
ENV COMPLIK_LOG_FORMAT=json
ENV COMPLIK_LOG_FILE=/var/log/complik/app.log
ENV COMPLIK_LOG_MAX_SIZE=104857600
ENV COMPLIK_LOG_MAX_BACKUPS=10
ENV COMPLIK_LOG_MAX_AGE=30
```

### Kubernetes ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: complik-logging
data:
  COMPLIK_LOG_LEVEL: "INFO"
  COMPLIK_LOG_FORMAT: "json"
  COMPLIK_LOG_COLORED: "false"
  COMPLIK_LOG_CALLER: "true"
```

### systemd Service

```ini
[Service]
Environment="COMPLIK_LOG_LEVEL=INFO"
Environment="COMPLIK_LOG_FILE=/var/log/complik/app.log"
Environment="COMPLIK_LOG_MAX_SIZE=104857600"
```

## Troubleshooting

### Logs Not Outputting

1. Check if the log level is set correctly
2. Verify that the log file path has write permissions
3. Validate that environment variables are set correctly

### Performance Issues

1. Lower the log level (e.g., from DEBUG to INFO)
2. Use asynchronous log writing
3. Increase log file rotation frequency
4. Consider using a dedicated log collection service

### Log Files Too Large

1. Configure a reasonable rotation strategy
2. Reduce unnecessary DEBUG logs
3. Regularly archive historical logs