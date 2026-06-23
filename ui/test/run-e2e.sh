#!/usr/bin/env bash
# run-e2e.sh — orchestrate the AID GUI real-browser E2E run.
#
# Boots two `aid serve` instances (a seeded store + an empty store) on free
# ephemeral ports with temp plans dirs, seeds the vendored oracle plans through
# the real REST API, runs the Playwright spec in headless chromium against the
# live servers, and always tears everything down.
#
# Required env / args:
#   AID_BIN                 path to the built aid binary (default ./bin/aid)
#   PLAYWRIGHT_NODE_MODULES node_modules with @playwright/test (offline resolve)
#   PLAYWRIGHT_BROWSERS_PATH chromium cache (offline resolve)
#
# Invoked by the `ui-e2e` Makefile target; runnable standalone from the repo
# root once `make ui` + the binary build have happened.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

AID_BIN="${AID_BIN:-$REPO_ROOT/bin/aid}"
PLAYWRIGHT_NODE_MODULES="${PLAYWRIGHT_NODE_MODULES:-$HOME/afewell-hh/hh-learn/node_modules}"
export PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-$HOME/.cache/ms-playwright}"

if [ ! -x "$AID_BIN" ]; then
  echo "run-e2e: aid binary not found at $AID_BIN (run 'make ui-e2e' or build first)" >&2
  exit 1
fi
PW_CLI="$PLAYWRIGHT_NODE_MODULES/.bin/playwright"
if [ ! -e "$PW_CLI" ]; then
  echo "run-e2e: playwright CLI not found at $PW_CLI" >&2
  echo "         set PLAYWRIGHT_NODE_MODULES to a node_modules with @playwright/test@1.48.x" >&2
  exit 1
fi

free_port() {
  python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()'
}

SEED_PORT="$(free_port)"
EMPTY_PORT="$(free_port)"
SEED_DIR="$(mktemp -d)"
EMPTY_DIR="$(mktemp -d)"
SEED_PID=""
EMPTY_PID=""

cleanup() {
  [ -n "$SEED_PID" ] && kill "$SEED_PID" 2>/dev/null || true
  [ -n "$EMPTY_PID" ] && kill "$EMPTY_PID" 2>/dev/null || true
  rm -rf "$SEED_DIR" "$EMPTY_DIR"
}
trap cleanup EXIT INT TERM

wait_up() {
  local port="$1"
  for _ in $(seq 1 50); do
    curl -fsS "http://127.0.0.1:$port/api/plans" >/dev/null 2>&1 && return 0
    sleep 0.2
  done
  echo "run-e2e: server on port $port never came up" >&2
  return 1
}

# Detach the servers so they do not hold this script's stdout pipe open.
setsid "$AID_BIN" serve --port "$SEED_PORT"  --plans-dir "$SEED_DIR"  >/tmp/aid-e2e-seed.log  2>&1 </dev/null &
SEED_PID=$!
setsid "$AID_BIN" serve --port "$EMPTY_PORT" --plans-dir "$EMPTY_DIR" >/tmp/aid-e2e-empty.log 2>&1 </dev/null &
EMPTY_PID=$!

wait_up "$SEED_PORT"
wait_up "$EMPTY_PORT"

BASE_URL="http://127.0.0.1:$SEED_PORT"
EMPTY_URL="http://127.0.0.1:$EMPTY_PORT"

bash "$REPO_ROOT/ui/test/seed-oracle-plans.sh" "$BASE_URL" "$REPO_ROOT"

export BASE_URL EMPTY_URL

# ESM bare-specifier resolution (the config + spec `import "@playwright/test"`)
# requires a node_modules adjacent to ui/test — NODE_PATH does NOT cover ESM.
# Since the aid repo vendors none, create an EPHEMERAL symlink to the resolved
# install for the duration of the run, then remove it. The link is .gitignored
# and the cleanup trap drops it even on failure. This is the least-hacky
# reproducible offline shim (a real `npm ci` in ui/test/ would produce the same
# node_modules and is what CI should do).
LINK="$REPO_ROOT/ui/test/node_modules"
LINK_CREATED=""
if [ ! -e "$LINK" ]; then
  ln -s "$PLAYWRIGHT_NODE_MODULES" "$LINK"
  LINK_CREATED=1
fi
drop_link() { [ -n "$LINK_CREATED" ] && [ -L "$LINK" ] && rm -f "$LINK" || true; }
trap 'drop_link; cleanup' EXIT INT TERM

echo "run-e2e: BASE_URL=$BASE_URL EMPTY_URL=$EMPTY_URL"
echo "run-e2e: PLAYWRIGHT_BROWSERS_PATH=$PLAYWRIGHT_BROWSERS_PATH"
echo "run-e2e: @playwright/test via $PLAYWRIGHT_NODE_MODULES (ephemeral ui/test/node_modules symlink)"

cd "$REPO_ROOT/ui/test"
"$PW_CLI" test -c playwright.config.mjs
