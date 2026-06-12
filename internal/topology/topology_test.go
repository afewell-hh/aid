package topology

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/objectmodel"
	"github.com/afewell-hh/aid/internal/oracle"
)

func trainingYAML(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(oracle.LayerADir(), "training.yaml"))
	if err != nil {
		t.Fatalf("read training fixture: %v", err)
	}
	return b
}

// --- test fixtures: a real split plan + catalog (built in-test) ---------------

func ref(name string) objectmodel.Ref {
	return objectmodel.Ref{ID: objectmodel.ID{Name: name, Version: "1"}}
}

// nicType returns a NIC hardware type with the given number of transceiver cages.
func nicType(name string, cages int) catalog.Item {
	it := catalog.Item{ID: objectmodel.ID{Name: name, Version: "1"}, Kind: catalog.KindNIC, Layer: catalog.LayerHardwareType}
	for i := 0; i < cages; i++ {
		it.PortTemplates = append(it.PortTemplates, catalog.PortTemplate{
			Name: "p" + string(rune('0'+i)), PortKind: catalog.TransceiverCage,
			MaxSpeedGbps: 400, CageType: "QSFP112", RequiresTransceiver: true,
		})
	}
	return it
}

// serverClass returns a server CLASS referencing a NIC type via one nic slot.
func serverClass(id, nicSlot, nicRef string) catalog.Item {
	return catalog.Item{
		ID: objectmodel.ID{Name: id, Version: "1"}, Kind: catalog.KindServer, Layer: catalog.LayerClass,
		ComponentSlots: []catalog.ComponentSlot{{SlotID: nicSlot, Target: ref(nicRef), Quantity: 1, Required: true}},
	}
}

