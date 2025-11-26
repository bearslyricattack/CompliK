# Keyword Analyzer

A keyword frequency analysis tool for CompliK compliance detection records.

## Overview

The Keyword Analyzer connects to the CompliK MySQL database, extracts keywords from compliance detection records, performs statistical frequency analysis, and generates visual histogram charts to identify the most common compliance issues.

## Features

- **Database Integration**: Connects to MySQL with configurable connection pooling
- **Keyword Extraction**: Parses JSON arrays from detector_records table
- **Frequency Analysis**: Counts and ranks keywords by occurrence
- **Visualization**: Generates professional histogram charts with:
  - Gradient color bars
  - Chinese font support (cross-platform)
  - Rotated labels for readability
  - Count values displayed on bars
- **Cross-Platform**: Automatic font detection for Windows, Linux, and macOS

## Prerequisites

- Go 1.24 or later
- MySQL database with CompliK schema
- Access to detector_records table

## Installation

```bash
cd analyze
go mod download
```

## Configuration

Edit the DSN (Data Source Name) in `main.go`:

```go
dsn := "user:password@tcp(host:port)/database?charset=utf8mb4&parseTime=True&timeout=10s"
```

Default configuration:
- **User**: root
- **Password**: (empty)
- **Host**: 127.0.0.1:3306
- **Database**: complik
- **Charset**: utf8mb4
- **Timeout**: 10s

## Usage

### Run the Analyzer

```bash
go run main.go
```

### Output

The program generates two types of output:

1. **Console Statistics**:
   ```
   ============================================================
              Keyword Analysis Program Started
   ============================================================
   ✓ Database connection established successfully!
   Total records fetched: 1250
   Total keywords extracted: 5678 (including duplicates)

   Total unique keywords: 342

   Keyword Frequency Statistics (Top 50):
   ------------------------------------------------------------
    1. password-policy              :    456 occurrences
    2. ssl-certificate              :    389 occurrences
    3. access-control               :    312 occurrences
   ...
   ------------------------------------------------------------
   ```

2. **Histogram Chart**: `keywords_histogram.png`
   - 2400x1000 pixel resolution
   - Top N keywords (default: 50)
   - Visual frequency distribution

## Database Schema

The analyzer expects the following table structure:

```sql
CREATE TABLE detector_records (
    id INT PRIMARY KEY AUTO_INCREMENT,
    keywords JSON,  -- Array of keyword strings
    -- other fields...
);
```

Example `keywords` field:
```json
["password-policy", "ssl-certificate", "access-control"]
```

## Architecture

### Core Components

1. **KeywordAnalyzer**: Main analyzer struct
   - Database connection management
   - Connection pool configuration
   - Lifecycle management

2. **FetchKeywords**: Data extraction
   - Queries detector_records table
   - Parses JSON arrays
   - Aggregates all keywords

3. **AnalyzeKeywords**: Frequency analysis
   - Counts keyword occurrences
   - Sorts by frequency
   - Returns top N results

4. **PlotHistogram**: Visualization
   - Loads Chinese fonts
   - Generates histogram chart
   - Saves PNG image

5. **GetChineseFont**: Font detection
   - Cross-platform font paths
   - Automatic fallback

### Data Flow

```
Database (MySQL)
    ↓
FetchKeywords() → Extract JSON arrays
    ↓
AnalyzeKeywords() → Count & Sort
    ↓
PlotHistogram() → Generate Chart
    ↓
keywords_histogram.png
```

## Customization

### Change Top N Results

```go
analyzer.Run(100, "keywords_histogram.png")  // Top 100 instead of 50
```

### Modify Chart Size

Edit the chart dimensions in `PlotHistogram()`:

```go
graph := chart.Chart{
    Width:  3200,  // Increase width
    Height: 1600,  // Increase height
    // ...
}
```

### Custom Bar Colors

Modify the gradient calculation:

```go
// Current: Blue gradient
barColor := drawing.Color{R: 50, G: 100, B: intensity, A: 255}

// Example: Green gradient
barColor := drawing.Color{R: 50, G: intensity, B: 100, A: 255}
```

## Troubleshooting

### Database Connection Issues

**Error**: `failed to connect to database`
- Verify MySQL is running
- Check DSN credentials
- Ensure network connectivity
- Verify database exists

**Error**: `failed to ping database`
- Check firewall rules
- Verify MySQL user permissions
- Test connection with mysql CLI

### Font Issues

**Warning**: `no Chinese font file found in system paths`
- Install a Chinese font:
  - **Windows**: Built-in (SimHei, Microsoft YaHei)
  - **Linux**: `apt install fonts-wqy-microhei`
  - **macOS**: Built-in (PingFang)
- The chart will still generate with default fonts
- Chinese characters may appear as boxes

### Empty Results

**Warning**: `No keyword data found!`
- Check if detector_records table has data
- Verify keywords column is not empty
- Ensure JSON format is valid

## Performance

### Connection Pooling

The analyzer uses optimized connection pool settings:

```go
db.SetMaxOpenConns(10)          // Max concurrent connections
db.SetMaxIdleConns(5)           // Keep 5 connections ready
db.SetConnMaxLifetime(time.Hour) // Refresh connections hourly
```

### Memory Usage

- **Typical**: 50-100 MB for 10,000 records
- **Peak**: During histogram rendering (chart generation)
- **Scales**: O(n) with number of unique keywords

## Dependencies

```go
require (
    github.com/go-sql-driver/mysql v1.8.1      // MySQL driver
    github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0  // Font rendering
    github.com/wcharczuk/go-chart/v2 v2.1.2    // Chart generation
)
```

## License

Copyright 2025 CompliK Authors

Licensed under the Apache License, Version 2.0. See [LICENSE](../LICENSE) for details.

## Related Projects

- [CompliK](../complik/) - Main compliance monitoring platform
- [ProcScan](../procscan/) - Security scanning DaemonSet
- [Block-Controller](../block-controller/) - Namespace lifecycle manager
