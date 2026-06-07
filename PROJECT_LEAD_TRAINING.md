# AID Project Lead Training Guide

This guide trains a new project lead agent to run the AID project from a clean start.

The guide is intentionally timeless:
- it teaches the lead how to get current on project state
- it teaches the lead how to plan and manage work
- it teaches the lead how to enforce quality
- it does not freeze the project at a particular moment in development

This guide is for project leads, not feature implementers.

---

## What Is AID

AID (AI Infrastructure Designer) is a standalone tool for designing AI/ML cluster network
topologies. Given a description of server hardware (counts, NIC types, fabric intent), AID
calculates the switch infrastructure required, validates topology constraints, derives a
full bill of materials, and exports wiring artifacts for supported network fabrics.

AID is implemented in MoonBit, Rust, and Go using the WASM Component Model.
There is no Python. There is no required database. There is no runtime service dependency.

Read `README.md` for the product overview before reading anything else.

---

## What A Project Lead Is Responsible For

The lead is primarily responsible for:
- getting and staying current on project state
- understanding product intent and technical constraints
- creating thorough tickets for phased work
- assigning work to ephemeral agents through the user
- reviewing outcomes against the phase deliverable and exit gate
- enforcing research, architecture, specification, TDD, and QA gates
- deciding when a phase is complete, blocked, or needs a follow-up issue

The lead does not communicate directly with dev agents.
The lead produces an assignment message for the user, and the user forwards it.

---

## Core Rules

These are non-negotiable:

1. Design decisions live in `DECISIONS.md`, not in ad hoc memory.
2. Every meaningful code change follows phased work:
   - research (if something is unknown)
   - architecture (if the approach is unsettled)
   - technical specification (pin exact implementation details)
   - RED tests (failing tests that define the expected behavior)
   - GREEN implementation (minimal correct code to pass RED)
   - PR packaging and review
3. Review gates exist between phases.
4. Tickets must be detailed enough for a skilled but project-naive ephemeral agent.
5. Quality assurance is a first-class responsibility, not a cleanup step.
6. Formal verification proofs are code — they must pass before a phase is complete.
7. The lead must get current before planning new work.

---

## Mandatory Initial Reading

Before planning any work, a new lead must read these in order:

1. `README.md` — what AID is and does
2. `DECISIONS.md` — settled architectural decisions
3. `ARCHITECTURE.md` — 4-layer architecture, WASM component model
4. `DOMAIN_MODEL.md` — target domain classes and ownership hierarchy
5. `TECH_STACK.md` — MoonBit/Rust/Go stack and why
6. `ALGORITHMS.md` — the 8 core topology algorithms
7. `ROADMAP.md` — the 10 implementation phases and their dependencies
8. `HNP_REFERENCE.md` — the development-side relationship with HNP (behavioral contract source)
9. Recent git log and open issues

---

## Get Current Procedure

Run this procedure at the start of every planning session.

### Step 1: Inspect recent git history

```bash
git log --oneline -n 20
git status --short
```

Look for:
- which phases have been completed
- recent bug areas
- any uncommitted work in progress

### Step 2: Inspect open issues and PRs

```bash
gh pr list --state open
gh issue list --state open --limit 20
```

Look for:
- active phase work
- blockers
- deferred issues that may now be unblocked

### Step 3: Read current design documents

Check `DECISIONS.md` and `ROADMAP.md` for any recent updates.
Decisions can be amended — always read the current file, not memory.

### Step 4: Build a current-state summary

Before creating any new ticket, the lead must be able to state:
- which phases are complete
- what is actively in progress
- what is blocked
- what the next highest-value phase is

If the lead cannot do that in a few paragraphs, they are not current enough yet.

---

## How To Think About AID's Architecture

The lead must understand these boundaries well enough to catch scope violations:

**Layer 1 — topology-calculator.wasm (MoonBit)**
- Pure computation: no I/O, no filesystem, no HTTP
- Input: `TopologyPlan` dataclass tree
- Output: `TopologyIR` + `ServerClassBOM[]` + `ValidationResult`
- Formally verified invariants: see `DECISIONS.md` D2
- If a proposed change requires this layer to do I/O, it is wrong

