package topology

// F1 ingestion tests (Issue #50): a real training_*.yaml ingests LOSSLESSLY into
// the relational topology model — the server_nics join, per-NIC-port connection
// intent, and catalog-ref resolution (server/switch classes + transceiver
// bindings resolved into the configured class) — and the expected.counts
// self-check reproduces the plan's committed counts. Still no calculation (F2+).

import (
	"errors"
	"testing"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/objectmodel"
)

// ingestTraining ingests the committed xoc-64 training form.
func ingestTraining(t *testing.T) (*Plan, *catalog.Catalog) {
	t.Helper()
	plan, cat, err := IngestBundled(trainingYAML(t))
	if err != nil {
		t.Fatalf("IngestBundled(training): %v", err)
	}
	if plan == nil || cat == nil {
		t.Fatal("IngestBundled must return a plan and a catalog")
	}
	return plan, cat
}

// serverItem returns the configured server class item named id.
func serverItem(t *testing.T, cat *catalog.Catalog, id string) catalog.Item {
	t.Helper()
	it, ok := cat.ByName(id)
	if !ok {
		t.Fatalf("configured server class %q not in extracted catalog", id)
	}
	if it.Kind != catalog.KindServer || it.Layer != catalog.LayerClass {
		t.Fatalf("%q must be a configured server CLASS, got kind=%s layer=%s", id, it.Kind, it.Layer)
	}
	return it
}

// nicSlots returns the component slots whose target resolves to a NIC type
// (i.e. the server_nics join), excluding the chassis/non-NIC slots.
func nicSlots(cat *catalog.Catalog, it catalog.Item) []catalog.ComponentSlot {
	var out []catalog.ComponentSlot
	for _, s := range it.ComponentSlots {
		if tgt, ok := cat.Get(s.Target.ID); ok && tgt.Kind == catalog.KindNIC {
			out = append(out, s)
		}
	}
	return out
}

// --- server_nics join ---------------------------------------------------------

func TestF1_ServerNicsJoin_Ingested(t *testing.T) {
	_, cat := ingestTraining(t)

	// compute_xpu has 4 NICs in the join: scale_out, soc_storage, inb_mgmt, bmc.
	compute := serverItem(t, cat, "compute_xpu")
	wantSlots := map[string]string{
		"scale_out":   "nic_xpu_scale_out_8x400",
		"soc_storage": "nic_xpu_soc_storage_2x200",
		"inb_mgmt":    "nic_dual_25g",
		"bmc":         "bmc_module",
	}
	got := map[string]string{}
	for _, s := range nicSlots(cat, compute) {
		got[s.SlotID] = s.Target.ID.Name
		if s.Target.Version == "" {
			t.Errorf("nic slot %q target ref not pinned: %+v", s.SlotID, s.Target)
		}
		if _, ok := cat.Get(s.Target.ID); !ok {
			t.Errorf("nic slot %q target %s does not resolve to a catalog NIC type", s.SlotID, s.Target.ID)
		}
	}
	for slot, nic := range wantSlots {
		if got[slot] != nic {
			t.Errorf("compute_xpu nic slot %q: got module %q want %q", slot, got[slot], nic)
		}
	}

	// The whole join must be ingested: 14 server_nics across the 5 classes.
	total := 0
	for _, scu := range []string{"compute_xpu", "storage_srv", "metadata_srv", "hh_gateway", "hh_controller"} {
		total += len(nicSlots(cat, serverItem(t, cat, scu)))
	}
	if total != 14 {
		t.Errorf("server_nics join: ingested %d NIC slots, want 14", total)
	}
}

// --- NIC + transceiver hardware types resolved against the catalog ------------

