// Package planedit is the server-side structured-editing core for the GUI's
// structured plan editor (D26, Issue #67 / P1.1). It does two things over the
// canonical DIET/training plan YAML:
//
//   - Project: parse the plan into a small JSON projection of the editable
//     fields (server classes incl. joined NICs, switch classes incl. the
//     mesh|clos topology mode) plus the dropdown id lists from reference_data,
//     so the MoonBit UI can render data-derived forms without a YAML parser.
//
//   - Apply: apply structured field edits by SURGICALLY mutating the yaml.Node
//     tree (never unmarshal-to-struct then remarshal — that reorders keys and
//     strips comments on the ~900-line file), then re-validate the result
//     through internal/topology ingest BEFORE returning it. A bad edit returns
//     an error and never yields a plan the caller would persist.
//
// Untouched regions (switch_port_zones, server_connections, reference_data) keep
// their content, key order, and comments faithfully across the round-trip;
// yaml.v3 node encoding normalizes only insignificant whitespace (blank lines).
// The proved kernel/engine is untouched — this is additive Go only.
package planedit

import (
	"bytes"
	"fmt"

	"github.com/afewell-hh/aid/internal/topology"
	"gopkg.in/yaml.v3"
)

// --- Projection (read) ------------------------------------------------------

// Projection is the editable view of a plan the structured form renders from.
type Projection struct {
	ServerClasses []ServerClass `json:"server_classes"`
	SwitchClasses []SwitchClass `json:"switch_classes"`
	Catalog       Catalog       `json:"catalog"`
}

// NIC is one server-class NIC row (joined from the top-level server_nics list).
type NIC struct {
	NicID      string `json:"nic_id"`
	ModuleType string `json:"module_type"`
}

// Connection is one server-class connection row (joined from the top-level
// server_connections list). target_zone is the raw "switch_class/zone_name" join
// (P1.1b, #69) — the form turns it into a dropdown over the plan's valid zones.
//
// Index is the connection's position in the flat server_connections list — the
// stable key edits/removes reference, because connection_id is NOT unique (a
// class can have several connections sharing one id, e.g. compute_xpu's three
// "soc-storage" rows differing by port_index).
type Connection struct {
	Index              int    `json:"index"`
	ConnectionID       string `json:"connection_id"`
	ConnectionName     string `json:"connection_name"`
	TargetZone         string `json:"target_zone"`
	NIC                string `json:"nic"`
	PortsPerConnection int    `json:"ports_per_connection"`
	HedgehogConnType   string `json:"hedgehog_conn_type"`
	Distribution       string `json:"distribution"`
	Speed              int    `json:"speed"`
	Rail               *int   `json:"rail"`
}

type ServerClass struct {
	ID               string       `json:"id"`
	Quantity         int          `json:"quantity"`
	GpusPerServer    int          `json:"gpus_per_server"`
	ServerDeviceType string       `json:"server_device_type"`
	Nics             []NIC        `json:"nics"`
	Connections      []Connection `json:"connections"`
}

type SwitchClass struct {
	ID                  string `json:"id"`
	TopologyMode        string `json:"topology_mode"` // "" | mesh | clos | spine-leaf
	DeviceTypeExtension string `json:"device_type_extension"`
	OverrideQuantity    *int   `json:"override_quantity"` // nil when absent
}

// Catalog carries the dropdown id lists the form binds to (data-derived, never
// hardcoded) — D26 / ticket: reference_data.{module_types, device_types,
// device_type_extensions, breakout_options}.
type Catalog struct {
	ModuleTypes          []string `json:"module_types"`
	DeviceTypes          []string `json:"device_types"`
	DeviceTypeExtensions []string `json:"device_type_extensions"`
	BreakoutOptions      []string `json:"breakout_options"`
	// TargetZones are the valid connection target_zone options — the
	// "switch_class/zone_name" joins from switch_port_zones (P1.1b, #69), so the
	// hand-typed string join becomes a dropdown.
	TargetZones []string `json:"target_zones"`
}

