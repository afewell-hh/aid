// Package wiring is AID's F4 hhfab wiring renderer
// (docs/foundation-redesign.md §2.5; docs/foundation/f4-architecture-note.md;
// Issue #60; D22/D23). It is a PURE transform of the F2 IR (calc.CalcOutput) +
// the ingested topology plan + the (overlay-merged) catalog into hhfab wiring
// CRDs (wiring.githedgehog.com/v1beta1 + vpc.githedgehog.com/v1beta1), one
// document per managed fabric (grouped by fabric_name — note §2.1). No new
// topology calc: it consumes F2's per-(switch,zone) endpoints and only pairs the
// mesh-zone ports F2 defers to F4 (note §2.6). D22: wiring only — no
// netbox_inventory.json. No empty `ecmp: {}` (the field is omitted entirely).
//
// The structural-equivalence bar (note §3B) is enforced by
// internal/oracle.CompareWiringHhfab + the hhfab-validate hard gate.
package wiring

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/topology"
)

const (
	wiringAPI = "wiring.githedgehog.com/v1beta1"
	vpcAPI    = "vpc.githedgehog.com/v1beta1"
	namespace = "default"
)

// Doc is one managed fabric's wiring YAML — the unit of output. Fabric is the
// managed fabric_name (e.g. "soc-storage-scale-out"); YAML is the multi-document
// CRD stream that file (`wiring-{Fabric}.yaml`) would contain.
type Doc struct {
	Fabric string
	YAML   []byte
}

