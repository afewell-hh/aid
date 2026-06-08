// AID topology kernel — domain & output types.
//
// This file is a hand-authored 1:1 mirror of `wit/types.wit` (interface `types`)
// and the `calc-output` / `calc-error` shapes in `wit/topology-calculator.wit`.
// `wit/` is the single source of truth (DECISIONS.md D16); the
// `kernel/tools/check-types-drift.sh` guard regenerates `wit-bindgen moonbit`
// and fails if a WIT record/enum/variant is missing here.
//
// WIT -> MoonBit mapping (kebab-case identifiers become snake_case):
//   record       -> struct        enum         -> enum (no payload)
//   variant      -> enum (payload) option<T>   -> T?
//   list<T>      -> Array[T]       string       -> String
//   u32          -> UInt          u64          -> UInt64       f64 -> Double   bool -> Bool
//
// NOTE on JSON field names: the JSON wire shape decoded by the kernel uses the
// user-facing *plan schema* snake_case names (schema/topology-plan-v1.json),
// which coincide with these MoonBit field names. See D16.

// ----------------------------------------------------------------------
// Enums (no payload)
// ----------------------------------------------------------------------

pub(all) enum CageType {
  Osfp
  Qsfp112
  Qsfp28
  Sfp28
  Rj45
} derive(Eq)

pub(all) enum Medium {
  Optical
  Dac
  Aoc
  Copper
} derive(Eq)

pub(all) enum FabricClass {
  Managed
  Unmanaged
} derive(Eq)

pub(all) enum TopologyMode {
  Clos
  Mesh
} derive(Eq)

pub(all) enum ZoneType {
  Server
  Uplink
  Mesh
  Peer
  Session
  Oob
} derive(Eq)

pub(all) enum AllocationStrategy {
  Sequential
  Interleaved
  Spaced
} derive(Eq)

pub(all) enum ConnectionType {
  Unbundled
  Bundled
  Mclag
  Eslag
} derive(Eq)

pub(all) enum Distribution {
  SameSwitch
  Alternating
  RailOptimized
} derive(Eq)

pub(all) enum PortType {
  Data
  Ipmi
  Pxe
} derive(Eq)

pub(all) enum PlanRole {
  Server
  Spine
  ServerLeaf
  BorderLeaf
  OobLeaf
  Hhg
} derive(Eq)

pub(all) enum PlanStatus {
  Draft
  Review
  Approved
  Exported
} derive(Eq)

pub(all) enum IssueSeverity {
  Error
  Warning
} derive(Eq)

pub(all) enum NodeType {
  Server
  Switch
  Spine
} derive(Eq)

// ----------------------------------------------------------------------
// DeviceClass — universal recursive hardware/BOM unit (normalized catalog)
// ----------------------------------------------------------------------

pub(all) struct Attribute {
  key : String
  value : String
} derive(Eq)

pub(all) struct PortSpec {
  port_id : String
  speed_gbps : UInt
  cage : CageType
  medium : Medium
} derive(Eq)

pub(all) struct SubComponent {
  slot_id : String
  device_class_id : String
  quantity_per_parent : UInt
} derive(Eq)

pub(all) struct DeviceClass {
  id : String
  name : String
  slug : String
  category : String
  manufacturer : String?
  part_number : String?
  description : String?
  attributes : Array[Attribute]
  ports : Array[PortSpec]
  sub_components : Array[SubComponent]
} derive(Eq)

// ----------------------------------------------------------------------
// Fabric
// ----------------------------------------------------------------------

pub(all) struct FabricDomain {
  fabric_id : String
  fabric_name : String
  fabric_class : FabricClass
  topology_mode : TopologyMode
  switch_entry_ids : Array[String]
} derive(Eq)

// ----------------------------------------------------------------------
// Switch port zones
// ----------------------------------------------------------------------

pub(all) struct BreakoutOption {
  logical_ports : UInt
  logical_speed_gbps : UInt
} derive(Eq)

pub(all) struct SwitchPortZone {
  zone_id : String
  zone_name : String
  zone_type : ZoneType
  port_range : String
  breakout : BreakoutOption?
  priority : UInt
  allocation : AllocationStrategy
  transceiver_intent_id : String?
  peer_zone_id : String?
} derive(Eq)

