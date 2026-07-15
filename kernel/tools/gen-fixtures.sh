#!/usr/bin/env bash
# Regenerate the committed plan.json from the plan YAML source of truth.
#
# Source of truth : tests/fixtures/{valid,invalid}/*/plan.yaml
# Output (committed, regenerable, never hand-edited):
#   - <fixture>/plan.json          yaml -> json (js-yaml; Node only, no Python)
#
# NOTE (#85 / D28): the MoonBit fixture embedding (kernel/src/fixtures_gen.mbt via
# gen-fixtures-mbt.mjs) was RETIRED with the legacy WIT/topology-calculator kernel
# path — both are deleted, so this script no longer emits any MoonBit source. The
# toy fixtures under tests/fixtures/{valid,invalid} are a separate D20 concern; the
# live kernel is tested against the real XOC oracle, not these.
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