func TestF1_HardwareTypes_InCatalog(t *testing.T) {
	_, cat := ingestTraining(t)

	// NIC types with their expected cage counts (= interface_templates).
	nicCages := map[string]int{
		"nic_xpu_scale_out_8x400":  8,
		"nic_xpu_soc_storage_2x200": 2,
		"nic_dual_200g":            2,
		"nic_dual_25g":             2,
		"bmc_module":               1,
	}
	for id, cages := range nicCages {
		it, ok := cat.ByName(id)
		if !ok {
			t.Errorf("NIC type %q missing from extracted catalog", id)
			continue
		}
		if it.Kind != catalog.KindNIC || it.Layer != catalog.LayerHardwareType {
			t.Errorf("%q must be a NIC hardware type, got kind=%s layer=%s", id, it.Kind, it.Layer)
		}
		if c := countCages(it); c != cages {
			t.Errorf("NIC %q: %d cages, want %d", id, c, cages)
		}
	}

	// Transceiver types (no interface_templates) classified as transceivers.
	for _, id := range []string{"osfp_400g_dr4", "qsfp112_200gbase_sr2", "sfp28_25gbase_sr", "sfp_plus_10gbase_sr", "rj45_1000base_t", "r4113_a9220_vr", "r4113_a9221_dr"} {
		it, ok := cat.ByName(id)
		if !ok {
			t.Errorf("transceiver type %q missing from extracted catalog", id)
			continue
		}
		if it.Kind != catalog.KindTransceiver {
			t.Errorf("%q must be a transceiver hardware type, got kind=%s", id, it.Kind)
		}
	}
}

// --- per-NIC-port connection-level transceiver resolved INTO the class --------

func TestF1_PerNICPortCageBindings(t *testing.T) {
	_, cat := ingestTraining(t)
	compute := serverItem(t, cat, "compute_xpu")

	// Index bindings by (nic slot, port).
	type key struct {
		slot string
		port int
	}
	binds := map[key]string{}
	for _, b := range compute.CageBindings {
		binds[key{b.NICSlotID, b.PortIndex}] = b.SelectedTransceiver.ID.Name
		if _, ok := cat.Get(b.SelectedTransceiver.ID); !ok {
			t.Errorf("cage binding %s/%d transceiver %s does not resolve", b.NICSlotID, b.PortIndex, b.SelectedTransceiver.ID)
		}
	}

	// scale_out: 8 rail ports → osfp_400g_dr4.
	for p := 0; p < 8; p++ {
		if got := binds[key{"scale_out", p}]; got != "osfp_400g_dr4" {
			t.Errorf("scale_out port %d: bound %q want osfp_400g_dr4", p, got)
		}
	}
	// soc_storage: ports_per_connection=2 → ports 0 and 1 → qsfp112_200gbase_sr2.
	for p := 0; p < 2; p++ {
		if got := binds[key{"soc_storage", p}]; got != "qsfp112_200gbase_sr2" {
			t.Errorf("soc_storage port %d: bound %q want qsfp112_200gbase_sr2", p, got)
		}
	}
	// inb_mgmt port 0 → sfp28; bmc port 0 → rj45.
	if got := binds[key{"inb_mgmt", 0}]; got != "sfp28_25gbase_sr" {
		t.Errorf("inb_mgmt port 0: bound %q want sfp28_25gbase_sr", got)
	}
	if got := binds[key{"bmc", 0}]; got != "rj45_1000base_t" {
		t.Errorf("bmc port 0: bound %q want rj45_1000base_t", got)
	}
	// compute_xpu total bindings = 8 + 2 + 1 + 1 = 12.
	if len(compute.CageBindings) != 12 {
		t.Errorf("compute_xpu cage bindings: %d, want 12", len(compute.CageBindings))
	}
}

// --- catalog-ref resolution over the whole plan -------------------------------

func TestF1_ResolvePlan_AllRefsResolve(t *testing.T) {
	plan, cat := ingestTraining(t)
	if err := ResolvePlan(plan, cat); err != nil {
		t.Fatalf("ResolvePlan(xoc-64): %v", err)
	}
	// 21 per-NIC-port connections ingested.
	if len(plan.Spec.Connections) != 21 {
		t.Errorf("connections ingested: %d, want 21", len(plan.Spec.Connections))
	}
	// 3 collapsed switch classes ingested (training form).
	if len(plan.Spec.SwitchClasses) != 3 {
		t.Errorf("switch classes ingested: %d, want 3", len(plan.Spec.SwitchClasses))
	}
}

func TestF1_ResolvePlan_RejectsDanglingTransceiver(t *testing.T) {
	plan, cat := ingestTraining(t)
	plan.Spec.Connections[0].TransceiverID = "does_not_exist"
	if err := ResolvePlan(plan, cat); !errors.Is(err, ErrUnresolvedRef) {
		t.Fatalf("dangling transceiver: want ErrUnresolvedRef, got %v", err)
	}
}