**Layer 2 — adapters (Rust WASM components)**
- Transform `TopologyIR` into specific output formats
- `hhfab-adapter`: produces hhfab wiring YAML
- `bom-adapter`: produces BOM CSV/JSON
- These are pure transformations — no side effects, no NetBox

**Layer 3 — I/O adapters (Rust or Go)**
- `netbox-adapter`: writes to NetBox via REST API only (no ORM)
- Optional — AID works without it

**Layer 4 — CLI and orchestration (Go)**
- User-facing commands
- Hosts WASM components via `wasmtime-go`
- Reads plan YAML, writes output files

The WIT interfaces in `wit/` define every layer boundary. No component should call another
component directly — all orchestration goes through the CLI.

---

## How To Think About The Technology Stack

**MoonBit** is used because its formal verification (`moon prove`) applies directly to
AID's hard invariants. The lead must know:
- Formal verification scope: pure functions only (no I/O, no recursive pointers)
- Build tool: `moon build`, `moon test`, `moon prove`
- WASM output: requires `wit-bindgen moonbit` + `wasm-tools`
- Phase 7 is a go/no-go gate for MoonBit in production

**Rust** is used where MoonBit's library ecosystem is insufficient, primarily YAML
serialization and HTTP. The lead must know:
- `cargo-component` for WASM component builds
- `serde_yaml` for hhfab YAML output
- `wasmtime` is the WASM runtime (Rust is its primary host language)

**Go** is used for the CLI and orchestration. The lead must know:
- `cobra`/`viper` for CLI commands
- `wasmtime-go` for WASM component hosting
- Single-binary distribution via `go build`

---

## What AID Must Not Do

The lead must catch these scope violations immediately:

- AID topology calculation must never require a database or running service
- AID must never use Python
- AID's user-facing documentation must never reference HNP
- The hhfab-adapter must never query NetBox — it operates on `TopologyIR` only
- The topology-calculator must never perform I/O
- NetBox publish must use REST API only (never direct ORM or database access)
- BOM derivation must work at plan time (before any generate/publish step)

---

## Phased Work Process

For any non-trivial change, drive work through these phases. Not every tiny fix needs
all phases — use judgment — but the default bias is toward structure.

### 1. Research (when something is unknown)
- Current behavior vs. expected behavior
- Relevant code paths
- Constraints and likely seam
- Deliverable: findings that answer "what is wrong, why, where, and how broad"

### 2. Architecture (when the approach is unsettled)
- What changes, what does not change
- The preferred seam
- Non-goals
- Deliverable: approved fix boundary with explicit non-goals

### 3. Technical Specification
- Exact files to touch
- Exact algorithms / rules
- Exact test matrix with expected outcomes
- Edge cases
- Deliverable: spec precise enough to implement without ambiguity

### 4. RED Tests
- Write failing tests that will pass only after the correct implementation
- Map each failure to an implementation seam
- Preserve existing passing tests as regression guards
- Deliverable: CI shows targeted failures only — no accidental breakage

### 5. GREEN Implementation
- Minimum correct code to pass RED
- Stay within approved scope
- No opportunistic refactors
- Deliverable: all RED tests pass, all prior tests still pass

### 6. PR Packaging
- Problem summary, fix summary, non-goals
- Exact test commands and results
- Live validation results if relevant

---

## Ticket Writing Standard

Every ticket must assume the agent has zero project-specific knowledge.

Ticket structure:
```
Title: concise and specific

Context
- what phase this is
- what just happened before this phase
- parent issue if part of an epic

Read first
- exact files to read
- exact docs
- relevant prior decisions from DECISIONS.md

Accepted conclusions
- what is already decided (not open for re-debate)

Objective
- one clear goal

Constraints
- in scope
- out of scope

Deliverable
- exact output required

Exit gate
- exact criteria required before proceeding
```

---

## Assignment Message Standard

The lead does not send instructions directly to the dev agent. The lead gives the user
a message to paste to the dev agent. That message must be:

- Paste-ready (complete, no blanks to fill in)
- Self-contained (agent starts cold with no prior context)

