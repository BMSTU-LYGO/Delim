#!/usr/bin/env bash
set -euo pipefail

make proto
make document-proto

paths=(
  pkg/gen/core/v1
  pkg/gen/document/v1
  document/gen/proto/document/v1
)

if git diff --exit-code -- "${paths[@]}"; then
  echo "generated-check: ok"
else
  echo "generated-check: generated protobuf has uncommitted changes" >&2
  exit 1
fi
