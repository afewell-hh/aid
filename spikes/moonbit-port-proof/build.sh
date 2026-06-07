#!/usr/bin/env bash
# Rebuild the MoonBit core-wasm artifact consumed by the Go harness.
#
# The committed go/testdata/alloc.wasm (147 bytes) is produced by this script.
# It is checked in so `go test ./...` runs without the MoonBit/Why3 toolchain;
# re-run this script after changing src/ to regenerate it.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$here"

# Requires `moon` on PATH (see DEVELOPMENT.md).
moon build --target wasm --release

src_wasm="$here/_build/wasm/release/build/src/src.wasm"
dst_wasm="$here/go/testdata/alloc.wasm"
cp "$src_wasm" "$dst_wasm"
echo "wrote $dst_wasm ($(wc -c < "$dst_wasm") bytes)"