func TestF1_ResolvePlan_RejectsDanglingZone(t *testing.T) {
	plan, cat := ingestTraining(t)
	plan.Spec.Connections[0].TargetZone = "no_such_zone"
	if err := ResolvePlan(plan, cat); !errors.Is(err, ErrUnresolvedRef) {
		t.Fatalf("dangling zone: want ErrUnresolvedRef, got %v", err)
	}
}

// --- mesh ingestion (faithful: topology_mode + mesh zone; no fabricated links) -

func TestF1_MeshIntentIngested(t *testing.T) {
	plan, _ := ingestTraining(t)
	var meshLeaf *SwitchClassUse
	for i := range plan.Spec.SwitchClasses {
		if plan.Spec.SwitchClasses[i].SwitchClassID == "soc_storage_scale_out_leaf" {
			meshLeaf = &plan.Spec.SwitchClasses[i]
		}
	}
	if meshLeaf == nil || meshLeaf.TopologyMode != "mesh" {
		t.Fatalf("soc_storage_scale_out_leaf topology_mode: got %+v want mesh", meshLeaf)
	}
	mesh := false
	for _, z := range plan.Spec.PortZones {
		if z.ZoneType == "mesh" {
			mesh = true
		}
	}
	if !mesh {
		t.Error("expected a mesh-type port zone to be ingested")
	}
}

// --- zone transceivers resolved ----------------------------------------------

func TestF1_ZoneTransceiversResolved(t *testing.T) {
	plan, cat := ingestTraining(t)
	seen := 0
	for _, z := range plan.Spec.PortZones {
		if z.Transceiver == "" {
			continue
		}
		seen++
		if _, ok := cat.ByName(z.Transceiver); !ok {
			t.Errorf("zone %s/%s transceiver %q does not resolve", z.SwitchClassID, z.ZoneName, z.Transceiver)
		}
	}
	if seen == 0 {
		t.Error("no zone transceivers ingested (transceiver_module_type dropped on zone ingest)")
	}
}

// --- expected.counts self-check (D21) ----------------------------------------

func TestF1_SelfCheck_XOC64(t *testing.T) {
	plan, _ := ingestTraining(t)
	got, err := SelfCheck(plan)
	if err != nil {
		t.Fatalf("SelfCheck(xoc-64): %v", err)
	}
	want := Counts{ServerClasses: 5, SwitchClasses: 3, Connections: 21}
	if got != want {
		t.Errorf("self-check counts: got %+v want %+v", got, want)
	}
	// Computed must be populated by the self-check.
	if plan.Status == nil || plan.Status.Computed == nil || plan.Status.Computed.Counts != want {
		t.Errorf("self-check must populate Status.Computed.Counts = %+v", want)
	}
}

func TestF1_SelfCheck_Mismatch(t *testing.T) {
	plan, _ := ingestTraining(t)
	plan.Status.Expected.Counts.Connections = 999
	if _, err := SelfCheck(plan); !errors.Is(err, ErrSelfCheckMismatch) {
		t.Fatalf("tampered expected: want ErrSelfCheckMismatch, got %v", err)
	}
}

func TestF1_SelfCheck_NoExpected(t *testing.T) {
	plan, _ := ingestTraining(t)
	plan.Status = nil
	if _, err := SelfCheck(plan); !errors.Is(err, ErrNoExpected) {
		t.Fatalf("no expected block: want ErrNoExpected, got %v", err)
	}
}

// --- ports_per_connection expansion over the REAL ingested catalog ------------

func TestF1_ExpandPorts_RealSocStorage(t *testing.T) {
	plan, cat := ingestTraining(t)
	var conn *ServerConnection
	for i := range plan.Spec.Connections {
		c := &plan.Spec.Connections[i]
		if c.ServerClassID == "compute_xpu" && c.NICSlotID == "soc_storage" {
			conn = c
		}
	}
	if conn == nil {
		t.Fatal("compute_xpu soc_storage connection not found")
	}
	if conn.PortsPerConnection != 2 {
		t.Fatalf("soc_storage ports_per_connection: got %d want 2", conn.PortsPerConnection)
	}
	binds, err := ExpandPorts(*conn, cat)
	if err != nil {
		t.Fatalf("ExpandPorts(real soc_storage): %v", err)
	}
	if len(binds) != 2 || binds[0].PortIndex != 0 || binds[1].PortIndex != 1 {
		t.Errorf("expansion: got %+v want 2 cages at ports 0,1", binds)
	}
}

var _ = objectmodel.Ref{} // keep import available for GREEN assertions