// read structs (projection only; the WRITE path never round-trips through these).
type rawPlan struct {
	ServerClasses []struct {
		ID               string `yaml:"server_class_id"`
		Quantity         int    `yaml:"quantity"`
		GpusPerServer    int    `yaml:"gpus_per_server"`
		ServerDeviceType string `yaml:"server_device_type"`
	} `yaml:"server_classes"`
	SwitchClasses []struct {
		ID                  string `yaml:"switch_class_id"`
		TopologyMode        string `yaml:"topology_mode"`
		DeviceTypeExtension string `yaml:"device_type_extension"`
		OverrideQuantity    *int   `yaml:"override_quantity"`
	} `yaml:"switch_classes"`
	ServerNics []struct {
		ServerClass string `yaml:"server_class"`
		NicID       string `yaml:"nic_id"`
		ModuleType  string `yaml:"module_type"`
	} `yaml:"server_nics"`
	ServerConnections []struct {
		ServerClass        string `yaml:"server_class"`
		ConnectionID       string `yaml:"connection_id"`
		ConnectionName     string `yaml:"connection_name"`
		TargetZone         string `yaml:"target_zone"`
		NIC                string `yaml:"nic"`
		PortsPerConnection int    `yaml:"ports_per_connection"`
		HedgehogConnType   string `yaml:"hedgehog_conn_type"`
		Distribution       string `yaml:"distribution"`
		Speed              int    `yaml:"speed"`
		Rail               *int   `yaml:"rail"`
	} `yaml:"server_connections"`
	SwitchPortZones []struct {
		SwitchClass string `yaml:"switch_class"`
		ZoneName    string `yaml:"zone_name"`
	} `yaml:"switch_port_zones"`
	ReferenceData struct {
		ModuleTypes          []idOnly `yaml:"module_types"`
		DeviceTypes          []idOnly `yaml:"device_types"`
		DeviceTypeExtensions []idOnly `yaml:"device_type_extensions"`
		BreakoutOptions      []idOnly `yaml:"breakout_options"`
	} `yaml:"reference_data"`
}

type idOnly struct {
	ID string `yaml:"id"`
}

func ids(xs []idOnly) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, x.ID)
	}
	return out
}

// Project parses the plan YAML into the editable projection.
func Project(src []byte) (*Projection, error) {
	var rp rawPlan
	if err := yaml.Unmarshal(src, &rp); err != nil {
		return nil, fmt.Errorf("planedit: parse plan: %w", err)
	}
	nicsByClass := map[string][]NIC{}
	for _, n := range rp.ServerNics {
		nicsByClass[n.ServerClass] = append(nicsByClass[n.ServerClass], NIC{NicID: n.NicID, ModuleType: n.ModuleType})
	}
	connsByClass := map[string][]Connection{}
	for i, c := range rp.ServerConnections {
		connsByClass[c.ServerClass] = append(connsByClass[c.ServerClass], Connection{
			Index: i, ConnectionID: c.ConnectionID, ConnectionName: c.ConnectionName, TargetZone: c.TargetZone,
			NIC: c.NIC, PortsPerConnection: c.PortsPerConnection, HedgehogConnType: c.HedgehogConnType,
			Distribution: c.Distribution, Speed: c.Speed, Rail: c.Rail,
		})
	}
	p := &Projection{ServerClasses: []ServerClass{}, SwitchClasses: []SwitchClass{}}
	for _, sc := range rp.ServerClasses {
		nics := nicsByClass[sc.ID]
		if nics == nil {
			nics = []NIC{}
		}
		conns := connsByClass[sc.ID]
		if conns == nil {
			conns = []Connection{}
		}
		p.ServerClasses = append(p.ServerClasses, ServerClass{
			ID: sc.ID, Quantity: sc.Quantity, GpusPerServer: sc.GpusPerServer,
			ServerDeviceType: sc.ServerDeviceType, Nics: nics, Connections: conns,
		})
	}
	for _, sw := range rp.SwitchClasses {
		p.SwitchClasses = append(p.SwitchClasses, SwitchClass{
			ID: sw.ID, TopologyMode: sw.TopologyMode,
			DeviceTypeExtension: sw.DeviceTypeExtension, OverrideQuantity: sw.OverrideQuantity,
		})
	}
	targetZones := make([]string, 0, len(rp.SwitchPortZones))
	for _, z := range rp.SwitchPortZones {
		targetZones = append(targetZones, z.SwitchClass+"/"+z.ZoneName)
	}
	p.Catalog = Catalog{
		ModuleTypes:          ids(rp.ReferenceData.ModuleTypes),
		DeviceTypes:          ids(rp.ReferenceData.DeviceTypes),
		DeviceTypeExtensions: ids(rp.ReferenceData.DeviceTypeExtensions),
		BreakoutOptions:      ids(rp.ReferenceData.BreakoutOptions),
		TargetZones:          targetZones,
	}
	return p, nil
}

// --- Patch (write) ----------------------------------------------------------

