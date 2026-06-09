#!/usr/bin/env bash
# moon-prove-gate.sh — the formal-proof build gate (Issue #21, DECISIONS.md D2).
#
# Runs `moon prove` in each given package dir and FAILS the build on any
# unproved goal. `moon prove` ALWAYS exits 0 (even when goals fail), so this
# NEVER trusts $? — it parses stdout for the `N of M packages proved` summary
# and fails unless every package proves all its goals.
#
# Usage:
#   scripts/moon-prove-gate.sh <pkg-dir> [<pkg-dir> ...]
#   e.g. scripts/moon-prove-gate.sh spikes/moonbit-port-proof
# Phase 8 wires the kernel by appending its proof package dir(s) — no edits here.
#
# Fails if, for any package: N != M, or M == 0 (nothing proved — catches a
# missing `"proof-enabled": true` or a no-goal package masquerading as success),
# or the output shows `Failed`/`timeout`, or no `Summary:` is produced at all
# (e.g. why3 not on PATH).
set -euo pipefail

log()  { printf '\033[1;34m[prove-gate]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[prove-gate] FAIL:\033[0m %s\n' "$*" >&2; }
ok()   { printf '\033[1;32m[prove-gate] OK:\033[0m %s\n' "$*"; }

[ "$#" -ge 1 ] || { echo "usage: $0 <pkg-dir> [<pkg-dir> ...]" >&2; exit 2; }

# Resolve moon (Makefile convention: ~/.moon/bin/moon).
MOON="${MOON:-}"
if [ -z "$MOON" ]; then
  if command -v moon >/dev/null 2>&1; then MOON="moon"
  elif [ -x "$HOME/.moon/bin/moon" ]; then MOON="$HOME/.moon/bin/moon"
  else fail "moon not found (set \$MOON or add ~/.moon/bin to PATH)"; exit 1
  fi
fi
command -v why3 >/dev/null 2>&1 || {
  fail "why3 not on PATH — 'moon prove' needs Why3 ${WHY3_HINT:-1.7.2} (run scripts/setup-proof-toolchain.sh)"
  exit 1
}

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
overall_fail=0

for pkg in "$@"; do
  dir="$pkg"
  [ -d "$dir" ] || dir="$REPO_ROOT/$pkg"
  [ -d "$dir" ] || { fail "package dir not found: $pkg"; overall_fail=1; continue; }

  log "proving: $pkg"
  out="$( cd "$dir" && "$MOON" prove 2>&1 )" || true   # exit code is unreliable — ignore it
  printf '%s\n' "$out" | sed 's/^/    /'

  # Authoritative line: "N of M packages proved".
  summary="$(printf '%s\n' "$out" | grep -E '[0-9]+ of [0-9]+ packages proved' | tail -1 || true)"
  if [ -z "$summary" ]; then
    fail "$pkg: no 'N of M packages proved' summary (moon prove errored or proved nothing — is why3 configured? is \"proof-enabled\": true set?)"
    overall_fail=1
    continue
  fi

  if [[ "$summary" =~ ([0-9]+)\ of\ ([0-9]+)\ packages\ proved ]]; then
    proved="${BASH_REMATCH[1]}"
    total="${BASH_REMATCH[2]}"
  else
    fail "$pkg: could not parse summary line: $summary"
    overall_fail=1
    continue
  fi

  if printf '%s\n' "$out" | grep -Eq 'Failed|timeout|Failed goals:'; then
    fail "$pkg: output reports Failed/timeout goals"
    overall_fail=1
    continue
  fi
  if [ "$total" -eq 0 ]; then
    fail "$pkg: 0 packages had goals to prove (missing \"proof-enabled\": true?)"
    overall_fail=1
    continue
  fi
  if [ "$proved" -ne "$total" ]; then
    fail "$pkg: only $proved of $total packages proved"
    overall_fail=1
    continue
  fi
  ok "$pkg: $proved of $total packages proved"
done

if [ "$overall_fail" -ne 0 ]; then
  fail "one or more packages have unproved goals — build blocked (D2)"
  exit 1
fi
ok "all packages proved — proof gate passed"
