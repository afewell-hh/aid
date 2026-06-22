# AID build — produces the proved kernel WASM component and the single static
# `aid` binary (D4). The Makefile is the source of truth for embed/kernel.wasm.
# (F7d retired the Rust hhfab/bom adapters — #64/#35.)
#
#   make wasm    build kernel.wasm (MoonBit), copy into embed/
#   make build   wasm + go build -o aid ./cmd/aid
#   make test    go test ./...
#   make embed-check  fail if embed/kernel.wasm is still a placeholder (#33)

SHELL := /bin/bash
MOON  ?= $(HOME)/.moon/bin/moon
EMBED := embed

UI_SRC := ui/src
UI_STATIC := ui/static
UI_BUNDLE := $(UI_STATIC)/app.js

.PHONY: wasm kernel-wasm build test embed-check clean ui ui-check ui-test

wasm: kernel-wasm
	@echo "embedded components:" && ls -l $(EMBED)/*.wasm

kernel-wasm:
	cd kernel/wasm && $(MOON) build --target wasm --release
	cp "$$(find kernel/_build/wasm/release/build/wasm -name '*.wasm' | head -1)" $(EMBED)/kernel.wasm

build: wasm
	go build -o aid ./cmd/aid

test:
	go test ./...

# Stale-embed guard (#33): the placeholder is an 8-byte wasm header; a real
# component is far larger. Run in CI before tests to catch an un-rebuilt embed.
embed-check:
	@fail=0; for f in $(EMBED)/kernel.wasm; do \
		sz=$$(wc -c < "$$f"); \
		if [ "$$sz" -lt 1024 ]; then echo "STALE: $$f is $$sz bytes (placeholder) — run 'make wasm'"; fail=1; fi; \
	done; \
	if [ "$$fail" -ne 0 ]; then exit 1; fi; \
	echo "embed OK (kernel built)"

# --- Web frontend (Phase 6b Stage B): MoonBit -> JS, embedded under ui/static ---

# Build the MoonBit->JS bundle and copy it to ui/static/app.js (the committed
# artifact go:embed compiles in, like embed/*.wasm).
ui:
	cd ui && $(MOON) build --target js --release
	cp "$$(find ui/_build/js/release/build/src -name 'src.js' | head -1)" $(UI_BUNDLE)
	@echo "wrote $(UI_BUNDLE) ($$(wc -c < $(UI_BUNDLE)) bytes)"

# Run the Node ESM smoke tests against the committed bundle (no npm deps;
# uses node:test + node:assert). Drives the real render/wire functions with a
# stubbed document/fetch.
ui-test:
	node --test ui/test/*.test.mjs

# Freshness guard (#33-style, the app.js analogue of embed-check): rebuild the
# bundle to a temp file and fail if it differs from the committed ui/static/app.js
# (or if the committed bundle is a placeholder). Run in CI before ui-test.
ui-check:
	@sz=$$(wc -c < $(UI_BUNDLE) 2>/dev/null || echo 0); \
	if [ "$$sz" -lt 512 ]; then echo "STALE: $(UI_BUNDLE) is $$sz bytes (placeholder) — run 'make ui'"; exit 1; fi
	@cd ui && $(MOON) build --target js --release >/dev/null
	@fresh="$$(find ui/_build/js/release/build/src -name 'src.js' | head -1)"; \
	if ! diff -q "$$fresh" $(UI_BUNDLE) >/dev/null; then \
		echo "STALE: $(UI_BUNDLE) differs from a fresh 'moon build' — run 'make ui' and commit"; exit 1; \
	fi; \
	echo "ui bundle OK (app.js matches ui/src)"

clean:
	rm -f aid
