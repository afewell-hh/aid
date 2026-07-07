#!/usr/bin/env bash
# run-evidence.sh — orchestrate air-gapped GUI evidence capture (P2.5, #73).
#
# Boots ONE seeded `aid serve` on a free ephemeral port with a temp plans dir,
# seeds the vendored oracle plans through the REAL REST API, then runs the
# Playwright-driven generator (ui/test/gen-evidence.mjs) in headless chromium to
# capture each surface AFTER a real request round-trip — writing self-contained
# HTML under ui/docs/evidence. PNG screenshots under ui/docs/screenshots are
# refreshed only when AID_UPDATE_SCREENSHOTS=1 because browser PNG output is
# run-variable across environments. Always tears the server down.
#
# Same offline shims as run-e2e.sh: chromium via PLAYWRIGHT_BROWSERS_PATH, the
# @playwright/test module via an ephemeral ui/test/node_modules symlink.
#
# Required env / args (the `ui-evidence` Makefile target provides these):
#   AID_BIN, PLAYWRIGHT_NODE_MODULES, PLAYWRIGHT_BROWSERS_PATH
# Optional:
#   AID_UPDATE_SCREENSHOTS=1  refresh tracked PNG screenshots
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

AID_BIN="${AID_BIN:-$REPO_ROOT/bin/aid}"
PLAYWRIGHT_NODE_MODULES="${PLAYWRIGHT_NODE_MODULES:-$HOME/afewell-hh/hh-learn/node_modules}"
export PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-$HOME/.cache/ms-playwright}"

if [ ! -x "$AID_BIN" ]; then
  echo "run-evidence: aid binary not found at $AID_BIN (run 'make ui-evidence')" >&2
  exit 1
fi
if [ ! -e "$PLAYWRIGHT_NODE_MODULES/@playwright/test" ]; then
  echo "run-evidence: @playwright/test not found under $PLAYWRIGHT_NODE_MODULES" >&2
  exit 1
fi

free_port() {
  python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()'
}

PORT="$(free_port)"
PLANS_DIR="$(mktemp -d)"
PID=""
LINK="$REPO_ROOT/ui/test/node_modules"
LINK_CREATED=""

cleanup() {
  [ -n "$PID" ] && kill "$PID" 2>/dev/null || true
  rm -rf "$PLANS_DIR"
  [ -n "$LINK_CREATED" ] && [ -L "$LINK" ] && rm -f "$LINK" || true
}
trap cleanup EXIT INT TERM

# Detach the server so it does not hold this script's stdout pipe open.
setsid "$AID_BIN" serve --port "$PORT" --plans-dir "$PLANS_DIR" >/tmp/aid-evidence.log 2>&1 </dev/null &
PID=$!

for _ in $(seq 1 50); do
  curl -fsS "http://127.0.0.1:$PORT/api/plans" >/dev/null 2>&1 && break
  sleep 0.2
done

BASE_URL="http://127.0.0.1:$PORT"
bash "$REPO_ROOT/ui/test/seed-oracle-plans.sh" "$BASE_URL" "$REPO_ROOT"

# ESM bare-specifier resolution: gen-evidence.mjs lives in ui/test, so an
# ephemeral ui/test/node_modules symlink lets `import "@playwright/test"` resolve
# (same shim as run-e2e.sh; .gitignored; dropped by the trap).
if [ ! -e "$LINK" ]; then
  ln -s "$PLAYWRIGHT_NODE_MODULES" "$LINK"
  LINK_CREATED=1
fi

echo "run-evidence: BASE_URL=$BASE_URL"
BASE_URL="$BASE_URL" node "$REPO_ROOT/ui/test/gen-evidence.mjs"
