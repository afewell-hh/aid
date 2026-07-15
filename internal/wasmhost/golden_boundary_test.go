package wasmhost_test

// Golden contract tests for the LIVE kernel JSON-over-linear-memory boundary
// (D16 / D28). These replace the retired #38 WIT drift guard: instead of checking
// that a dead WIT mirror covers a set of type names, they pin the ACTUAL wire
// behavior of the two live exports — export_f2_calculate and export_f3_bom — in
// both directions that matter:
//
//   - stable ACCEPTED INPUT shape: a fixed representative calc-plan / bom-scale
//     plan JSON is fed across the boundary;
//   - stable OUTPUT ENVELOPE: the raw kernel output bytes are pinned against a
//     committed golden file AND decoded on the Go side (calc.CalcOutput for F2, a
//     []int fleet array for F3) so a field rename/retype on either side fails.
//
// Regenerate the golden files after an INTENTIONAL boundary change:
//   go test ./internal/wasmhost -run TestGoldenBoundary -update
//
// The goldens are the executable definition of the live contract; a diff here is a
// wire change and must be reviewed as one.

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/components"
)

var updateGolden = flag.Bool("update", false, "regenerate golden boundary files")

// A representative F2 calc-plan: one derived (no override_quantity) leaf switch
// class with a breakout-free server zone, one server class, one same-switch
// connection carrying matching server/zone transceiver optics. Exercises
// switch-count derivation, sequential port allocation, per-endpoint identity, and
// a transceiver verdict — a deterministic, valid plan.
const goldenF2Plan = `{"switch_classes":[{"switch_class_id":"leaf","override_quantity":null,"redundancy":"none","topology_mode":"clos","zones":[{"zone_name":"z","zone_type":"server","port_spec":"1-4","breakout_logical_ports":1,"allocation_strategy":"sequential","transceiver_attrs":{"medium":"optical","cage_type":"qsfp28","connector":"mpo"}}]}],"server_classes":[{"server_class_id":"srv","quantity":4}],"connections":[{"connection_id":"c","server_class_id":"srv","server_quantity":4,"nic_slot_id":"n","port_index":0,"ports_per_connection":1,"speed":100,"distribution":"same-switch","target_switch_class":"leaf","target_zone":"z","server_transceiver_attrs":{"medium":"optical","cage_type":"qsfp28","connector":"mpo"}}]}`

// A representative F3 bom-scale plan: a two-root nested tree (a server line with a
// nested 8x component that itself nests a 2x child, plus a switch line), so the
// proven child_qpu/fleet_quantity recursion is exercised across >1 nesting level.
const goldenF3Plan = `{"nodes":[{"parent_index":-1,"quantity_per_parent":1,"plan_quantity":6},{"parent_index":0,"quantity_per_parent":8,"plan_quantity":6},{"parent_index":1,"quantity_per_parent":2,"plan_quantity":6},{"parent_index":-1,"quantity_per_parent":1,"plan_quantity":3}]}`

func goldenPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", "golden", name)
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := goldenPath(t, name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated golden %s (%d bytes)", name, len(got))
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update to create): %v", name, err)
	}
	if string(got) != string(want) {
		t.Fatalf("wire drift in %s:\n got: %s\nwant: %s", name, got, want)
	}
}

func TestGoldenBoundaryF2(t *testing.T) {
	kernel, err := components.Kernel()
	if err != nil {
		t.Fatal(err)
	}
	out, err := kernel.Call(components.KernelF2Calculate, []byte(goldenF2Plan))
	if err != nil {
		t.Fatal(err)
	}

	// (1) raw output envelope pinned as the wire contract.
	checkGolden(t, "f2_calc_output.json", out)

	// (2) Go-side decode: the envelope must decode into calc.CalcOutput and carry
	// the expected shape (no calc errors, derived quantity, all endpoints, verdict).
	var co calc.CalcOutput
	if err := json.Unmarshal(out, &co); err != nil {
		t.Fatalf("F2 output must decode as calc.CalcOutput: %v", err)
	}
	if len(co.Errors) != 0 {
		t.Fatalf("golden plan is valid; got errors: %+v", co.Errors)
	}
	if len(co.SwitchQuantity) != 1 || co.SwitchQuantity[0].Quantity != 1 {
		t.Fatalf("derived switch quantity: got %+v want [{leaf 1}]", co.SwitchQuantity)
	}
	if len(co.Endpoints) != 4 {
		t.Fatalf("endpoints: got %d want 4", len(co.Endpoints))
	}
	if len(co.TransceiverVerdicts) != 1 || co.TransceiverVerdicts[0].ReasonCode != "R_MATCH" {
		t.Fatalf("verdict: got %+v want one R_MATCH", co.TransceiverVerdicts)
	}
}

func TestGoldenBoundaryF3(t *testing.T) {
	kernel, err := components.Kernel()
	if err != nil {
		t.Fatal(err)
	}
	out, err := kernel.Call(components.KernelF3Bom, []byte(goldenF3Plan))
	if err != nil {
		t.Fatal(err)
	}

	// (1) raw output envelope pinned as the wire contract.
	checkGolden(t, "f3_scaled_lines.json", out)

	// (2) Go-side decode: compact []int fleet array, one per node, proven scaling.
	//   node0 root:       fleet = 1  * 6 = 6
	//   node1 8x/root:    fleet = 8  * 6 = 48
	//   node2 2x/node1:   fleet = 16 * 6 = 96
	//   node3 root:       fleet = 1  * 3 = 3
	var fleets []int
	if err := json.Unmarshal(out, &fleets); err != nil {
		t.Fatalf("F3 output must decode as []int: %v", err)
	}
	want := []int{6, 48, 96, 3}
	if len(fleets) != len(want) {
		t.Fatalf("fleets: got %v want %v", fleets, want)
	}
	for i := range want {
		if fleets[i] != want[i] {
			t.Fatalf("fleets: got %v want %v", fleets, want)
		}
	}
}
