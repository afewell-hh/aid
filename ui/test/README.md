# AID GUI test harnesses

Two complementary harnesses exercise the MoonBit→JS GUI (`ui/src/*.mbt` →
`ui/static/app.js`):

## 1. Fast unit/smoke harness — `make ui-test`

`ui/test/*.test.mjs` drive the REAL compiled `app.js` render/wire functions
against a dependency-free mock DOM + fetch stub (`harness.mjs`). No npm deps, no
browser, air-gapped. This is the fast inner-loop guard and stays the default.

## 2. Real-browser E2E harness — `make ui-e2e`

`ui/test/e2e/*.spec.mjs` run in **headless chromium via Playwright** against a
**live `aid serve`**. The `make ui-e2e` target owns the full lifecycle:

1. `make ui` (rebuild `app.js`) + `go build -o ./bin/aid ./cmd/aid`.
2. Boot `aid serve` on a free ephemeral port with a temp `--plans-dir`
   (the seeded server) and a second one with an empty store (the empty-state
   server).
3. Seed the seeded server with the vendored oracle plans through the **real**
   `POST /api/plans` / `PUT /api/plans/{id}/overlay` (`seed-oracle-plans.sh`),
   plus one synthetic over-allocated plan for the calc-invalid path.
4. Export `BASE_URL` / `EMPTY_URL` and run `playwright test`.
5. Always tear both servers down (shell `trap`).

### Offline chromium resolution (no network installs)

This package declares `@playwright/test@1.48.2` as a devDependency but **does NOT
vendor `node_modules`** and does NOT run `npx playwright install`. Both are
resolved from an existing install:

- **Playwright module**: resolved via `NODE_PATH` pointing at a sibling
  `node_modules` that already has `@playwright/test@1.48.2`
  (`PLAYWRIGHT_NODE_MODULES`, default
  `~/afewell-hh/hh-learn/node_modules`). The make target invokes that
  install's `playwright` CLI directly.
- **Chromium browser**: resolved via
  `PLAYWRIGHT_BROWSERS_PATH=~/.cache/ms-playwright` (override with
  `PLAYWRIGHT_BROWSERS_PATH=...`). Playwright 1.48.2 expects chromium
  **revision 1140** (browserVersion 130.0.6723.31), which is present in that
  cache.

This is the least-hacky reproducible offline path given the aid repo has no
`node_modules` and offline `playwright install` is blocked. **CI must provide
both**: a Playwright 1.48.x install reachable via `PLAYWRIGHT_NODE_MODULES`
(or a normal `npm ci` inside `ui/test/`) and a matching chromium cache via
`PLAYWRIGHT_BROWSERS_PATH`. Override either with environment variables:

```
make ui-e2e \
  PLAYWRIGHT_NODE_MODULES=/path/to/node_modules \
  PLAYWRIGHT_BROWSERS_PATH=/path/to/ms-playwright
```

### Covered cases (`golden-path.spec.mjs`)

- (a) initial load renders the plan list from `GET /api/plans`.
- (b) empty store renders the table shell with `0 plan(s)` and no rows
  (against `EMPTY_URL`).
- (c) clicking a plan's **View** renders the detail.
- (d) **Calculate** on a valid **mesh** plan (xoc-64) → Valid badge + correct
  switch/server quantities.
- (d′) **Calculate** on a valid **Clos** plan (xoc-256) → Valid badge +
  correct switch/server quantities.
- (e) **Calculate** on the synthetic over-allocated plan → Invalid badge +
  `ZONE_OVERFLOW` error text (calc-errors-as-data).
- (f) **View BOM** → rows render.