// Op is one structured edit. The discriminator is Op; the other fields are read
// per op kind. Values arrive as strings (the form's field values); numeric
// fields are emitted as unquoted YAML scalars so they decode as ints.
type Op struct {
	Op           string `json:"op"`
	ServerClass  string `json:"server_class,omitempty"`
	SwitchClass  string `json:"switch_class,omitempty"`
	NicID        string `json:"nic_id,omitempty"`
	ConnectionID string `json:"connection_id,omitempty"`
	// ConnIndex is the position in server_connections that set/remove target
	// (connection_id is not unique, so index is the stable key, P1.1b/#69).
	ConnIndex int    `json:"conn_index,omitempty"`
	Field     string `json:"field,omitempty"`
	Value        string `json:"value,omitempty"`
	// add_server_class:
	Quantity         string `json:"quantity,omitempty"`
	GpusPerServer    string `json:"gpus_per_server,omitempty"`
	ServerDeviceType string `json:"server_device_type,omitempty"`
	// add_connection: the connection's fields (connection_name, target_zone, nic,
	// ports_per_connection, hedgehog_conn_type, distribution, speed, rail).
	Fields map[string]string `json:"fields,omitempty"`
}

// Patch is the request body for the structured field-patch endpoint.
type Patch struct {
	Ops []Op `json:"ops"`
}

// Apply applies the ops to the plan by surgically editing the yaml.Node tree,
// re-encodes it, and re-validates the result through topology ingest (the D26
// safety invariant). On any op error or a failed re-validate it returns an
// error and the ORIGINAL plan is left for the caller to keep.
func Apply(src []byte, ops []Op) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("planedit: parse plan: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("planedit: plan is not a YAML mapping")
	}
	root := doc.Content[0]
	for i, op := range ops {
		if err := applyOne(root, op); err != nil {
			return nil, fmt.Errorf("planedit: op[%d] %q: %w", i, op.Op, err)
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("planedit: re-encode plan: %w", err)
	}
	_ = enc.Close()
	out := buf.Bytes()

	// Safety invariant (D26): the edited plan MUST still ingest. A bad edit is
	// rejected here — never persisted.
	if _, _, err := topology.IngestBundled(out); err != nil {
		return nil, fmt.Errorf("planedit: edited plan fails validation: %w", err)
	}
	return out, nil
}

func applyOne(root *yaml.Node, op Op) error {
	switch op.Op {
	case "set_server_field":
		sc, err := findInSeq(root, "server_classes", "server_class_id", op.ServerClass)
		if err != nil {
			return err
		}
		switch op.Field {
		case "quantity", "gpus_per_server":
			setScalar(sc, op.Field, op.Value)
		case "server_device_type":
			// A structured edit must not leave a server class without a real
			// device type (devb #67 finding: the ingest guard alone permits it).
			if err := validateDeviceType(root, op.Value); err != nil {
				return err
			}
			setScalar(sc, op.Field, op.Value)
		default:
			return fmt.Errorf("unsupported server field %q", op.Field)
		}
	case "set_nic_module_type":
		nic, err := findNic(root, op.ServerClass, op.NicID)
		if err != nil {
			return err
		}
		setScalar(nic, "module_type", op.Value)
	case "set_switch_field":
		sw, err := findInSeq(root, "switch_classes", "switch_class_id", op.SwitchClass)
		if err != nil {
			return err
		}
		switch op.Field {
		case "topology_mode", "device_type_extension":
			setScalar(sw, op.Field, op.Value)
		case "override_quantity":
			if op.Value == "" {
				deleteKey(sw, "override_quantity") // clear the override → derived
			} else {
				setScalar(sw, "override_quantity", op.Value)
			}
		default:
			return fmt.Errorf("unsupported switch field %q", op.Field)
		}
	case "add_server_class":
		return addServerClass(root, op)
	case "set_connection_field":
		conn, err := connectionAt(root, op.ConnIndex)
		if err != nil {
			return err
		}
		switch op.Field {
		case "target_zone":
			if err := validateTargetZone(root, op.Value); err != nil {
				return err
			}
			setScalar(conn, op.Field, op.Value)
		case "nic":
			scID := ""
			if sc := mapValue(conn, "server_class"); sc != nil {
				scID = sc.Value
			}
			if err := validateNic(root, scID, op.Value); err != nil {
				return err
			}
			setScalar(conn, op.Field, op.Value)
		case "connection_name", "hedgehog_conn_type", "distribution", "speed", "ports_per_connection", "rail":
			setScalar(conn, op.Field, op.Value)
		default:
			return fmt.Errorf("unsupported connection field %q", op.Field)
		}
	case "add_connection":
		return addConnection(root, op)
	case "remove_connection":
		return removeConnectionAt(root, op.ConnIndex)
	default:
		return fmt.Errorf("unknown op")
	}
	return nil
}

