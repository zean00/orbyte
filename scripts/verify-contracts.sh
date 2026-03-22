#!/usr/bin/env bash
set -euo pipefail

go test ./contracts ./internal/platform/integration
