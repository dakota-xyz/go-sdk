#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GEN_DIR="$ROOT_DIR/client/gen"

cd "$GEN_DIR"

go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 \
  -config oapi-codegen.yaml \
  -o client.gen.go \
  openapi.yaml

gofmt -w client.gen.go
