# AID build — produces the three WASM components and the single static `aid`
# binary (D4). The Makefile is the source of truth for embed/*.wasm.
#
#   make wasm    build kernel.wasm (MoonBit) + hhfab.wasm + bom.wasm (Rust),
#                copy into embed/
#   make build   wasm + go build -o aid ./cmd/aid
#   make test    go test ./...
#   make embed-check  fail if any embed/*.wasm is still a placeholder (#33)

SHELL := /bin/bash
MOON  ?= $(HOME)/.moon/bin/moon
EMBED := embed
WASM_TARGET := wasm32-unknown-unknown

.PHONY: wasm kernel-wasm hhfab-wasm bom-wasm build test embed-check clean

wasm: kernel-wasm hhfab-wasm bom-wasm
	@echo "embedded components:" && ls -l $(EMBED)/*.wasm

kernel-wasm:
	cd kernel/wasm && $(MOON) build --target wasm --release
	cp "$$(find kernel/_build/wasm/release/build/wasm -name '*.wasm' | head -1)" $(EMBED)/kernel.wasm

hhfab-wasm:
	cd hhfab-adapter && cargo build --release --target $(WASM_TARGET)
	cp "$$(find hhfab-adapter/target/$(WASM_TARGET)/release -maxdepth 1 -name '*.wasm' | head -1)" $(EMBED)/hhfab.wasm

bom-wasm:
	cd bom-adapter && cargo build --release --target $(WASM_TARGET)
	cp "$$(find bom-adapter/target/$(WASM_TARGET)/release -maxdepth 1 -name '*.wasm' | head -1)" $(EMBED)/bom.wasm

build: wasm
	go build -o aid ./cmd/aid

test:
	go test ./...

# Stale-embed guard (#33): the placeholders are 8-byte wasm headers; a real
# component is far larger. Run in CI before tests to catch an un-rebuilt embed.
embed-check:
	@fail=0; for f in $(EMBED)/kernel.wasm $(EMBED)/hhfab.wasm $(EMBED)/bom.wasm; do \
		sz=$$(wc -c < "$$f"); \
		if [ "$$sz" -lt 1024 ]; then echo "STALE: $$f is $$sz bytes (placeholder) — run 'make wasm'"; fail=1; fi; \
	done; \
	if [ "$$fail" -ne 0 ]; then exit 1; fi; \
	echo "embed OK (all components built)"

clean:
	rm -f aid