// Render transforms the F2 IR + plan + catalog into one wiring Doc per managed
// fabric_name (note §2.1). It is a pure function of its inputs — no calc, no
// catalog mutation, no I/O. The catalog is expected to be overlay-merged so the
// switch Item.Model resolves to the hhfab profile (note §2.4).
func Render(plan *topology.Plan, cat *catalog.Catalog, calcOut *calc.CalcOutput) ([]Doc, error) {
	if plan == nil || cat == nil || calcOut == nil {
		return nil, fmt.Errorf("wiring: Render needs a plan, catalog and calc-output")
	}

	switchQty := map[string]int{}
	for _, q := range calcOut.SwitchQuantity {
		switchQty[q.ClassID] = q.Quantity
	}
	breakouts := parseBreakouts(cat.ReferenceData())

	// Managed switch classes grouped by fabric_name; class metadata resolved once.
	type swClass struct {
		id, fabric, role, profile string
		redType, redGroup         string
		qty                       int
		zones                     []topology.SwitchPortZone
	}
	classByID := map[string]swClass{}
	classesByFabric := map[string][]string{}
	for _, sw := range plan.Spec.SwitchClasses {
		if sw.FabricClass != "managed" {
			continue // unmanaged fabrics (e.g. oob) are not rendered as hhfab wiring
		}
		item, _ := cat.Get(sw.ClassRef.ID)
		classByID[sw.SwitchClassID] = swClass{
			id: sw.SwitchClassID, fabric: sw.FabricName, role: sw.HedgehogRole,
			redType: sw.RedundancyType, redGroup: sw.RedundancyGroup,
			profile: item.Model, qty: switchQty[sw.SwitchClassID],
		}
		classesByFabric[sw.FabricName] = append(classesByFabric[sw.FabricName], sw.SwitchClassID)
	}
	for cid := range classByID {
		c := classByID[cid]
		for _, z := range plan.Spec.PortZones {
			if z.SwitchClassID == cid {
				c.zones = append(c.zones, z)
			}
		}
		classByID[cid] = c
	}

	// Server class catalog items (for NIC interface-name resolution).
	serverItemByClass := map[string]catalog.Item{}
	for _, sc := range plan.Spec.ServerClasses {
		if it, ok := cat.Get(sc.ClassRef.ID); ok {
			serverItemByClass[sc.ServerClassID] = it
		}
	}

	var docs []Doc
	for _, fabric := range sortedKeys(classesByFabric) {
		inFabric := map[string]bool{}
		for _, cid := range classesByFabric[fabric] {
			inFabric[cid] = true
		}

		var objs []any
		objs = append(objs, vlanNamespaceCRD(), ipv4NamespaceCRD())

		// SwitchGroups — one per distinct redundancy_group among the fabric's
		// member classes (MCLAG; e.g. fe-mclag). Emitted before the switches that
		// reference them, mirroring the committed wiring layout (note §2.4).
		var groups []string
		seenGroup := map[string]bool{}
		for _, cid := range classesByFabric[fabric] {
			if g := classByID[cid].redGroup; g != "" && !seenGroup[g] {
				seenGroup[g] = true
				groups = append(groups, g)
			}
		}
		sort.Strings(groups)
		for _, g := range groups {
			objs = append(objs, switchGroupCRD(g))
		}

		// Switches — one per instance of each member class, sorted by device name.
		type swInst struct {
			name string
			cls  swClass
			idx  int
		}
		var switches []swInst
		for _, cid := range classesByFabric[fabric] {
			c := classByID[cid]
			for i := 0; i < c.qty; i++ {
				switches = append(switches, swInst{name: switchDevName(cid, i), cls: c, idx: i})
			}
		}
		sort.Slice(switches, func(i, j int) bool { return switches[i].name < switches[j].name })
		for _, s := range switches {
			objs = append(objs, switchCRD(s.name, switchBootName(s.cls.id, s.idx), s.cls.profile, s.cls.role, s.cls.redType, s.cls.redGroup, s.cls.zones, breakouts))
		}

		// Servers + unbundled Connections — one per F2 endpoint on this fabric.
		serverSeen := map[string]bool{}
		var serverNames []string
		type uconn struct{ name, serverPort, switchPort string }
		var uconns []uconn
		for _, e := range calcOut.Endpoints {
			if !inFabric[e.SwitchClassID] {
				continue
			}
			serverDev := serverDevName(e.ServerClassID, e.ServerIndex)
			if !serverSeen[serverDev] {
				serverSeen[serverDev] = true
				serverNames = append(serverNames, serverDev)
			}
			switchDev := switchDevName(e.SwitchClassID, e.SwitchIndex)
			iface := nicIfaceName(serverItemByClass[e.ServerClassID], cat, e.NICSlotID, e.PortIndex)
			serverPort := serverDev + "/" + e.NICSlotID + "-" + iface
			switchPort := switchDev + "/" + e.PortSlot.Name
			name := trunc63(sanitize(serverDev+"-"+e.NICSlotID+"-"+iface) + "--unbundled--" + sanitize(switchDev))
			uconns = append(uconns, uconn{name: name, serverPort: serverPort, switchPort: switchPort})
		}
		sort.Strings(serverNames)
		for _, n := range serverNames {
			objs = append(objs, serverCRD(n))
		}
		sort.Slice(uconns, func(i, j int) bool { return uconns[i].name < uconns[j].name })
		for _, u := range uconns {
			objs = append(objs, unbundledCRD(u.name, u.serverPort, u.switchPort))
		}

		// Mesh Connections — F4 pairs the mesh-zone ports F2 defers (note §2.6).
		for _, cid := range classesByFabric[fabric] {
			c := classByID[cid]
			objs = append(objs, meshCRDs(cid, c.qty, c.zones)...)
		}

		// Fabric Connections — Clos leaf↔spine links (F6 §1.5/§2.4). Each leaf's
		// uplink ports are split into S contiguous groups (S = spine instances),
		// one group per spine; each spine fills its fabric-zone downlinks with a
		// per-spine cursor in leaf order. One Connection per (spine, leaf) pair.
		type leafInst struct {
			dev     string
			uplinks []int
		}
		type spineInst struct {
			dev       string
			downlinks []int
		}
		var leaves []leafInst
		var spines []spineInst
		for _, cid := range classesByFabric[fabric] {
			c := classByID[cid]
			switch {
			case isLeafRole(c.role):
				up := zonePorts(c.zones, "uplink")
				for i := 0; i < c.qty; i++ {
					leaves = append(leaves, leafInst{dev: switchDevName(cid, i), uplinks: up})
				}
			case c.role == "spine":
				down := zonePorts(c.zones, "fabric")
				for i := 0; i < c.qty; i++ {
					spines = append(spines, spineInst{dev: switchDevName(cid, i), downlinks: append([]int(nil), down...)})
				}
			}
		}
		sort.Slice(leaves, func(i, j int) bool { return leaves[i].dev < leaves[j].dev })
		sort.Slice(spines, func(i, j int) bool { return spines[i].dev < spines[j].dev })
		if s := len(spines); s > 0 {
			cursor := make([]int, s) // per-spine downlink cursor, advanced in leaf order
			for k := 0; k < s; k++ {
				sp := spines[k]
				for _, lf := range leaves {
					pps := len(lf.uplinks) / s // ports this leaf devotes to each spine
					if pps == 0 {
						continue
					}
					var links []any
					for n := 0; n < pps; n++ {
						leafPort := "E1/" + strconv.Itoa(lf.uplinks[k*pps+n])
						spinePort := "E1/" + strconv.Itoa(sp.downlinks[cursor[k]+n])
						links = append(links, map[string]any{
							"leaf":  map[string]any{"port": lf.dev + "/" + leafPort},
							"spine": map[string]any{"port": sp.dev + "/" + spinePort},
						})
					}
					cursor[k] += pps
					name := trunc63(sanitize(sp.dev + "-fabric-" + lf.dev))
					objs = append(objs, map[string]any{
						"apiVersion": wiringAPI, "kind": "Connection",
						"metadata": meta(name),
						"spec":     map[string]any{"fabric": map[string]any{"links": links}},
					})
				}
			}
		}

		y, err := marshalDocs(objs)
		if err != nil {
			return nil, fmt.Errorf("wiring: marshal fabric %q: %w", fabric, err)
		}
		docs = append(docs, Doc{Fabric: fabric, YAML: y})
	}
	return docs, nil
}

