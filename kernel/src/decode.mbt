// PARTIALLY LEGACY (#84): the TopologyPlan decoder in this file is NON-LIVE
// pre-rebuild surface (no host/Go consumer after D18–D27), retained for quarantine
// and the #38 guard only. The shared JSON primitives (d_obj, d_field, d_str,
// d_field_str, d_arr_or_empty, DecodeErr) ARE live — the F2/F3 decoders reuse them
// — so this file must NOT be deleted wholesale. Retire-vs-reconcile deferred to a
// follow-up.
//
// JSON -> TopologyPlan decoder (the kernel ingress edge, DECISIONS.md D16).
//
// The kernel is pure: this takes a JSON *string* (never a file path) in the
// user-facing plan-schema shape (snake_case) and returns a typed plan, or
// `calc-error::invalid-plan` on malformed/structurally-unusable input. Schema
// validation (ajv against schema/topology-plan-v1.json) stays OUTSIDE the kernel
// at the ingress/test layer; this decoder fails closed on anything it cannot
// turn into a well-formed TopologyPlan.

// ---- legacy-only primitive accessors ------------------------------------
// The shared d_obj / d_field / d_str / d_arr / d_field_str / d_arr_or_empty
// helpers and the DecodeErr suberror moved to json_util.mbt (#85) because the live
// F2/F3 decoders reuse them; the UInt accessors below are used only by this retired
// decoder and go away with it.

fn d_uint(j : Json) -> UInt raise DecodeErr {
  if j is Number(n, ..) {
    n.to_int().reinterpret_as_uint()
  } else {
    raise DecodeErr("expected number")
  }
}

fn d_field_uint(m : Map[String, Json], k : String) -> UInt raise DecodeErr {
  d_uint(d_field(m, k))
}

fn d_opt_str(m : Map[String, Json], k : String) -> String? raise DecodeErr {
  match m.get(k) {
    Some(v) => Some(d_str(v))
    None => None
  }
}

fn d_opt_uint(m : Map[String, Json], k : String) -> UInt? raise DecodeErr {
  match m.get(k) {
    Some(v) => Some(d_uint(v))
    None => None
  }
}

// ---- enum decoders ------------------------------------------------------

fn d_cage(j : Json) -> CageType raise DecodeErr {
  match d_str(j) {
    "osfp" => Osfp
    "qsfp112" => Qsfp112
    "qsfp28" => Qsfp28
    "sfp28" => Sfp28
    "rj45" => Rj45
    s => raise DecodeErr("bad cage_type: " + s)
  }
}

fn d_medium(j : Json) -> Medium raise DecodeErr {
  match d_str(j) {
    "optical" => Optical
    "dac" => Dac
    "aoc" => Aoc
    "copper" => Copper
    s => raise DecodeErr("bad medium: " + s)
  }
}

fn d_fabric_class(j : Json) -> FabricClass raise DecodeErr {
  match d_str(j) {
    "managed" => Managed
    "unmanaged" => Unmanaged
    s => raise DecodeErr("bad fabric_class: " + s)
  }
}

fn d_topology_mode(j : Json) -> TopologyMode raise DecodeErr {
  match d_str(j) {
    "clos" => Clos
    "mesh" => TopologyMode::Mesh
    s => raise DecodeErr("bad topology_mode: " + s)
  }
}

fn d_zone_type(j : Json) -> ZoneType raise DecodeErr {
  match d_str(j) {
    "server" => ZoneType::Server
    "uplink" => Uplink
    "mesh" => ZoneType::Mesh
    "peer" => Peer
    "session" => Session
    "oob" => Oob
    s => raise DecodeErr("bad zone_type: " + s)
  }
}

fn d_allocation(j : Json) -> AllocationStrategy raise DecodeErr {
  match d_str(j) {
    "sequential" => Sequential
    "interleaved" => Interleaved
    "spaced" => Spaced
    s => raise DecodeErr("bad allocation: " + s)
  }
}

fn d_connection_type(j : Json) -> ConnectionType raise DecodeErr {
  match d_str(j) {
    "unbundled" => Unbundled
    "bundled" => Bundled
    "mclag" => ConnectionType::Mclag
    "eslag" => ConnectionType::Eslag
    s => raise DecodeErr("bad connection_type: " + s)
  }
}

