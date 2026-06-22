// Package embed carries the MoonBit proved kernel WASM component compiled into
// the aid binary (D4: single static binary). The artifact is produced by
// `make wasm` from kernel/wasm; it is committed so `go build`/`go test` work
// without the moon toolchain.
//
// The committed file may be an empty-module placeholder before `make wasm` runs.
// Use Stale() / the `make embed-check` target (issue #33) to detect that.
//
// (F7d retired the Rust hhfab/bom adapters; only the kernel remains — #64/#35.)
package embed

import _ "embed"

//go:embed kernel.wasm
var Kernel []byte

// minLen is the size of a real component (the placeholder is 8 bytes — just the
// wasm header). A built component is far larger.
const minLen = 1024

// Stale reports whether the embedded kernel is still a placeholder (i.e.
// `make wasm` has not produced a real component). Used by tests / embed-check.
func Stale() bool {
	return len(Kernel) < minLen
}
