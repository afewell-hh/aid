#!/usr/bin/env bash
# LEGACY / NON-LIVE drift guard (#84): ensure kernel/src/types.mbt still covers
# every type in the RETIRED pre-rebuild WIT contract (wit/). wit/ is NOT the live
# source of truth: after the foundation rebuild (D18–D27) the live host boundary is
# export_f2_calculate / export_f3_bom, whose JSON shapes live in
# kernel/src/f2_types.mbt and are NOT WIT-mirrored (DECISIONS.md D16, amended).
# This guard is retained only to keep the quarantined legacy WIT ↔ types.mbt mirror
# internally consistent, pending a separate retire-vs-reconcile decision. It does
# NOT validate the live F2/F3 contract.
#
# Mechanics: types.mbt is a hand-authored mirror, so this check regenerates the
# `wit-bindgen moonbit` bindings as a reference oracle and fails if any WIT
# record/enum/variant is missing from types.mbt.
#
# Runnable check — wire into CI / run before committing a wit/ change.
# Exit 0 = in sync; exit 1 = drift (a legacy WIT type not mirrored in types.mbt).
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
echo "OK (legacy/quarantined — #84): kernel/src/types.mbt mirrors all $count retired WIT types (types + topology-calculator interfaces). This guards the pre-rebuild WIT only, not the live F2/F3 boundary."