fn d_distribution(j : Json) -> Distribution raise DecodeErr {
  match d_str(j) {
    "same-switch" => SameSwitch
    "alternating" => Alternating
    "rail-optimized" => RailOptimized
    s => raise DecodeErr("bad distribution: " + s)
  }
}

fn d_port_type(j : Json) -> PortType raise DecodeErr {
  match d_str(j) {
    "data" => Data
    "ipmi" => Ipmi
    "pxe" => Pxe
    s => raise DecodeErr("bad port_type: " + s)
  }
}

fn d_plan_role(j : Json) -> PlanRole raise DecodeErr {
  match d_str(j) {
    "server" => PlanRole::Server
    "spine" => PlanRole::Spine
    "server-leaf" => ServerLeaf
    "border-leaf" => BorderLeaf
    "oob-leaf" => OobLeaf
    "hhg" => Hhg
    s => raise DecodeErr("bad role: " + s)
  }
}

fn d_plan_status(j : Json) -> PlanStatus raise DecodeErr {
  match d_str(j) {
    "draft" => Draft
    "review" => Review
    "approved" => Approved
    "exported" => Exported
    s => raise DecodeErr("bad status: " + s)
  }
}

// ---- record decoders ----------------------------------------------------

fn d_attribute(j : Json) -> Attribute raise DecodeErr {
  let m = d_obj(j)
  { key: d_field_str(m, "key"), value: d_field_str(m, "value") }
}

fn d_port_spec(j : Json) -> PortSpec raise DecodeErr {
  let m = d_obj(j)
  {
    port_id: d_field_str(m, "port_id"),
    speed_gbps: d_field_uint(m, "speed_gbps"),
    cage: d_cage(d_field(m, "cage_type")),
    medium: d_medium(d_field(m, "medium")),
  }
}

fn d_sub_component(j : Json) -> SubComponent raise DecodeErr {
  let m = d_obj(j)
  {
    slot_id: d_field_str(m, "slot_id"),
    device_class_id: d_field_str(m, "device_class_id"),
    quantity_per_parent: d_field_uint(m, "quantity_per_parent"),
  }
}

fn d_device_class(j : Json) -> DeviceClass raise DecodeErr {
  let m = d_obj(j)
  let attributes = []
  for el in d_arr_or_empty(m, "attributes") {
    attributes.push(d_attribute(el))
  }
  let ports = []
  for el in d_arr_or_empty(m, "ports") {
    ports.push(d_port_spec(el))
  }
  let subs = []
  for el in d_arr_or_empty(m, "sub_components") {
    subs.push(d_sub_component(el))
  }
  {
    id: d_field_str(m, "id"),
    name: d_field_str(m, "name"),
    slug: d_field_str(m, "slug"),
    category: d_field_str(m, "category"),
    manufacturer: d_opt_str(m, "manufacturer"),
    part_number: d_opt_str(m, "part_number"),
    description: d_opt_str(m, "description"),
    attributes,
    ports,
    sub_components: subs,
  }
}

fn d_fabric_domain(j : Json) -> FabricDomain raise DecodeErr {
  let m = d_obj(j)
  let ids = []
  for el in d_arr_or_empty(m, "switch_entry_ids") {
    ids.push(d_str(el))
  }
  {
    fabric_id: d_field_str(m, "fabric_id"),
    fabric_name: d_field_str(m, "fabric_name"),
    fabric_class: d_fabric_class(d_field(m, "fabric_class")),
    topology_mode: d_topology_mode(d_field(m, "topology_mode")),
    switch_entry_ids: ids,
  }
}

fn d_breakout(m : Map[String, Json]) -> BreakoutOption? raise DecodeErr {
  match m.get("breakout") {
    Some(v) => {
      let bm = d_obj(v)
      Some({
        logical_ports: d_field_uint(bm, "logical_ports"),
        logical_speed_gbps: d_field_uint(bm, "logical_speed_gbps"),
      })
    }
    None => None
  }
}