// --- CRD builders -------------------------------------------------------------

func vlanNamespaceCRD() map[string]any {
	return map[string]any{
		"apiVersion": wiringAPI, "kind": "VLANNamespace",
		"metadata": meta("default"),
		"spec":     map[string]any{"ranges": []any{map[string]any{"from": 1000, "to": 2999}}},
	}
}

func ipv4NamespaceCRD() map[string]any {
	return map[string]any{
		"apiVersion": vpcAPI, "kind": "IPv4Namespace",
		"metadata": meta("default"),
		"spec":     map[string]any{"subnets": []any{"10.0.0.0/16"}},
	}
}

// switchCRD builds a Switch with role/profile/boot.mac and the port map. A leaf in
// a redundancy group also carries spec.groups + spec.redundancy (MCLAG, F6 §2.4);
// classes with no group omit both (no empty ecmp: {} either — note §2.3).
func switchCRD(name, bootName, profile, role, redType, redGroup string, zones []topology.SwitchPortZone, brk map[string]breakout) map[string]any {
	spec := map[string]any{
		"role":    role,
		"profile": profile,
		"boot":    map[string]any{"mac": bootMAC(bootName)},
	}
	if redGroup != "" {
		spec["groups"] = []any{redGroup}
		if redType != "" {
			// hhfab requires the group alongside the type (a redundancy type without
			// a group is rejected at validate); the committed Switch carries both.
			spec["redundancy"] = map[string]any{"type": redType, "group": redGroup}
		}
	}
	breakouts, speeds := portMaps(zones, brk)
	if len(breakouts) > 0 {
		spec["portBreakouts"] = breakouts
	}
	if len(speeds) > 0 {
		spec["portSpeeds"] = speeds
	}
	return map[string]any{
		"apiVersion": wiringAPI, "kind": "Switch",
		"metadata": meta(name), "spec": spec,
	}
}

// switchGroupCRD builds a SwitchGroup (the MCLAG/redundancy group leaves
// reference via spec.groups). spec is empty, matching the committed wiring.
func switchGroupCRD(name string) map[string]any {
	return map[string]any{
		"apiVersion": wiringAPI, "kind": "SwitchGroup",
		"metadata": meta(name), "spec": map[string]any{},
	}
}

