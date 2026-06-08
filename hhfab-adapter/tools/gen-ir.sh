#!/usr/bin/env bash
# Regenerate the vendored topology-ir test data for the hhfab adapter.
#
# Source of truth : the merged Phase-3 kernel (aid/kernel) calculate(), run over
#                   each fixture's embedded plan JSON via tools/ir-gen.
# Output (committed, regeneratable, never hand-edited):
#   hhfab-adapter/tests/testdata/<fixture>.ir.json
#
# This is the single-sourced Layer-1 -> Layer-2 wire contract (snake_case JSON
# mapped field-for-field to wit/types.wit `topology-ir`; see IR_CONTRACT.md).
# ir-gen is strictly additive interim tooling — it makes NO edits under kernel/.
#
# Requires: moon (~/.moon/bin), node (already used by kernel/tools/gen-fixtures.sh).
# Run from anywhere:  hhfab-adapter/tools/gen-ir.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
adapter="$(cd "$here/.." && pwd)"
export PATH="$HOME/.moon/bin:$PATH"

testdata="$adapter/tests/testdata"
mkdir -p "$testdata"

# Run the kernel-backed generator; it prints {fixture-name: <ir>} as one JSON object.
all="$(cd "$here/ir-gen" && moon run gen 2>/dev/null)"

# Split into one pretty-printed file per fixture (whitespace is insignificant;
# the contract is the field shape, not the byte layout).
printf '%s' "$all" | node -e '
  const fs = require("fs");
  const o = JSON.parse(fs.readFileSync(0, "utf8"));
  const dir = process.argv[1];
  for (const [name, ir] of Object.entries(o)) {
    if (ir === null) { throw new Error("kernel returned no IR for " + name); }
    const path = dir + "/" + name + ".ir.json";
    fs.writeFileSync(path, JSON.stringify(ir, null, 2) + "\n");
    console.error("wrote " + path +
      " (nodes=" + ir.nodes.length + " edges=" + ir.edges.length + ")");
  }
' "$testdata"