fn d_switch_port_zone(j : Json) -> SwitchPortZone raise DecodeErr {
  let m = d_obj(j)
  {
    zone_id: d_field_str(m, "zone_id"),
    zone_name: d_field_str(m, "zone_name"),
    zone_type: d_zone_type(d_field(m, "zone_type")),
    port_range: d_field_str(m, "port_range"),
    breakout: d_breakout(m),
    priority: d_field_uint(m, "priority"),
    allocation: d_allocation(d_field(m, "allocation")),
    transceiver_intent_id: d_opt_str(m, "transceiver_intent_id"),
    peer_zone_id: d_opt_str(m, "peer_zone_id"),
  }
}

fn d_plan_connection(j : Json) -> PlanConnection raise DecodeErr {
  let m = d_obj(j)
  {
    connection_id: d_field_str(m, "connection_id"),
    nic_slot_id: d_field_str(m, "nic_slot_id"),
    port_index: d_field_uint(m, "port_index"),
    ports_per_connection: d_field_uint(m, "ports_per_connection"),
    connection_type: d_connection_type(d_field(m, "connection_type")),
    distribution: d_distribution(d_field(m, "distribution")),
    target_zone_id: d_field_str(m, "target_zone_id"),
    speed_gbps: d_field_uint(m, "speed_gbps"),
    rail: d_opt_uint(m, "rail"),
    port_type: d_port_type(d_field(m, "port_type")),
    transceiver_intent_id: d_opt_str(m, "transceiver_intent_id"),
  }
}

fn d_redundancy(m : Map[String, Json]) -> Redundancy raise DecodeErr {
  match m.get("redundancy") {
    None => RedNone
    Some(v) => {
      let rm = d_obj(v)
      match d_field_str(rm, "type") {
        "none" => RedNone
        "mclag" => RedMclag({ switch_count: d_field_uint(rm, "switch_count") })
        "eslag" => RedEslag({ switch_count: d_field_uint(rm, "switch_count") })
        s => raise DecodeErr("bad redundancy type: " + s)
      }
    }
  }
}

fn d_opt_topology_mode(m : Map[String, Json]) -> TopologyMode? raise DecodeErr {
  match m.get("topology_mode") {
    Some(v) => Some(d_topology_mode(v))
    None => None
  }
}

fn d_plan_entry(j : Json) -> PlanEntry raise DecodeErr {
  let m = d_obj(j)
  let zones = []
  for el in d_arr_or_empty(m, "port_zones") {
    zones.push(d_switch_port_zone(el))
  }
  let conns = []
  for el in d_arr_or_empty(m, "connections") {
    conns.push(d_plan_connection(el))
  }
  {
    entry_id: d_field_str(m, "entry_id"),
    device_class_id: d_field_str(m, "device_class_id"),
    quantity: d_field_uint(m, "quantity"),
    role: d_plan_role(d_field(m, "role")),
    label: d_opt_str(m, "label"),
    fabric_id: d_opt_str(m, "fabric_id"),
    override_quantity: d_opt_uint(m, "override_quantity"),
    topology_mode: d_opt_topology_mode(m),
    redundancy: d_redundancy(m),
    port_zones: zones,
    connections: conns,
  }
}

fn decode_plan_inner(plan_json : String) -> TopologyPlan raise {
  let j = @json.parse(plan_json)
  let m = d_obj(j)
  let catalog = []
  for el in d_arr_or_empty(m, "device_catalog") {
    catalog.push(d_device_class(el))
  }
  let fabrics = []
  for el in d_arr_or_empty(m, "fabric_domains") {
    fabrics.push(d_fabric_domain(el))
  }
  let entries = []
  for el in d_arr_or_empty(m, "entries") {
    entries.push(d_plan_entry(el))
  }
  {
    id: d_field_str(m, "id"),
    name: d_field_str(m, "name"),
    customer_name: d_field_str(m, "customer_name"),
    status: d_plan_status(d_field(m, "status")),
    device_catalog: catalog,
    fabric_domains: fabrics,
    entries,
  }
}

/// Decode a plan JSON string into a typed TopologyPlan.
pub fn decode_plan(plan_json : String) -> Result[TopologyPlan, CalcError] {
  try Ok(decode_plan_inner(plan_json)) catch {
    _ => Err(InvalidPlan("plan JSON is malformed or structurally invalid"))
  }
}