// isLeafRole reports whether a switch role connects up to spines (F6 §1.5).
func isLeafRole(role string) bool { return role == "server-leaf" || role == "border-leaf" }

// zonePorts enumerates, ascending, the physical ports of a class's zones of the
// given zone_type (uplink for leaves, fabric for spines).
func zonePorts(zones []topology.SwitchPortZone, zoneType string) []int {
	var out []int
	for _, z := range zones {
		if z.ZoneType == zoneType {
			out = append(out, enumeratePorts(z.PortSpec)...)
		}
	}
	return out
}

func serverCRD(name string) map[string]any {
	return map[string]any{
		"apiVersion": wiringAPI, "kind": "Server",
		"metadata": meta(name), "spec": map[string]any{},
	}
}

func unbundledCRD(name, serverPort, switchPort string) map[string]any {
	return map[string]any{
		"apiVersion": wiringAPI, "kind": "Connection",
		"metadata": meta(name),
		"spec": map[string]any{"unbundled": map[string]any{"link": map[string]any{
			"server": map[string]any{"port": serverPort},
			"switch": map[string]any{"port": switchPort},
		}}},
	}
}

// meshCRDs emits the mesh Connection(s) for a switch class: for every unordered
// pair of its switch instances (ordered by device name → leaf1/leaf2) it links
// the matching mesh-zone port on each. xoc-64 is a 2-switch mesh (one pair).
func meshCRDs(classID string, qty int, zones []topology.SwitchPortZone) []any {
	var meshPorts []int
	for _, z := range zones {
		if z.ZoneType == "mesh" {
			meshPorts = append(meshPorts, enumeratePorts(z.PortSpec)...)
		}
	}
	if qty < 2 || len(meshPorts) == 0 {
		return nil
	}
	names := make([]string, qty)
	for i := 0; i < qty; i++ {
		names[i] = switchDevName(classID, i)
	}
	sort.Strings(names)
	var out []any
	for i := 0; i < qty; i++ {
		for j := i + 1; j < qty; j++ {
			leaf1, leaf2 := names[i], names[j]
			var links []any
			for _, p := range meshPorts {
				port := "E1/" + strconv.Itoa(p)
				links = append(links, map[string]any{
					"leaf1": map[string]any{"port": leaf1 + "/" + port},
					"leaf2": map[string]any{"port": leaf2 + "/" + port},
				})
			}
			name := trunc63(sanitize("mesh-" + leaf1 + "-" + leaf2))
			out = append(out, map[string]any{
				"apiVersion": wiringAPI, "kind": "Connection",
				"metadata": meta(name),
				"spec":     map[string]any{"mesh": map[string]any{"links": links}},
			})
		}
	}
	return out
}

func meta(name string) map[string]any {
	return map[string]any{"name": name, "namespace": namespace}
}

// --- port maps + breakout resolution ------------------------------------------

type breakout struct {
	label     string // portBreakouts value (e.g. "2x400G")
	speed     string // portSpeeds value (e.g. "25G")
	highSpeed bool   // from_speed >= 100 → portBreakouts; else portSpeeds
}

// parseBreakouts indexes the catalog's breakout_options reference data by id.
func parseBreakouts(refData map[string]any) map[string]breakout {
	out := map[string]breakout{}
	opts, _ := refData["breakout_options"].([]any)
	for _, o := range opts {
		m, ok := o.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		bid, _ := m["breakout_id"].(string)
		from := asInt(m["from_speed"])
		logical := asInt(m["logical_speed"])
		out[id] = breakout{
			label:     strings.ReplaceAll(bid, "g", "G"),
			speed:     strconv.Itoa(logical) + "G",
			highSpeed: from >= 100,
		}
	}
	return out
}

