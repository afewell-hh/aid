// F2 calculation kernel — re-derived calc-plan / calc-output types.
//
// These mirror the approved boundary contract in
// docs/foundation/f2-architecture-note.md §1.1 (Issue #52). The calc-plan is the
// FLAT, NUMERIC, catalog-resolved input Go hands the kernel (Go resolves
// breakout_option -> logical_ports and the optic attribute_data); the kernel
// parses the raw `port_spec` string and stays catalog-free (D16). The calc-output
// carries the oracle quantities PLUS one record per REALIZED ENDPOINT (per server
// instance × port) so distribution is faithfully re-derivable and F4 can build
// wiring without re-running it (devb review finding 1).
//
// Mesh-link pairing/cabling is deferred to F4 (note §2.6); F2 keeps only the
// 2-or-3 structural gate. No mesh-link field appears here.
//
// F2 RED: types + stubs only; the bodies are filled in GREEN.

// ----------------------------------------------------------------------
// calc-plan (Go -> kernel)
// ----------------------------------------------------------------------

/// The optic `attribute_data` Go resolved from the catalog, passed to the kernel
/// so it can evaluate transceiver verdicts WITHOUT touching the catalog (note
/// §1.1, §2.5; transceiver_rules.py:160-256). The fields are exactly the keys the
/// verdict rules read: `medium` (mismatch BLOCKS), `cage_type` / `connector`
/// (mismatch → needs_review). An empty string means "unspecified" for that key;
/// the option itself being `None` means the end carries no optic intent.
/// (Cable-assembly `far_end_*` attrs extend this in GREEN if a fixture needs them.)
pub(all) struct XcvrAttrs {
  medium : String
  cage_type : String
  connector : String
} derive(Eq, Show)

/// A port-allocation zone on a switch class. `port_spec` is the RAW string the
/// kernel parses (comma-list + range + `start-end:step`); `breakout_logical_ports`
/// is resolved by Go from the catalog `breakout_option`. `transceiver_attrs` is
/// the zone-side (switch-end) optic, resolved by Go (note §1.1).
pub(all) struct ZoneIn {
  zone_name : String
  zone_type : String
  port_spec : String
  breakout_logical_ports : Int
  allocation_strategy : String
  transceiver_attrs : XcvrAttrs?
} derive(Eq, Show)

/// A switch class with its derivation inputs. `override_quantity` is `None` on the
/// derived path; `redundancy` ∈ {none,mclag,eslag}; `topology_mode` ∈ {clos,mesh}.
pub(all) struct SwitchClassIn {
  switch_class_id : String
  override_quantity : Int?
  redundancy : String
  topology_mode : String
  zones : Array[ZoneIn]
} derive(Eq, Show)

pub(all) struct ServerClassIn {
  server_class_id : String
  quantity : Int
} derive(Eq, Show)

/// A connection definition with FULL per-connection identity (devb finding 1):
/// `connection_id`, `nic_slot_id`, `port_index`, and `speed` are preserved so
/// `alternating` (which keys on `port_index`) and per-instance fan-out are
/// faithfully re-derivable. One definition fans out across `server_quantity`
/// server instances. `rail` is `None` unless `distribution == rail-optimized`.
pub(all) struct ConnIn {
  connection_id : String
  server_class_id : String
  server_quantity : Int
  nic_slot_id : String
  port_index : Int
  ports_per_connection : Int
  speed : Int
  distribution : String
  rail : Int?
  target_switch_class : String
  target_zone : String
  // The server-side (connection-end) optic, resolved by Go. Paired against the
  // target zone's `transceiver_attrs` to produce a transceiver verdict (§2.5).
  server_transceiver_attrs : XcvrAttrs?
} derive(Eq, Show)

pub(all) struct CalcPlan {
  switch_classes : Array[SwitchClassIn]
  server_classes : Array[ServerClassIn]
  connections : Array[ConnIn]
} derive(Eq, Show)

// ----------------------------------------------------------------------
// calc-output (kernel -> Go)
// ----------------------------------------------------------------------

/// One allocated switch port. `breakout_index` is `None` for a non-breakout port
/// (name `E1/{port}`) or the lane `1..N` for a breakout port (name
/// `E1/{port}/{lane}`).
pub(all) struct PortSlot {
  physical_port : Int
  breakout_index : Int?
  name : String
} derive(Eq, Show)

/// One realized endpoint: which server INSTANCE got which switch INSTANCE and
/// port slot, for one connection port. This is the per-instance identity the
/// boundary must preserve.
pub(all) struct Endpoint {
  server_class_id : String
  server_index : Int
  connection_id : String
  nic_slot_id : String
  port_index : Int
  switch_class_id : String
  switch_index : Int
  zone : String
  port_slot : PortSlot
} derive(Eq, Show)

/// A (class_id, quantity) pair — the headline oracle row, as a list so the output
/// stays catalog-free and order-deterministic.
pub(all) struct ClassQty {
  class_id : String
  quantity : Int
} derive(Eq, Show)

pub(all) struct Verdict {
  connection_id : String
  outcome : String
  reason_code : String
} derive(Eq, Show)

pub(all) struct CalcOutput2 {
  switch_quantity : Array[ClassQty]
  server_quantity : Array[ClassQty]
  endpoints : Array[Endpoint]
  transceiver_verdicts : Array[Verdict]
} derive(Eq, Show)
