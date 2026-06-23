#!/usr/bin/env bash
# seed-oracle-plans.sh — seed the live `aid serve` with the vendored oracle
# plans through the REAL REST API (POST /api/plans, PUT .../overlay), plus one
# synthetic over-allocated plan that exercises the calc-errors-as-data path.
#
# Usage: seed-oracle-plans.sh BASE_URL REPO_ROOT
#   BASE_URL   e.g. http://127.0.0.1:54321
#   REPO_ROOT  the aid repo root (for tests/oracle/*)
#
# Idempotent enough for a fresh temp store; exits non-zero if a seed fails.
set -euo pipefail

BASE="${1:?BASE_URL required}"
ROOT="${2:?REPO_ROOT required}"
ORACLE="$ROOT/tests/oracle"

post_plan() {
  local file="$1"
  curl -fsS -X POST --data-binary "@$file" "$BASE/api/plans" >/dev/null
}

put_overlay() {
  local id="$1" file="$2"
  [ -f "$file" ] || return 0
  curl -fsS -X PUT --data-binary "@$file" "$BASE/api/plans/$id/overlay" >/dev/null
}

# 1) Vendored oracle plans (mesh xoc-64, Clos xoc-256, mesh xoc-128).
post_plan "$ORACLE/xoc-64-mesh-conv-ro/training.yaml"
post_plan "$ORACLE/xoc-256-2xopg128-clos-ro/training.yaml"
post_plan "$ORACLE/xoc-128-2xopg64-mesh-conv-ro/training.yaml"

put_overlay "training_xoc256_2xopg128_clos_ro" "$ORACLE/xoc-256-2xopg128-clos-ro/optic-overlay.yaml"
put_overlay "training_xoc128_2xopg64_mesh_conv_ro" "$ORACLE/xoc-128-2xopg64-mesh-conv-ro/optic-overlay.yaml"

# 2) Synthetic over-allocated plan (calc-errors-as-data): a copy of xoc-64 with
#    compute_xpu inflated past the soc/storage leaf server-zone capacity. The
#    live kernel returns is_valid:false + ZONE_OVERFLOW errors as DATA (200).
overflow_yaml="$(mktemp)"
sed \
  -e 's/training_xoc64_1xopg64_mesh_conv_ro/invalid_zone_overflow/' \
  -e 's/Training XOC-64 1x OPG-64 Mesh Converged RO/Invalid Zone Overflow (xoc-64 over-allocated)/' \
  "$ORACLE/xoc-64-mesh-conv-ro/training.yaml" \
  | awk '
      /server_class_id: compute_xpu/ { inblk=1 }
      inblk && /quantity: 8/ { sub(/quantity: 8/, "quantity: 40"); inblk=0 }
      { print }
    ' > "$overflow_yaml"
post_plan "$overflow_yaml"
rm -f "$overflow_yaml"

echo "seeded oracle plans + 1 synthetic over-allocated plan"