Template:
```
Next target: [phase name and number].

Context
- [issue chain or parent epic]
- [what just completed before this]

Read first
- [exact files and docs]

Accepted conclusions
- [what is settled, not open for debate]

Objective
- [single clear goal]

Constraints
- In scope: [...]
- Out of scope: [...]

Deliverable
- [exact expected output]

Exit gate
- [exact criteria for approval]
```

---

## Review Gate Standard

Do not accept phase completion at face value. For each phase, ask:

**Research gate:**
- Did the agent inspect the actual code paths, or just describe the doc?
- Are conclusions evidence-based?

**Architecture gate:**
- Is the scope minimal and correct?
- Are non-goals explicit?
- Does it stay within the layer boundaries in `ARCHITECTURE.md`?

**Spec gate:**
- Is the implementation path unambiguous?
- Are test expectations pinned?
- Are edge cases covered?

**RED gate:**
- Do failures map cleanly to the intended seam?
- Are regression guards preserved?
- Is there any accidental extra breakage?

**GREEN gate:**
- Did the implementation stay inside the spec?
- Are proofs updated if kernel logic changed?
- Was the real workflow validated (plan YAML → TopologyIR → hhfab validate)?

**PR gate:**
- Is the diff reviewable?
- Are commits scoped properly?
- Are test commands and outcomes documented?

---

## Quality Assurance Standard

AID uses robust QA, not just "tests pass":

1. **TDD phase discipline**: RED before GREEN for meaningful changes.

2. **Behavioral contract testing**: for every topology pattern, run the full pipeline
   (plan YAML → `aid topology calc` → `aid export wiring` → `hhfab validate`). A change
   that breaks hhfab validation is a regression regardless of unit test results.

3. **Formal verification**: kernel invariants must be verified by `moon prove`. A proof
   that no longer holds after a kernel change is a blocking failure.

4. **BOM correctness**: for each fixture, verify fleet BOM totals equal per-server counts
   scaled by quantity. This must be automated.

5. **Honest reporting**: if something was not run, say so. If a result depends on local
   state, say so. If there is residual risk, name it.

---

## Behavioral Contract (The Fixture-Based Baseline)

AID's correctness is defined by its ability to reproduce the expected output for each
reference architecture fixture. The fixtures and expected outputs live in
`tests/fixtures/`. They were derived from the HNP reference implementation.

For any change to the topology kernel, the lead must require:
- All fixture acceptance tests still pass (device count, cable count, per-fabric switch counts)
- `hhfab validate` still passes for all affected fixtures
- BOM totals still match expected values

If a fixture test fails after a kernel change, it is either:
a) A regression (fix the code), or
b) An intentional improvement (update the expected output with explicit justification)

The lead decides which. Never silently update expected outputs.

---

## Red Flags — Stop and Reassess

Stop and reassess if you see:

- The dev agent solving a broader problem than the ticket asked for
- Production code changes during a RED phase
- Tests updated to match code rather than to define the spec
- A Layer 1 component doing I/O (topology calculator touching files or HTTP)
- A Layer 2 adapter querying NetBox directly
- Python appearing in the codebase
- BOM computation requiring a database query
- "Generated by HNP" or any HNP reference in user-facing output or docs
- MoonBit formal proofs deleted or skipped to make a build pass
- The hhfab adapter re-reading plan YAML instead of consuming TopologyIR

---

## HNP Derivation (Development Context Only)

AID's algorithms and behavioral contract are derived from HNP:
`https://github.com/afewell-hh/hh-netbox-plugin`

The key derivation assets are:
- Fixture YAML files in `netbox_hedgehog/test_cases/` — these are AID's acceptance test inputs
- Algorithm implementations in `netbox_hedgehog/services/device_generator.py`
- Rule engine in `netbox_hedgehog/services/transceiver_rules.py`

HNP was never released. It has no users. Do not reference it in any user-facing material.

For full derivation guidance, read `HNP_REFERENCE.md`.

---

## End-State Standard

A good project lead for AID should be able to:
- Get current quickly from git + issues
- Select the right next phase from `ROADMAP.md`
- Write strong tickets using the template
- Enforce the layer boundaries from `ARCHITECTURE.md`
- Catch scope violations before they reach code review
- Verify hhfab validation and formal proof status as part of every GREEN review
- Decide when to merge, defer, or open a follow-up

If a lead can do that reliably, they are ready to run this project.
