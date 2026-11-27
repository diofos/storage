#!/usr/bin/env bash
# DIOFOS Zero-Install Launcher
# Automatically selects and executes the pre-compiled binary for your architecture.
# No Go, Make, or developer tooling required.

set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

BINARY=""

if [ "$OS" = "darwin" ]; then
    if [ "$ARCH" = "arm64" ]; then
        BINARY="$DIR/bin/dist/diofos-darwin-arm64"
    else
        BINARY="$DIR/bin/dist/diofos-darwin-amd64"
    fi
elif [ "$OS" = "linux" ]; then
    if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
        BINARY="$DIR/bin/dist/diofos-linux-arm64"
    else
        BINARY="$DIR/bin/dist/diofos-linux-amd64"
    fi
fi

if [ -z "$BINARY" ] || [ ! -f "$BINARY" ]; then
    # Fallback to bin/diofos if present
    if [ -f "$DIR/bin/diofos" ]; then
        BINARY="$DIR/bin/diofos"
    else
        echo "Error: Pre-compiled binary not found for OS=$OS ARCH=$ARCH"
        echo "Please download the binary directly from the DIOFOS web UI."
        exit 1
    fi
fi

chmod +x "$BINARY"
exec "$BINARY" "$@"
