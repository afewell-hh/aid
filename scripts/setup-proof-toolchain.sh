#!/usr/bin/env bash
# setup-proof-toolchain.sh — idempotent install of the `moon prove` proof
# backend: Why3 1.7.2 (via opam, OCaml 4.14.2) + Z3 4.8.12 (apt).
#
# One path shared by a clean developer machine and CI (Issue #21). Versions are
# pinned to DEVELOPMENT.md. Re-runnable: on a machine/cache that already has the
# `why3env` opam switch + Why3 1.7.2, the expensive OCaml/Why3 build is skipped
# and only PATH export + `why3 config detect` run.
#
# Usage:
#   scripts/setup-proof-toolchain.sh          # install (or no-op if present)
#   scripts/setup-proof-toolchain.sh --check  # verify only, no install
#
# After a successful run, `why3` and `z3` resolve and `moon prove` works with no
# manual `eval $(opam env)`:
#   - in CI, the opam switch bin dir is appended to $GITHUB_PATH;
#   - locally, it is appended to ~/.profile (guarded against duplicates).
set -euo pipefail

# --- Pinned versions (DEVELOPMENT.md) ---------------------------------------
OCAML_VERSION="4.14.2"
WHY3_VERSION="1.7.2"
Z3_VERSION="4.8.12"          # apt default on ubuntu-22.04; soft-checked
OPAM_SWITCH="why3env"

log()  { printf '\033[1;34m[proof-toolchain]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[proof-toolchain] WARN:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[proof-toolchain] ERROR:\033[0m %s\n' "$*" >&2; exit 1; }

SUDO=""
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo"; fi

CHECK_ONLY=0
[ "${1:-}" = "--check" ] && CHECK_ONLY=1

# --- opam switch bin dir (where why3 lands) ---------------------------------
switch_bin() {
  # Resolve without needing the env loaded; opam var is authoritative once opam exists.
  if command -v opam >/dev/null 2>&1; then
    opam var --switch="$OPAM_SWITCH" bin 2>/dev/null || echo "$HOME/.opam/$OPAM_SWITCH/bin"
  else
    echo "$HOME/.opam/$OPAM_SWITCH/bin"
  fi
}

# Make why3 resolvable in THIS process for config-detect + verification.
load_env() {
  if command -v opam >/dev/null 2>&1 && opam switch list --short 2>/dev/null | grep -qx "$OPAM_SWITCH"; then
    eval "$(opam env --switch="$OPAM_SWITCH" 2>/dev/null)" || true
  fi
  local bin
  bin="$(switch_bin)"
  export PATH="$bin:$PATH"
}

verify() {
  load_env
  command -v why3 >/dev/null 2>&1 || die "why3 not on PATH after setup (expected in $(switch_bin))"
  command -v z3   >/dev/null 2>&1 || die "z3 not on PATH after setup"
  local why3_v z3_v
  why3_v="$(why3 --version 2>&1 | head -1)"
  z3_v="$(z3 --version 2>&1 | head -1)"
  log "why3: $why3_v"
  log "z3:   $z3_v"
  echo "$why3_v" | grep -q "$WHY3_VERSION" || die "Why3 version mismatch: want $WHY3_VERSION, got: $why3_v"
  echo "$z3_v"   | grep -q "$Z3_VERSION"   || warn "Z3 version is not the pinned $Z3_VERSION (got: $z3_v) — runner image may have drifted; re-validate."
  log "proof toolchain OK (why3 $WHY3_VERSION, z3 present)"
}

if [ "$CHECK_ONLY" -eq 1 ]; then
  verify
  exit 0
fi

# --- 1. apt: Z3 + opam build deps (idempotent — apt skips installed) --------
if ! command -v z3 >/dev/null 2>&1 || ! command -v opam >/dev/null 2>&1; then
  log "installing apt packages: z3 opam libgmp-dev pkg-config m4"
  $SUDO apt-get update -qq
  $SUDO apt-get install -y -qq z3 opam libgmp-dev pkg-config m4
else
  log "z3 + opam already present (skipping apt)"
fi

# --- 2. opam init (skip if already initialized / cache-restored) ------------
if [ ! -d "$HOME/.opam" ] || ! opam var root >/dev/null 2>&1; then
  log "opam init (--disable-sandboxing: runner bubblewrap may be unavailable)"
  opam init --disable-sandboxing -y --bare
else
  log "opam already initialized (skipping init)"
fi

# --- 3. dedicated OCaml switch for Why3 (skip if it exists) ------------------
if ! opam switch list --short 2>/dev/null | grep -qx "$OPAM_SWITCH"; then
  log "creating opam switch $OPAM_SWITCH (ocaml-base-compiler.$OCAML_VERSION) — builds OCaml, slow on a cold cache"
  opam switch create "$OPAM_SWITCH" "ocaml-base-compiler.$OCAML_VERSION"
else
  log "opam switch $OPAM_SWITCH already exists (skipping — this is the cache-hit fast path)"
fi

eval "$(opam env --switch="$OPAM_SWITCH")"

# --- 4. Why3 1.7.2 (skip if already the pinned version in the switch) -------
if ! why3 --version 2>/dev/null | grep -q "$WHY3_VERSION"; then
  log "installing why3.$WHY3_VERSION"
  opam install "why3.$WHY3_VERSION" -y
else
  log "why3 $WHY3_VERSION already installed (skipping)"
fi

# --- 5. detect Z3 → ~/.why3.conf (always; cheap; not cached: absolute paths) -
log "why3 config detect (writes ~/.why3.conf pointing at the apt Z3)"
why3 config detect

# --- 6. persist PATH for later steps / future shells ------------------------
BIN_DIR="$(switch_bin)"
if [ -n "${GITHUB_PATH:-}" ]; then
  log "appending $BIN_DIR to \$GITHUB_PATH (CI)"
  echo "$BIN_DIR" >> "$GITHUB_PATH"
else
  PROFILE="$HOME/.profile"
  LINE="export PATH=\"$BIN_DIR:\$PATH\""
  if ! grep -qsF "$BIN_DIR" "$PROFILE" 2>/dev/null; then
    log "appending opam switch bin to $PROFILE (so login shells resolve why3)"
    printf '\n# Why3 proof toolchain (AID Issue #21 / #5)\n%s\n' "$LINE" >> "$PROFILE"
  else
    log "$PROFILE already references $BIN_DIR (skipping)"
  fi
fi

# --- 7. verify ---------------------------------------------------------------
verify