// portMaps is the union over the switch class's zones: high-speed breakouts land
// in portBreakouts, low-speed (<100G) ports in portSpeeds (note §2.5).
func portMaps(zones []topology.SwitchPortZone, brk map[string]breakout) (breakouts, speeds map[string]string) {
	breakouts, speeds = map[string]string{}, map[string]string{}
	for _, z := range zones {
		b, ok := brk[z.BreakoutID]
		if !ok {
			continue
		}
		for _, p := range enumeratePorts(z.PortSpec) {
			key := "E1/" + strconv.Itoa(p)
			if b.highSpeed {
				breakouts[key] = b.label
			} else {
				speeds[key] = b.speed
			}
		}
	}
	return breakouts, speeds
}

// --- name normalization (note §2.2, §2.4, §2.7) -------------------------------

func switchDevName(classID string, idx int) string {
	return hyphenate(classID) + "-" + fmt.Sprintf("%02d", idx+1)
}

// switchBootName is the UNDERSCORE inventory form the boot.mac SHA hashes
// (note §2.4) — classID retains underscores; only the index is appended.
func switchBootName(classID string, idx int) string {
	return classID + "-" + fmt.Sprintf("%02d", idx+1)
}

func serverDevName(classID string, idx int) string {
	return hyphenate(classID) + "-" + fmt.Sprintf("%03d", idx+1)
}

func hyphenate(s string) string { return strings.ReplaceAll(s, "_", "-") }

func bootMAC(name string) string {
	h := sha256.Sum256([]byte(name))
	return fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x", h[1], h[2], h[3], h[4], h[5])
}

// sanitize lowercases, maps any non-[a-z0-9-] to '-', collapses runs of '-', and
// trims leading/trailing '-' (the HNP _sanitize_name rule).
func sanitize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return strings.Trim(out, "-")
}

// trunc63 truncates to the 63-char DNS-label limit and strips a trailing '-'.
func trunc63(s string) string {
	if len(s) > 63 {
		s = s[:63]
	}
	return strings.TrimRight(s, "-")
}

// nicIfaceName resolves a server NIC slot + port index to the NIC's interface
// template name (e.g. "so0") via the catalog (note §2.2).
func nicIfaceName(serverItem catalog.Item, cat *catalog.Catalog, nicSlotID string, portIndex int) string {
	for _, s := range serverItem.ComponentSlots {
		if s.SlotID != nicSlotID {
			continue
		}
		nic, ok := cat.Get(s.Target.ID)
		if !ok {
			return ""
		}
		if portIndex >= 0 && portIndex < len(nic.PortTemplates) {
			return nic.PortTemplates[portIndex].Name
		}
	}
	return ""
}

// --- port-spec enumeration (the diet zone grammar) ----------------------------

// enumeratePorts expands a port_spec ("A", "A-B", "A-B:step", comma-joined) into
// the physical port numbers it names — the enumerate analogue of bom.countPorts.
func enumeratePorts(spec string) []int {
	var out []int
	for _, tok := range strings.Split(spec, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		rng, step := tok, 1
		if i := strings.IndexByte(tok, ':'); i >= 0 {
			rng = tok[:i]
			if s, err := strconv.Atoi(tok[i+1:]); err == nil && s > 0 {
				step = s
			}
		}
		if i := strings.IndexByte(rng, '-'); i >= 0 {
			a, e1 := strconv.Atoi(strings.TrimSpace(rng[:i]))
			b, e2 := strconv.Atoi(strings.TrimSpace(rng[i+1:]))
			if e1 == nil && e2 == nil && b >= a {
				for p := a; p <= b; p += step {
					out = append(out, p)
				}
			}
			continue
		}
		if p, err := strconv.Atoi(rng); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// --- helpers ------------------------------------------------------------------

func sortedKeys(m map[string][]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

// marshalDocs renders the object stream as a multi-document YAML (each document
// prefixed with `---`), matching the committed wiring/*.yaml layout.
func marshalDocs(objs []any) ([]byte, error) {
	var buf bytes.Buffer
	for _, o := range objs {
		b, err := yaml.Marshal(o)
		if err != nil {
			return nil, err
		}
		buf.WriteString("---\n")
		buf.Write(b)
	}
	return buf.Bytes(), nil
}
