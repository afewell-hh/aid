# AID requirements artifacts (owner-provided)

Concrete, owner-provided inputs that the foundation redesign (#46) and the eventual
implementation must satisfy. These are **requirements/test data**, not generated output.

## `real-server-bom.csv`

A real, complete **purchasable** bill of materials for a single GPU server from a live
fabric design. It is the canonical exercise for the object-oriented component-modeling
requirements (R1–R5 on issue #46):

- **Arbitrary line items, physical and non-physical**, that must appear in a complete
  purchasable BOM and scale linearly with server quantity: barebone chassis, warranty
  (`EWCSC`), GPU baseboard, CPUs, memory, drives, NICs, DPU, software support
  (`SVC-NVSTDSWSUP-3Y`), accessory (`CBL-PWEX`), assembly (`MC0037`), onsite service
  (`OSNBD3`).
- **Nested embeddable components with their own attributes**:
  - `8× CX-7 NIC` (`AOC-CX766003N-SQ0`) — each has **one QSFP112 cage @ 400G** that must be
    populated with a user-selected transceiver.
  - `1× BlueField-3 DPU` (`GPU-NVDPU-BA3220-C`) — **one fixed 1000BASE-T BMC port** plus
    **two QSFP112 cages @ 200G**, each requiring a user-selected transceiver.
  - The transceivers for those cages are **not** line items in this flat CSV — AID's model
    must add them as required subcomponents from the device-type catalog.

A complete purchasable BOM AID emits for a topology must be the union of every device's
base line items + their required subcomponents (NICs, DPUs, transceivers) + arbitrary
non-physical line items, scaled by instance counts — and must also be able to reproduce
the (narrower, physical-only) HNP `bom.csv` as a validation subset.
