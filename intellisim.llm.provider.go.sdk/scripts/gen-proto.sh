#!/usr/bin/env bash
# Regenerates proto/llmbridge/v1/*.pb.go. See gen-proto.ps1 header for prerequisites.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PROTOC="${PROTOC:-$(command -v protoc || true)}"
if [[ -z "$PROTOC" ]]; then
  echo "protoc not found on PATH and \$PROTOC not set." >&2
  echo "Install from https://github.com/protocolbuffers/protobuf/releases (v29.3)." >&2
  exit 1
fi
"$PROTOC" --version
"$PROTOC" \
  --proto_path="$ROOT/proto" \
  --go_out="$ROOT/proto" --go_opt=paths=source_relative \
  --go-grpc_out="$ROOT/proto" --go-grpc_opt=paths=source_relative \
  llmbridge/v1/llmbridge.proto
echo "OK: generated proto/llmbridge/v1/*.pb.go"
