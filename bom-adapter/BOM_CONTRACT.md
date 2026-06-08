# Layer-1 → Layer-2 BOM wire contract

Per the D16 extension to Layer 2 (approved on issue #9, recorded in
`DECISIONS.md`), the **snake_case JSON shape of `device-class-bom[]`** is the
single-sourced wire contract between the kernel (Layer 1) and this adapter
(Layer 2). It is the exact byte shape the Phase-6 Go host will hand the adapter,
and the shape `src/bom.rs` deserializes.

There is **one** source of truth for the type shapes: **`wit/types.wit`**
(`device-class-bom`, `bom-line-item`, `device-class-summary`). This document
records how the JSON realizes those WIT records. The interim encoder lives in
`tools/bom-gen/` (runs the merged Phase-3 kernel `calculate()` and serializes
the `boms` slice — no `kernel/` edits); the Phase-6 kernel boundary encoder must
emit these same bytes (flagged there for consolidation — `bom-gen` is interim
test tooling, not the production boundary).

> **The adapter RENDERS this data.** It does not recompute the BOM or re-derive
> the role-based root-inclusion rule (Algorithm 6 — server entries omit the root
> SKU, switch entries include it). The kernel already applied that rule; the
> `path: []` root line is present on switch BOMs and absent on server BOMs, and
> the adapter prints whatever it is given.

## Conventions

- **Field names:** `snake_case` (WIT records are `kebab-case`; the JSON wire form
  mirrors the user-facing plan-schema convention used throughout AID — see D16).
- **`option<T>`:** JSON `null` when absent, else the bare value.
- **Integers:** every quantity is a non-negative integer token (`u32`).
  Whitespace is insignificant (vendored files are pretty-printed for review;
  the contract is the field shape, not the byte layout).

## Mapping (`wit/types.wit` → JSON)

`device-class-bom`:

| WIT field       | JSON key        | Type                              |
|-----------------|-----------------|-----------------------------------|
| `device-class`  | `device_class`  | `device-class-summary` (object)   |
| `entry-id`      | `entry_id`      | string                            |
| `plan-quantity` | `plan_quantity` | u32                               |
| `line-items`    | `line_items`    | array of `bom-line-item`          |

`bom-line-item`:

| WIT field             | JSON key              | Type                            | Notes                                          |
|-----------------------|-----------------------|---------------------------------|------------------------------------------------|
| `path`                | `path`                | array of string                 | slot path from root; `[]` for the root SKU line |
| `device-class`        | `device_class`        | `device-class-summary` (object) |                                                |
| `quantity-per-parent` | `quantity_per_parent` | u32                             |                                                |
| `quantity-per-unit`   | `quantity_per_unit`   | u32                             | product of `quantity_per_parent` along `path`  |
| `fleet-quantity`      | `fleet_quantity`      | u32                             | `quantity_per_unit * plan_quantity`            |

`device-class-summary`:

| WIT field      | JSON key       | Type            |
|----------------|----------------|-----------------|
| `id`           | `id`           | string          |
| `name`         | `name`         | string          |
| `slug`         | `slug`         | string          |
| `category`     | `category`     | string          |
| `manufacturer` | `manufacturer` | string \| null  |
| `part-number`  | `part_number`  | string \| null  |

## Adapter output shapes

The adapter emits `bom-output { format, content }` (`wit/bom-adapter.wit`). The
`content` string is either CSV text or JSON text per `format`.

### CSV (`format: csv`)

One document for all input BOMs — a single flat table, rows contiguous per
`entry_id` (groups visually in a spreadsheet; cleanly parseable). Line-items
preserved in the kernel's emitted order (pre-order DFS: switch root SKU first,
then its sub-components — parent before child, never re-sorted). Header:

```
entry_id,plan_quantity,level,path,slug,name,category,manufacturer,part_number,quantity_per_parent,quantity_per_unit,fleet_quantity
```

- `level` = `path.length` (`0` = root SKU line — switch entries only; `1` = a
  direct sub-component; …) — the tree depth.
- `path` = the slot path joined with `/` (e.g. `nic-fe/xcvr`); empty for the
  root line.
- `manufacturer` / `part_number` empty cell when `null`.
- **`include_fleet_totals` toggles only the `fleet_quantity` column** (dropped
  from header + rows when `false`). `plan_quantity` and the per-unit columns are
  always present.

### JSON (`format: json`)

`content` is a JSON document (snake_case). Per-unit fields always present;
`fleet_quantity` omitted from each line-item when `include_fleet_totals=false`.

```json
{
  "include_fleet_totals": true,
  "boms": [
    {
      "entry_id": "gpu-servers",
      "plan_quantity": 4,
      "device_class": { "id": "...", "name": "...", "slug": "...", "category": "...",
                        "manufacturer": null, "part_number": null },
      "line_items": [
        { "path": ["nic-fe", "xcvr"], "level": 2,
          "slug": "osfp-400g-xcvr", "name": "OSFP 400G Transceiver", "category": "transceiver",
          "manufacturer": null, "part_number": null,
          "quantity_per_parent": 1, "quantity_per_unit": 2, "fleet_quantity": 8 }
      ]
    }
  ]
}
```

## Regenerating the vendored BOM test data

```sh
bom-adapter/tools/gen-bom.sh
```

Runs `aid/kernel` `calculate()` (via `tools/bom-gen`, local-path dep on the
kernel) over each valid fixture's embedded plan JSON and writes
`tests/testdata/<fixture>.boms.json`. Never hand-edit those files.
