#!/bin/bash

set -e

echo "Checking dependencies for SmartHeart..."

# Check Go
if command -v go &> /dev/null; then
    GO_VERSION=$(go version)
    echo "Go installed: $GO_VERSION"
else
    echo "Go is not installed. Please install Go 1.24+ from https://go.dev/dl/"
    exit 1
fi

echo ""
echo "All dependencies are installed!"