// ----------------------------------------------------------------------
// Connections
// ----------------------------------------------------------------------

pub(all) struct PlanConnection {
  connection_id : String
  nic_slot_id : String
  port_index : UInt
  ports_per_connection : UInt
  connection_type : ConnectionType
  distribution : Distribution
  target_zone_id : String
  speed_gbps : UInt
  rail : UInt?
  port_type : PortType
  transceiver_intent_id : String?
} derive(Eq)

// ----------------------------------------------------------------------
// Plan entries
// ----------------------------------------------------------------------

pub(all) struct MclagConfig {
  switch_count : UInt
} derive(Eq)

pub(all) struct EslagConfig {
  switch_count : UInt
} derive(Eq)

pub(all) enum Redundancy {
  RedNone
  RedMclag(MclagConfig)
  RedEslag(EslagConfig)
} derive(Eq)

pub(all) struct PlanEntry {
  entry_id : String
  device_class_id : String
  quantity : UInt
  role : PlanRole
  label : String?
  // switch-specific
  fabric_id : String?
  override_quantity : UInt?
  topology_mode : TopologyMode?
  redundancy : Redundancy
  port_zones : Array[SwitchPortZone]
  // server-specific
  connections : Array[PlanConnection]
} derive(Eq)

pub(all) struct TopologyPlan {
  id : String
  name : String
  customer_name : String
  status : PlanStatus
  device_catalog : Array[DeviceClass]
  fabric_domains : Array[FabricDomain]
  entries : Array[PlanEntry]
} derive(Eq)

// ----------------------------------------------------------------------
// Bill of materials (recursive DeviceClass traversal, plan-time)
// ----------------------------------------------------------------------

pub(all) struct DeviceClassSummary {
  id : String
  name : String
  slug : String
  category : String
  manufacturer : String?
  part_number : String?
} derive(Eq)

pub(all) struct BomLineItem {
  path : Array[String]
  device_class : DeviceClassSummary
  quantity_per_parent : UInt
  quantity_per_unit : UInt
  fleet_quantity : UInt
} derive(Eq)

pub(all) struct DeviceClassBom {
  device_class : DeviceClassSummary
  entry_id : String
  plan_quantity : UInt
  line_items : Array[BomLineItem]
} derive(Eq)

// ----------------------------------------------------------------------
// Validation
// ----------------------------------------------------------------------

pub(all) struct ValidationIssue {
  severity : IssueSeverity
  code : String
  message : String
  entry_id : String?
  fabric_id : String?
  zone_id : String?
} derive(Eq)

pub(all) struct ValidationResult {
  is_valid : Bool
  errors : Array[ValidationIssue]
  warnings : Array[ValidationIssue]
} derive(Eq)

// ----------------------------------------------------------------------
// TopologyIR — pure calculator output, stable handoff to export adapters
// ----------------------------------------------------------------------

pub(all) struct TopologyNode {
  node_id : String
  name : String
  node_type : NodeType
  device_class_id : String
  fabric : String?
  hedgehog_role : String?
  instance_index : UInt
} derive(Eq)

pub(all) struct TopologyEdge {
  edge_id : String
  node_a_id : String
  node_b_id : String
  speed_gbps : UInt
  fabric : String
  zone : String
  breakout_index : UInt?
  connection_type : String
  port_a : String
  port_b : String
} derive(Eq)

pub(all) struct FabricSummary {
  fabric_name : String
  switch_count : UInt
  total_server_bandwidth_gbps : UInt64
  total_spine_bandwidth_gbps : UInt64
  oversubscription_ratio : Double
} derive(Eq)

pub(all) struct PlanMetadata {
  plan_id : String
  plan_name : String
  customer_name : String
} derive(Eq)

pub(all) struct TopologyIr {
  metadata : PlanMetadata
  nodes : Array[TopologyNode]
  edges : Array[TopologyEdge]
  fabrics : Array[FabricSummary]
} derive(Eq)

// ----------------------------------------------------------------------
// Calculator output / error (wit/topology-calculator.wit)
// ----------------------------------------------------------------------

pub(all) struct CalcOutput {
  ir : TopologyIr
  boms : Array[DeviceClassBom]
  validation : ValidationResult
} derive(Eq)

pub(all) enum CalcError {
  InvalidPlan(String)
  Internal(String)
} derive(Eq)