// --- yaml.Node helpers ------------------------------------------------------

// mapValue returns the value node for key in a mapping node, or nil.
func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// setScalar sets (or appends) key=value as a plain scalar (untyped tag so YAML
// infers int/bool/string), preserving any existing comments on the value node.
func setScalar(m *yaml.Node, key, value string) {
	if v := mapValue(m, key); v != nil {
		v.Kind = yaml.ScalarNode
		v.Tag = ""
		v.Style = 0
		v.Value = value
		v.Content = nil
		return
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
}

// deleteKey removes a key/value pair from a mapping node (no-op if absent).
func deleteKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

// deviceTypeIDs collects the reference_data.device_types ids (the server-device
// dropdown source) from the plan node tree.
func deviceTypeIDs(root *yaml.Node) map[string]bool {
	out := map[string]bool{}
	rd := mapValue(root, "reference_data")
	if rd == nil {
		return out
	}
	dts := mapValue(rd, "device_types")
	if dts == nil || dts.Kind != yaml.SequenceNode {
		return out
	}
	for _, item := range dts.Content {
		if v := mapValue(item, "id"); v != nil {
			out[v.Value] = true
		}
	}
	return out
}

// validateDeviceType rejects a blank or unknown server_device_type so the
// structured editor cannot store a semantically incomplete server class.
func validateDeviceType(root *yaml.Node, value string) error {
	if value == "" {
		return fmt.Errorf("server_device_type is required")
	}
	if !deviceTypeIDs(root)[value] {
		return fmt.Errorf("server_device_type %q is not a known device type", value)
	}
	return nil
}

// findInSeq finds the mapping item in root[seqKey] whose idKey == idVal.
func findInSeq(root *yaml.Node, seqKey, idKey, idVal string) (*yaml.Node, error) {
	seq := mapValue(root, seqKey)
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("no %s sequence", seqKey)
	}
	for _, item := range seq.Content {
		if v := mapValue(item, idKey); v != nil && v.Value == idVal {
			return item, nil
		}
	}
	return nil, fmt.Errorf("%s %q not found", idKey, idVal)
}

// findNic finds the server_nics row for (server_class, nic_id).
func findNic(root *yaml.Node, serverClass, nicID string) (*yaml.Node, error) {
	seq := mapValue(root, "server_nics")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("no server_nics sequence")
	}
	for _, item := range seq.Content {
		sc := mapValue(item, "server_class")
		nid := mapValue(item, "nic_id")
		if sc != nil && sc.Value == serverClass && nid != nil && nid.Value == nicID {
			return item, nil
		}
	}
	return nil, fmt.Errorf("nic %q on server_class %q not found", nicID, serverClass)
}

// addServerClass appends a new server class mapping to the server_classes seq.
func addServerClass(root *yaml.Node, op Op) error {
	seq := mapValue(root, "server_classes")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return fmt.Errorf("no server_classes sequence")
	}
	if op.ServerClass == "" {
		return fmt.Errorf("add_server_class needs a server_class id")
	}
	// A new server class must carry a real device type — never store a
	// semantically incomplete class (devb #67 finding).
	if err := validateDeviceType(root, op.ServerDeviceType); err != nil {
		return err
	}
	// reject a duplicate id up front (clearer than a downstream ingest error).
	for _, item := range seq.Content {
		if v := mapValue(item, "server_class_id"); v != nil && v.Value == op.ServerClass {
			return fmt.Errorf("server class %q already exists", op.ServerClass)
		}
	}
	q := op.Quantity
	if q == "" {
		q = "1"
	}
	g := op.GpusPerServer
	if g == "" {
		g = "0"
	}
	item := &yaml.Node{Kind: yaml.MappingNode}
	add := func(k, v string) {
		item.Content = append(item.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v},
		)
	}
	add("server_class_id", op.ServerClass)
	add("quantity", q)
	add("gpus_per_server", g)
	add("server_device_type", op.ServerDeviceType)
	seq.Content = append(seq.Content, item)
	return nil
}

// --- connections (P1.1b, #69) -----------------------------------------------

// targetZoneIDs collects the valid "switch_class/zone_name" joins from
// switch_port_zones (the connection target_zone dropdown source).
func targetZoneIDs(root *yaml.Node) map[string]bool {
	out := map[string]bool{}
	zones := mapValue(root, "switch_port_zones")
	if zones == nil || zones.Kind != yaml.SequenceNode {
		return out
	}
	for _, item := range zones.Content {
		sc := mapValue(item, "switch_class")
		zn := mapValue(item, "zone_name")
		if sc != nil && zn != nil {
			out[sc.Value+"/"+zn.Value] = true
		}
	}
	return out
}

