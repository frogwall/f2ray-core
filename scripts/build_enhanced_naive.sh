#!/bin/bash

# Build script for enhanced naive with uTLS Chrome fingerprints

set -e

echo "Building f2ray with enhanced naive (uTLS + Chrome simulation)..."

# Build with enhanced tag to enable uTLS Chrome fingerprints
go build -tags enhanced -o f2ray-enhanced ./main

echo "✅ Enhanced f2ray built successfully!"
echo ""
echo "Features enabled:"
echo "  - uTLS Chrome 120 fingerprint simulation"
echo "  - Chrome-like HTTP headers"
echo "  - Human-like random delays"
echo "  - Optimized HTTP/2 behavior"
echo ""
echo "Usage:"
echo "  ./f2ray-enhanced run -c your-config.json"
echo ""
echo "For comparison, build standard version:"
echo "  go build -o f2ray-standard ./main"
echo ""
echo "Note: Enhanced version requires additional dependencies but provides"
echo "      better traffic camouflage against DPI detection."
