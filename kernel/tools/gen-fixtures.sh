#!/usr/bin/env bash
# Regenerate the kernel's fixture test data from the plan YAML source of truth.
#
# Source of truth : tests/fixtures/{valid,invalid}/*/plan.yaml
# Outputs (committed, regenerable, never hand-edited):
#   - <fixture>/plan.json          yaml -> json (js-yaml; Node only, no Python)
#   - kernel/src/fixtures_gen.mbt  plan.json embedded as MoonBit String constants
#
# Run from anywhere:  kernel/tools/gen-fixtures.sh
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo"

for y in tests/fixtures/valid/*/plan.yaml tests/fixtures/invalid/*/plan.yaml; do
  d="$(dirname "$y")"
  npx --yes js-yaml@4.1.0 "$y" > "$d/plan.json"
  echo "wrote $d/plan.json"
done

node kernel/tools/gen-fixtures-mbt.mjs