// catalogFixture builds a catalog with a 2-cage and a 1-cage server class.
func catalogFixture(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.New(
		nicType("cx7-2cage", 2),
		nicType("cx7-1cage", 1),
		serverClass("compute", "nic-cx7", "cx7-2cage"),
		serverClass("compute1", "nic-cx7", "cx7-1cage"),
	)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func registryFixture(t *testing.T) *objectmodel.Registry {
	t.Helper()
	r, err := objectmodel.NewRegistry(
		objectmodel.Contract{Kind: catalog.KindNIC},
		objectmodel.Contract{Kind: catalog.KindServer, Relations: map[string]objectmodel.RelationContract{
			"component_slot": {Kind: "component_slot", Acyclic: true, QuantityField: "quantity"},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// goodPlan references the resolvable "compute" class with a pinned ref.
func goodPlan() *Plan {
	return &Plan{
		Meta: Meta{CaseID: "t", Name: "t"},
		Spec: Spec{
			Name:          "t",
			ServerClasses: []ServerClassUse{{ServerClassID: "compute", ClassRef: ref("compute"), Quantity: 4}},
		},
	}
}

// --- Guardrail 1: pinned catalog identity -------------------------------------

func TestIngestBundled_RefsArePinned(t *testing.T) {
	plan, cat, err := IngestBundled(trainingYAML(t))
	if err != nil {
		t.Fatalf("IngestBundled (F0 GREEN target): %v", err)
	}
	if plan == nil || cat == nil {
		t.Fatal("IngestBundled must return a plan and a catalog")
	}
	if got := len(plan.Spec.ServerClasses); got != 5 {
		t.Errorf("server classes: got %d want 5 (xoc-64 expected.counts)", got)
	}
	for _, sc := range plan.Spec.ServerClasses {
		if sc.ClassRef.Name == "" || sc.ClassRef.Version == "" {
			t.Errorf("class ref must be pinned (id+version), got %+v", sc.ClassRef)
		}
		// And the pinned ref must resolve into the extracted catalog.
		if _, ok := cat.Get(sc.ClassRef.ID); !ok {
			t.Errorf("class ref %s does not resolve into the extracted catalog", sc.ClassRef.ID)
		}
	}
}

func TestIngestPureReference_RejectsUnpinnedRef(t *testing.T) {
	// A pure-reference plan whose class_ref omits version must be rejected.
	unpinned := []byte(`
meta: {case_id: t, name: t}
spec:
  name: t
  server_classes:
    - {server_class_id: compute, class_ref: {name: compute}, quantity: 4}
`)
	_, err := IngestPureReference(unpinned, catalogFixture(t))
	if !errors.Is(err, ErrUnpinnedRef) {
		t.Fatalf("unpinned ref: want ErrUnpinnedRef (F0 GREEN target), got %v", err)
	}
}

// --- Guardrail 2: deterministic, lossless bundled ingest ----------------------

// bundledFacts extracts the load-bearing content a lossless round-trip must
// preserve: the set of reference_data device_type ids, the server_nics and
// server_connections counts, and expected.counts.
type bundledFacts struct {
	deviceTypeIDs []string
	nics          int
	connections   int
	expected      map[string]int
}

func extractBundledFacts(t *testing.T, y []byte) bundledFacts {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal(y, &doc); err != nil {
		t.Fatalf("parse bundled yaml: %v", err)
	}
	f := bundledFacts{expected: map[string]int{}}
	if rd, ok := doc["reference_data"].(map[string]any); ok {
		if dts, ok := rd["device_types"].([]any); ok {
			for _, dt := range dts {
				if m, ok := dt.(map[string]any); ok {
					if id, ok := m["id"].(string); ok {
						f.deviceTypeIDs = append(f.deviceTypeIDs, id)
					}
				}
			}
		}
	}
	if n, ok := doc["server_nics"].([]any); ok {
		f.nics = len(n)
	}
	if c, ok := doc["server_connections"].([]any); ok {
		f.connections = len(c)
	}
	if exp, ok := doc["expected"].(map[string]any); ok {
		if counts, ok := exp["counts"].(map[string]any); ok {
			for k, v := range counts {
				if iv, ok := v.(int); ok {
					f.expected[k] = iv
				}
			}
		}
	}
	sort.Strings(f.deviceTypeIDs)
	return f
}

func TestIngestRoundTrip_Lossless(t *testing.T) {
	src := trainingYAML(t)
	p, cat, err := IngestBundled(src)
	if err != nil {
		t.Fatalf("IngestBundled (F0 GREEN target): %v", err)
	}
	out, err := Rebundle(p, cat)
	if err != nil {
		t.Fatalf("Rebundle (F0 GREEN target): %v", err)
	}
	want := extractBundledFacts(t, src)
	got := extractBundledFacts(t, out)
	if !reflect.DeepEqual(want.deviceTypeIDs, got.deviceTypeIDs) {
		t.Errorf("round-trip lost/changed reference_data device_type ids:\n want %v\n got  %v", want.deviceTypeIDs, got.deviceTypeIDs)
	}
	if want.nics != got.nics {
		t.Errorf("round-trip changed server_nics count: want %d got %d", want.nics, got.nics)
	}
	if want.connections != got.connections {
		t.Errorf("round-trip changed server_connections count: want %d got %d", want.connections, got.connections)
	}
	if !reflect.DeepEqual(want.expected, got.expected) {
		t.Errorf("round-trip changed expected.counts: want %v got %v", want.expected, got.expected)
	}
}

// --- Guardrail 3: status/expected never drives production calc -----------------

func TestValidate_ResolvesRefs(t *testing.T) {
	// A well-formed plan over a catalog with resolvable, pinned refs validates.
	if err := Validate(goodPlan(), catalogFixture(t), registryFixture(t)); err != nil {
		t.Fatalf("Validate(good) (F0 GREEN target): %v", err)
	}
}

func TestValidate_RejectsUnresolvedRef(t *testing.T) {
	p := goodPlan()
	p.Spec.ServerClasses[0].ClassRef = ref("does-not-exist")
	p.Spec.ServerClasses[0].ServerClassID = "does-not-exist"
	if err := Validate(p, catalogFixture(t), registryFixture(t)); !errors.Is(err, ErrUnresolvedRef) {
		t.Fatalf("unresolved ref: want ErrUnresolvedRef (F0 GREEN target), got %v", err)
	}
}

func TestValidate_IgnoresStatus(t *testing.T) {
	// status/expected that conflicts with spec MUST NOT affect ordinary
	// validation (guardrail 3: it is read only in self-check mode).
	p := goodPlan()
	p.Status = &Status{Expected: &Expected{Counts: Counts{ServerClasses: 999, SwitchClasses: 999, Connections: 999}}}
	if err := Validate(p, catalogFixture(t), registryFixture(t)); err != nil {
		t.Fatalf("Validate must ignore status/expected (F0 GREEN target): %v", err)
	}
}

// --- Guardrail 4: deterministic ports_per_connection > 1 expansion -------------

func TestExpandPorts_DeterministicSequence(t *testing.T) {
	conn := ServerConnection{
		ServerClassID: "compute", NICSlotID: "nic-cx7", PortIndex: 0,
		PortsPerConnection: 2, TargetZone: "leaf/server",
	}
	got, err := ExpandPorts(conn, catalogFixture(t))
	if err != nil {
		t.Fatalf("ExpandPorts (F0 GREEN target): %v", err)
	}
	want := []CageBindingRef{
		{ServerClassID: "compute", NICSlotID: "nic-cx7", PortIndex: 0, ZoneName: "leaf/server"},
		{ServerClassID: "compute", NICSlotID: "nic-cx7", PortIndex: 1, ZoneName: "leaf/server"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ports_per_connection=2 expansion:\n want %+v\n got  %+v", want, got)
	}
}

func TestExpandPorts_RejectsInsufficientCages(t *testing.T) {
	// "compute1" references the 1-cage NIC; ports_per_connection=2 overflows it.
	conn := ServerConnection{
		ServerClassID: "compute1", NICSlotID: "nic-cx7", PortIndex: 0,
		PortsPerConnection: 2, TargetZone: "leaf/server",
	}
	if _, err := ExpandPorts(conn, catalogFixture(t)); !errors.Is(err, ErrInsufficientPorts) {
		t.Fatalf("insufficient cages: want ErrInsufficientPorts (F0 GREEN target), got %v", err)
	}
}
