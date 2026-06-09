// Package embed carries the three WASM components compiled into the aid binary
// (D4: single static binary). The artifacts are produced by `make wasm` from
// kernel/wasm (MoonBit) and the two Rust adapters; they are committed so
// `go build`/`go test` work without the moon/rust toolchains.
//
// The committed files may be empty-module placeholders before `make wasm` runs.
// Use Stale() / the `make embed-check` target (issue #33) to detect that.
package embed

import _ "embed"

//go:embed kernel.wasm
var Kernel []byte

//go:embed hhfab.wasm
var Hhfab []byte

//go:embed bom.wasm
var Bom []byte

// minLen is the size of a real component (the placeholders are 8 bytes — just
// the wasm header). A built component is far larger.
const minLen = 1024

// Stale reports whether any embedded artifact is still a placeholder (i.e.
// `make wasm` has not produced real components). Used by tests / embed-check.
func Stale() bool {
	return len(Kernel) < minLen || len(Hhfab) < minLen || len(Bom) < minLen
}
