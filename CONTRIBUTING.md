# Contributing to CompliK

Thank you for your interest in contributing to CompliK! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Contribution Workflow](#contribution-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Pull Request Process](#pull-request-process)
- [Issue Reporting](#issue-reporting)

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment. We expect all contributors to:

- Use welcoming and inclusive language
- Be respectful of differing viewpoints and experiences
- Accept constructive criticism gracefully
- Focus on what is best for the community
- Show empathy towards other community members

## Getting Started

### Prerequisites

- **Go**: 1.24 or later
- **Kubernetes**: 1.19+ for testing
- **Docker**: For building container images
- **Git**: For version control
- **Make**: For using the build system

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/CompliK.git
   cd CompliK
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/bearslyricattack/CompliK.git
   ```

## Development Setup

### Install Dependencies

```bash
# Install all project dependencies
make tidy-all

# Or for a specific component
cd complik && go mod tidy
cd block-controller && go mod tidy
cd procscan && go mod tidy
cd analyze && go mod tidy
```

### Build the Projects

```bash
# Build all components
make build-all

# Or build individual components
make build-complik
make build-block-controller
make build-procscan
make build-analyze
```

### Run Tests

```bash
# Run all tests
make test-all

# Or test individual components
make test-complik
make test-block-controller
make test-procscan
```

## Project Structure

CompliK uses a Monorepo architecture with four independent components:

- **`complik/`** - Compliance monitoring platform
- **`block-controller/`** - Namespace lifecycle manager
- **`procscan/`** - Security scanning DaemonSet
- **`analyze/`** - Keyword analysis tool

Each component has its own:
- `go.mod` - Independent Go module
- `README.md` - Component-specific documentation
- `Dockerfile` - Container image definition
- `deploy/` - Kubernetes manifests

## Contribution Workflow

### 1. Create a Branch

Create a feature branch from `main`:

```bash
git checkout -b feature/my-new-feature
# or
git checkout -b fix/issue-123
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation updates
- `refactor/` - Code refactoring
- `test/` - Test additions or improvements

### 2. Make Changes

- Write clean, readable code
- Follow the project's coding standards
- Add tests for new functionality
- Update documentation as needed

### 3. Commit Changes

Write clear, descriptive commit messages:

```bash
git add .
git commit -m "component: brief description

Detailed explanation of what changed and why.

Fixes #123"
```

Commit message format:
```
<component>: <short summary>

<detailed description>

<footer>
```

Examples:
- `complik: add support for custom compliance rules`
- `block-controller: fix namespace cleanup race condition`
- `procscan: improve memory usage in scanner`
- `docs: update installation instructions`

### 4. Keep Your Branch Updated

```bash
git fetch upstream
git rebase upstream/main
```

### 5. Push and Create Pull Request

```bash
git push origin feature/my-new-feature
```

Then create a Pull Request on GitHub.

## Coding Standards

### Go Code Style

Follow standard Go conventions:

- Use `gofmt` for formatting (run `make fmt-all`)
- Use `golangci-lint` for linting (run `make lint-all`)
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use meaningful variable and function names
- Keep functions small and focused

### Code Organization

```go
// Copyright 2025 CompliK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// ...

// Package comment describing the package
package mypackage

import (
    // Standard library
    "context"
    "fmt"

    // External dependencies
    "k8s.io/client-go/kubernetes"

    // Internal packages
    "github.com/bearslyricattack/CompliK/complik/pkg/models"
)

// Type definitions with comments
type MyStruct struct {
    Field1 string // Field description
    Field2 int    // Field description
}

// Function comments describe what the function does
func MyFunction(param string) error {
    // Implementation
}
```

### Comments and Documentation

- Add package-level comments for all packages
- Document all exported types, functions, and constants
- Use complete sentences in comments
- Write comments in English
- Include examples for complex functionality

### Error Handling

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to process request: %w", err)
}

// Good: Check errors immediately
result, err := someFunction()
if err != nil {
    return err
}

// Bad: Ignoring errors
someFunction() // Don't do this
```

## Testing Guidelines

### Writing Tests

- Place tests in `*_test.go` files
- Use table-driven tests when appropriate
- Test both success and failure cases
- Use meaningful test names

Example:
```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:    "valid input",
            input:   "test",
            want:    "TEST",
            wantErr: false,
        },
        {
            name:    "empty input",
            input:   "",
            want:    "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("MyFunction() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("MyFunction() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Test Coverage

- Aim for at least 70% code coverage for new code
- Critical paths should have 90%+ coverage
- Run tests with coverage: `go test -cover ./...`

## Documentation

### What to Document

- Public APIs and interfaces
- Configuration options
- Deployment procedures
- Troubleshooting guides
- Architecture decisions

### Documentation Format

- Use Markdown for documentation files
- Include code examples where appropriate
- Add diagrams for complex workflows (use Mermaid)
- Keep documentation up-to-date with code changes

### Updating Documentation

When making changes, update:
- Component README if functionality changes
- Package comments in code
- Architecture docs if design changes
- Configuration examples if new options added

## Pull Request Process

### Before Submitting

- [ ] Code follows project style guidelines
- [ ] Tests pass locally (`make test-all`)
- [ ] Code is formatted (`make fmt-all`)
- [ ] Linter passes (`make lint-all`)
- [ ] Documentation is updated
- [ ] Commit messages are clear and descriptive
- [ ] Branch is rebased on latest `main`

### PR Description Template

```markdown
## Description
Brief description of what this PR does

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Related Issues
Fixes #123
Related to #456

## Testing
Describe how you tested these changes

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Changelog updated (if applicable)
- [ ] All tests passing
```

### Review Process

1. Automated checks must pass (CI/CD)
2. At least one maintainer approval required
3. All review comments must be addressed
4. Branch must be up-to-date with `main`

### After Approval

Once approved, a maintainer will merge your PR. Your contribution will be included in the next release!

## Issue Reporting

### Before Creating an Issue

- Search existing issues to avoid duplicates
- Gather relevant information (logs, config, versions)
- Create a minimal reproducible example

### Issue Template

```markdown
**Component**: [complik/block-controller/procscan/analyze]

**Description**
Clear description of the issue

**Steps to Reproduce**
1. Step 1
2. Step 2
3. Step 3

**Expected Behavior**
What should happen

**Actual Behavior**
What actually happens

**Environment**
- Kubernetes version:
- Go version:
- CompliK version:
- OS:

**Logs**
```
Relevant logs here
```

**Additional Context**
Any other relevant information
```

### Issue Labels

Use appropriate labels:
- `bug` - Something isn't working
- `enhancement` - New feature request
- `documentation` - Documentation improvement
- `good first issue` - Good for newcomers
- `help wanted` - Extra attention needed
- Component labels: `complik`, `block-controller`, `procscan`, `analyze`

## Development Tips

### Debugging

```bash
# Run with verbose logging
VERBOSE=true ./bin/complik

# Run with debug logging
LOG_LEVEL=debug ./bin/procscan

# Enable Go race detector
go test -race ./...
```

### Local Kubernetes Testing

```bash
# Use kind for local testing
kind create cluster --name complik-test

# Deploy your changes
kubectl apply -f deploy/

# Check logs
kubectl logs -f deployment/complik
```

### Useful Commands

```bash
# Format all code
make fmt-all

# Run linters
make lint-all

# Clean build artifacts
make clean-all

# Build Docker images
make docker-build-all

# Run specific component tests
cd complik && go test -v ./...
```

## Getting Help

- **Questions**: [GitHub Discussions](https://github.com/bearslyricattack/CompliK/discussions)
- **Bug Reports**: [GitHub Issues](https://github.com/bearslyricattack/CompliK/issues)
- **Documentation**: See component README files

## License

By contributing to CompliK, you agree that your contributions will be licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.

---

Thank you for contributing to CompliK! 🎉