// validateTargetZone rejects a blank or unknown target_zone so a connection can
// never point at a non-existent switch_class/zone (it would dangle at ingest).
func validateTargetZone(root *yaml.Node, value string) error {
	if value == "" {
		return fmt.Errorf("target_zone is required")
	}
	if !targetZoneIDs(root)[value] {
		return fmt.Errorf("target_zone %q is not a valid switch_class/zone_name", value)
	}
	return nil
}

// serverNicIDs collects the nic_id set declared for a server_class in server_nics
// (a connection's nic must reference one of these).
func serverNicIDs(root *yaml.Node, serverClass string) map[string]bool {
	out := map[string]bool{}
	seq := mapValue(root, "server_nics")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return out
	}
	for _, item := range seq.Content {
		sc := mapValue(item, "server_class")
		nid := mapValue(item, "nic_id")
		if sc != nil && sc.Value == serverClass && nid != nil {
			out[nid.Value] = true
		}
	}
	return out
}

// validateNic rejects a blank or dangling connection nic — it must reference a
// NIC the connection's server_class actually declares (devb #69 finding: the
// ingest guard does not catch this; it is a calc-time check).
func validateNic(root *yaml.Node, serverClass, value string) error {
	if value == "" {
		return fmt.Errorf("nic is required")
	}
	if !serverNicIDs(root, serverClass)[value] {
		return fmt.Errorf("nic %q is not a NIC of server_class %q", value, serverClass)
	}
	return nil
}

// connectionAt returns the server_connections row at index idx (the stable key,
// since connection_id is not unique).
func connectionAt(root *yaml.Node, idx int) (*yaml.Node, error) {
	seq := mapValue(root, "server_connections")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("no server_connections sequence")
	}
	if idx < 0 || idx >= len(seq.Content) {
		return nil, fmt.Errorf("connection index %d out of range (0..%d)", idx, len(seq.Content)-1)
	}
	return seq.Content[idx], nil
}

// addConnection appends a new connection to server_connections. It requires a
// valid target_zone; connection_id need not be unique (it isn't in the schema).
// port_index/port_type/etc. get sane defaults so the result ingests; the
// re-validate guard (Apply) is the backstop.
func addConnection(root *yaml.Node, op Op) error {
	seq := mapValue(root, "server_connections")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return fmt.Errorf("no server_connections sequence")
	}
	if op.ServerClass == "" || op.ConnectionID == "" {
		return fmt.Errorf("add_connection needs server_class and connection_id")
	}
	f := op.Fields
	if f == nil {
		f = map[string]string{}
	}
	if err := validateTargetZone(root, f["target_zone"]); err != nil {
		return err
	}
	if err := validateNic(root, op.ServerClass, f["nic"]); err != nil {
		return err
	}
	defaulted := func(k, dflt string) string {
		if v, ok := f[k]; ok && v != "" {
			return v
		}
		return dflt
	}
	item := &yaml.Node{Kind: yaml.MappingNode}
	add := func(k, v string) {
		item.Content = append(item.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v},
		)
	}
	add("server_class", op.ServerClass)
	add("connection_id", op.ConnectionID)
	add("connection_name", defaulted("connection_name", op.ConnectionID))
	add("nic", f["nic"])
	add("port_index", defaulted("port_index", "0"))
	add("ports_per_connection", defaulted("ports_per_connection", "1"))
	add("hedgehog_conn_type", defaulted("hedgehog_conn_type", "unbundled"))
	add("distribution", defaulted("distribution", "same-switch"))
	add("target_zone", f["target_zone"])
	add("speed", defaulted("speed", "0"))
	add("port_type", defaulted("port_type", "data"))
	if r, ok := f["rail"]; ok && r != "" {
		add("rail", r)
	}
	seq.Content = append(seq.Content, item)
	return nil
}

// removeConnectionAt deletes the server_connections row at index idx.
func removeConnectionAt(root *yaml.Node, idx int) error {
	seq := mapValue(root, "server_connections")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return fmt.Errorf("no server_connections sequence")
	}
	if idx < 0 || idx >= len(seq.Content) {
		return fmt.Errorf("connection index %d out of range (0..%d)", idx, len(seq.Content)-1)
	}
	seq.Content = append(seq.Content[:idx], seq.Content[idx+1:]...)
	return nil
}
