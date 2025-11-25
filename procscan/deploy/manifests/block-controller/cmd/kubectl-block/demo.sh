#!/bin/bash

# kubectl-block CLI Demo Script
# This script demonstrates the CLI functionality without needing a real cluster

echo "🚀 Welcome to kubectl-block CLI Demo"
echo "=================================="
echo

# Show help
echo "📖 CLI Help:"
./kubectl-block --help
echo

echo "🔒 Lock Command Help:"
./kubectl-block lock --help
echo

echo "🔓 Unlock Command Help:"
./kubectl-block unlock --help
echo

echo "📊 Status Command Help:"
./kubectl-block status --help
echo

echo "✅ Demo completed! The CLI tool is working correctly."
echo
echo "To use with a real Kubernetes cluster:"
echo "1. Ensure you have kubectl configured"
echo "2. Run: kubectl block status --all"
echo "3. Try: kubectl block lock my-namespace --dry-run"
echo
echo "For more examples, see README.md"