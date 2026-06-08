#!/usr/bin/env bash
# Regenerate the vendored device-class-bom[] test data for the BOM adapter.
#
# Source of truth : the merged Phase-3 kernel (aid/kernel) calculate(), run over
#                   each valid fixture's embedded plan JSON via tools/bom-gen.
# Output (committed, regeneratable, never hand-edited):
#   bom-adapter/tests/testdata/<fixture>.boms.json
#
# This is the single-sourced Layer-1 -> Layer-2 wire contract (snake_case JSON
# mapped field-for-field to wit/types.wit `device-class-bom`; see BOM_CONTRACT.md).
# The adapter RENDERS this data — it does not recompute the BOM or re-derive the
# role-based root rule. bom-gen is strictly additive interim tooling — it makes
# NO edits under kernel/.
#
# Requires: moon (~/.moon/bin), node.
# Run from anywhere:  bom-adapter/tools/gen-bom.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
adapter="$(cd "$here/.." && pwd)"
export PATH="$HOME/.moon/bin:$PATH"

testdata="$adapter/tests/testdata"
mkdir -p "$testdata"

# Run the kernel-backed generator; it prints {fixture-name: [device-class-bom...]}
# as one JSON object.
all="$(cd "$here/bom-gen" && moon run gen 2>/dev/null)"

# Split into one pretty-printed file per fixture (whitespace is insignificant;
# the contract is the field shape, not the byte layout).
printf '%s' "$all" | node -e '
  const fs = require("fs");
  const o = JSON.parse(fs.readFileSync(0, "utf8"));
  const dir = process.argv[1];
  for (const [name, boms] of Object.entries(o)) {
    if (boms === null) { throw new Error("kernel returned no BOMs for " + name); }
    const path = dir + "/" + name + ".boms.json";
    fs.writeFileSync(path, JSON.stringify(boms, null, 2) + "\n");
    const lines = boms.reduce((n, b) => n + b.line_items.length, 0);
    console.error("wrote " + path +
      " (boms=" + boms.length + " line_items=" + lines + ")");
  }
' "$testdata"
