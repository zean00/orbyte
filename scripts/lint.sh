#!/usr/bin/env bash
set -euo pipefail

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "golangci-lint is required. Install from https://golangci-lint.run/usage/install/" >&2
  exit 1
fi

golangci-lint run ./...
