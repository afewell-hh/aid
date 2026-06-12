# AID Schemas

Canonical, versioned JSON Schemas (Draft 2020-12) for the rebuilt AID foundation
(see `docs/foundation-redesign.md`, decision records **D18–D21**). They are the
**language-neutral wire contracts**; the F0 Go types in `internal/{topology,
catalog,objectmodel}` are the first consumer, and later phases (the MoonBit calc
kernel, the Rust/Go reducers) consume the same schemas.

| File | Contents |
|------|----------|
| `topology-plan-v2.json` | The topology-plan shape: the real diet/XOC 9-section form **plus** a Kubernetes-style `spec`/`status` plane. Validates the real reference files (`training_*.yaml`, `topology-plan.yaml`). |
| `catalog-v1.json` | AID's separate, NetBox-independent **component catalog**: two layers (bare hardware *types* vs configured server/switch *classes*), `calc_profile`/`purchase_profile` attribute planes, `component_slots`, `port_templates`/cage templates, `bom_line_templates`. |
| `superseded/topology-plan-v1.json` | **Retired.** The invented v1 plan schema, kept for history. Superseded by `topology-plan-v2.json` (D18). |

## Model summary

- **Topology plan** (`topology-plan-v2.json`) is **pointers + intent**: it
  references catalog **classes** by pinned id and carries the plan-specific parts
  — `quantity` per class and **per-NIC-port** connection intent. Inventory detail
  lives in the separate catalog. The `expected`/`status` plane is a self-check
  oracle only — it never drives production calculation (guardrail 3). A real
  *bundled* file co-locating `reference_data` is accepted at the ingest boundary
  and split losslessly into the catalog (guardrail 2; D21).
- **Catalog** (`catalog-v1.json`) is a separate, versioned, AID-owned artifact of
  independent objects (CRD-style) referenced by pinned id+version (guardrail 1).
  Bare hardware **types** declare capability only; configured **server/switch
  classes** bind specific transceivers into specific NIC-port cages and are
  reusable inventory objects with complete, context-free BOMs (a different optic
  ⇒ a distinct class).

The object substrate, the two layers, and the validation contracts (stable IDs,
required-fields-per-projection, quantity composition, `component_slots`
acyclicity, clear errors) are specified in `docs/foundation-redesign.md` §4.2–§4.5
and enforced by `internal/objectmodel`.

## Validating

Plans/catalog are authored as YAML and validated as JSON against these schemas.
Any Draft 2020-12 validator works; AID validates via `internal/planschema`
(F0 GREEN wires the validator). Syntactic check of a schema document:

```bash
jq empty schema/topology-plan-v2.json schema/catalog-v1.json
```

## Semantic validation deferred to AID

JSON Schema enforces structure, required fields, enums, numeric bounds, and
patterns. Whole-document/graph checks — referential integrity of catalog refs,
catalog identity/version pinning, `component_slots` acyclicity, quantity
composition, per-NIC-port binding consistency, and all capacity/calculation
feasibility — are enforced by AID (`internal/objectmodel` contracts + the calc
kernel), not by the schema.
