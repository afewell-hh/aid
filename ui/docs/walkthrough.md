# AID web frontend — walkthrough & air-gapped evidence

The GUI is MoonBit→JS (`ui/src`, compiled to `ui/static/app.js`) over the REST API,
with Bootstrap 5 bundled locally (no CDN). Everything is served by `aid serve` from
the embedded `ui/static/`. As of **F7c (#64)** the GUI consumes the rebuilt engine's
response shapes (REST runs on `internal/design`, F7b): the calc `CalcOutput`
(`is_valid` + switch/server quantity arrays + endpoints + transceiver verdicts) and
the flat BOM `rows[]` — not the retired `ir{nodes,edges,fabrics}` / hierarchical
per-unit+fleet shapes.

## Reproduce

```bash
# 1. Build the bundle + the single static binary (embeds ui/static via go:embed).
make ui
go build -o aid ./cmd/aid

# 2. Run the server with an empty plans dir.
./aid serve --port 8080 --plans-dir /tmp/aid-plans

# 3. Seed a plan from a DIET/training bundle, then attach its optic overlay (the
#    companion sub-resource, F7b). Any committed XOC composition works.
curl -s -X POST --data-binary @tests/oracle/xoc-256-2xopg128-clos-ro/training.yaml \
  http://localhost:8080/api/plans                                   # -> {"id": "<id>", ...}
curl -s -X PUT  --data-binary @tests/oracle/xoc-256-2xopg128-clos-ro/optic-overlay.yaml \
  http://localhost:8080/api/plans/<id>/overlay

# 4. Open http://localhost:8080/ and walk: plan list -> View -> Calculate -> View BOM.
```

### Air-gapped evidence (no browser required)

This environment has no headless browser, so the F7c evidence is generated from the
**same compiled bundle** the server ships, via the Node DOM/fetch stub used by the
smoke tests:

```bash
node ui/docs/gen-evidence.mjs   # writes self-contained pages to ui/docs/evidence/
```

Open `ui/docs/evidence/*.html` offline (they link the locally-bundled Bootstrap CSS);
the markup is byte-identical to `aid serve`'s output. The PNGs under `screenshots/`
are the Phase-6b captures (old shapes), superseded by these pages — regenerate PNGs
with headless Chromium against a running `aid serve` when a browser is available.

## Walkthrough

### 1. Plan list (`GET /api/plans`)
NetBox-style dark navbar, table of plans, status badges, per-row **View**.
[`evidence/01-plan-list.html`](evidence/01-plan-list.html)

### 2. Plan detail (`GET /api/plans/{id}`)
Cards: plan summary (ID, status, actions) + the canonical DIET/training YAML.
[`evidence/02-plan-detail.html`](evidence/02-plan-detail.html)

### 3. Calc trigger (`POST /api/plans/{id}/calc`) — two-plane validation
A green **Valid** badge + the computed **switch/server quantities** (per class) and
an endpoint/verdict summary. A calc constraint violation renders a red **Invalid**
badge with the error codes (e.g. `ZONE_OVERFLOW`), surfaced as data (HTTP 200). A
*structural* failure (a 4xx `{"error": ...}` body) renders a distinct danger alert,
never a validity badge.
[`evidence/03-calc-valid.html`](evidence/03-calc-valid.html) ·
[`evidence/04-calc-invalid.html`](evidence/04-calc-invalid.html) ·
[`evidence/05-calc-structural.html`](evidence/05-calc-structural.html)

### 4. BOM (`GET /api/plans/{id}/bom`)
A flat table of the projection `rows[]` (section / model / hedgehog class /
manufacturer / quantity) plus the suppressed-cable-assembly footer.
[`evidence/06-bom.html`](evidence/06-bom.html)

## Air-gapped run (no network; all assets from the binary)

Every request the page makes is same-origin (served from the embedded FS) — there
are **no CDN / external requests**. Captured during the walkthrough above:

```
http://localhost:8080/
http://localhost:8080/static/bootstrap.min.css
http://localhost:8080/static/bootstrap.bundle.min.js
http://localhost:8080/static/app.js
http://localhost:8080/api/plans
http://localhost:8080/api/plans/<id>
http://localhost:8080/api/plans/<id>/overlay
http://localhost:8080/api/plans/<id>/calc
http://localhost:8080/api/plans/<id>/bom

external (non-localhost) requests: 0
PASS: all assets loaded from the binary (no CDN).
```
