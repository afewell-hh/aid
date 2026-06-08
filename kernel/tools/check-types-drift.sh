#!/usr/bin/env bash
# Drift guard: ensure kernel/src/types.mbt still covers every type in the WIT
# contract (wit/), which is the source of truth (DECISIONS.md D16; issue #6
# type-sourcing decision). types.mbt is a hand-authored mirror, so this check
# regenerates the `wit-bindgen moonbit` bindings as a reference oracle and fails
# if any WIT record/enum/variant is missing from types.mbt.
#
# Runnable check — wire into CI / run before committing a wit/ change.
# Exit 0 = in sync; exit 1 = drift (a WIT type not mirrored in types.mbt).
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo"

gen=/tmp/aid-typedrift-gen
scratch=/tmp/aid-typedrift-scratch
rm -rf "$gen" "$scratch"
mkdir -p "$scratch"
# wit-bindgen moonbit writes the `types` interface relative to CWD and the rest
# under --gen-dir (the spike's documented behaviour); run from a scratch dir.
( cd "$scratch" && wit-bindgen moonbit "$repo/wit" --gen-dir "$gen" ) >/dev/null 2>&1

types_oracle="$scratch/interface/aid/core/types/top.mbt"
calc_oracle="$gen/interface/aid/core/topologyCalculator/top.mbt"
mirror="$repo/kernel/src/types.mbt"

extract() { grep -hoE 'pub\(all\) (struct|enum) [A-Za-z0-9_]+' "$@" | awk '{print $3}' | sort -u; }

oracle_names="$(extract "$types_oracle" "$calc_oracle")"
mirror_names="$(extract "$mirror")"

missing="$(comm -23 <(echo "$oracle_names") <(echo "$mirror_names"))"

if [ -n "$missing" ]; then
  echo "TYPE DRIFT — these WIT types are not mirrored in kernel/src/types.mbt:" >&2
  echo "$missing" | sed 's/^/  - /' >&2
  echo "" >&2
  echo "Update kernel/src/types.mbt to match wit/ (see the oracle at $types_oracle)." >&2
  exit 1
fi

count="$(echo "$oracle_names" | grep -c . || true)"
echo "OK: kernel/src/types.mbt mirrors all $count WIT types (types + topology-calculator interfaces)."
