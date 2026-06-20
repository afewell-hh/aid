# Follow-up: kernel validation codes for surfaces (deferred from F7)

**Status:** deferred / not started. Tracked here because `gh` is unavailable in
the deva environment — **lead to file as a GitHub issue** and link from #64.

**Origin:** F7 surfaces retarget (#64), arch note §3.0 scope boundary + the F7b
review thread.

## What was deferred

The pre-rebuild REST/CLI validation path (`internal/orchestrate` →
`export_validate`) emitted **named semantic constraint codes** as
validation-as-data — e.g. `MCLAG_SWITCH_COUNT` (asserted by the old
`serve_test.go` / `cli_test.go`). The rebuilt engine's two-plane validation
(note §3.0) surfaces:

- **structural** failures (ingest / unresolved refs / kernel infra) → `4xx`, and
- **calc constraint violations** from the kernel as data (`is_valid:false` +
  `CalcOutput.Errors[{code,message}]`) — currently chiefly `ZONE_OVERFLOW`
  (over-allocation) and `INVALID_PLAN` (malformed calc-plan).

F7 deliberately **dropped the old `export_validate` code set** (and the warnings
channel) rather than re-implement it: those codes were tied to the retired
invented plan schema, there is no HNP oracle for them, and inventing equivalents
would be speculative. F7 surfaces exactly what the rebuilt kernel already
validates — no new kernel validation logic was added.

## The follow-up

Decide, against the real HNP/DIET semantics (not the retired invented schema),
**which named pre-flight validation codes the surfaces should report** beyond the
current calc errors — e.g. an MCLAG odd-member-count check, redundancy-group
sanity, zone/port-spec coherence — and implement them as **proved kernel checks**
(or Go pre-checks where no proof obligation exists), each with a committed oracle
or a clearly-labelled AID-defined rule.

**Explicitly NOT** a reason to keep `internal/orchestrate` alive: this is new
engine validation work on the rebuilt path, independent of the retired adapters.

## Acceptance (when picked up)

- A defined set of validation codes with provenance (HNP-derived vs AID-defined).
- Surfaced through `calc.CalcOutput.Errors` (so REST/CLI/GUI get them for free via
  the existing two-plane contract).
- Tests pinning each code against a fixture that genuinely triggers it.
- `moon prove` green if any check lands in the proved kernel.
