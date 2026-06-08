package wasmhost_test

// RED tests:
//   (1) kernel-ABI round-trip: plan JSON -> calc-output across kernel.wasm;
//       node/edge/fabric counts match the vendored IR; malformed -> {"err":...}
//       with no trap.
//   (2) wasmhost round-trip for ALL THREE components through the one host.
//
// These fail at RED because wasmhost.Call is unimplemented and the embedded
// components are placeholders. They pass at GREEN once the host is built and
// `make wasm` produces real components.

import (
	"encoding/json"
	"testing"

	"github.com/afewell-hh/aid/internal/components"
	"github.com/afewell-hh/aid/internal/fixtures"
)

type calcResult struct {
	Ok *struct {
		IR struct {
			Nodes   []json.RawMessage `json:"nodes"`
			Edges   []json.RawMessage `json:"edges"`
			Fabrics []json.RawMessage `json:"fabrics"`
		} `json:"ir"`
		Boms       []json.RawMessage `json:"boms"`
		Validation struct {
			IsValid bool `json:"is_valid"`
		} `json:"validation"`
	} `json:"ok"`
	Err json.RawMessage `json:"err"`
}

// vendoredCounts reads the committed IR and returns its node/edge/fabric counts.
func vendoredCounts(t *testing.T, name string) (int, int, int) {
	t.Helper()
	raw, err := fixtures.VendoredIR(name)
	if err != nil {
		t.Fatalf("read vendored IR %s: %v", name, err)
	}
	var ir struct {
		Nodes   []json.RawMessage `json:"nodes"`
		Edges   []json.RawMessage `json:"edges"`
		Fabrics []json.RawMessage `json:"fabrics"`
	}
	if err := json.Unmarshal(raw, &ir); err != nil {
		t.Fatalf("parse vendored IR %s: %v", name, err)
	}
	return len(ir.Nodes), len(ir.Edges), len(ir.Fabrics)
}

func TestKernelABI_CountsMatchVendoredIR(t *testing.T) {
	kernel, err := components.Kernel()
	if err != nil {
		t.Fatalf("load kernel: %v", err)
	}
	for _, name := range []string{"clos-small", "mesh-two-switch", "switch-bom"} {
		plan, err := fixtures.PlanJSON("valid", name)
		if err != nil {
			t.Fatalf("read plan %s: %v", name, err)
		}
		out, err := kernel.Call(components.KernelCalculate, plan)
		if err != nil {
			t.Fatalf("%s: kernel Call: %v", name, err)
		}
		var res calcResult
		if err := json.Unmarshal(out, &res); err != nil {
			t.Fatalf("%s: parse calc-output: %v", name, err)
		}
		if res.Ok == nil {
			t.Fatalf("%s: expected ok, got err=%s", name, res.Err)
		}
		if !res.Ok.Validation.IsValid {
			t.Errorf("%s: expected valid plan", name)
		}
		wn, we, wf := vendoredCounts(t, name)
		if got := len(res.Ok.IR.Nodes); got != wn {
			t.Errorf("%s: nodes=%d want %d", name, got, wn)
		}
		if got := len(res.Ok.IR.Edges); got != we {
			t.Errorf("%s: edges=%d want %d", name, got, we)
		}
		if got := len(res.Ok.IR.Fabrics); got != wf {
			t.Errorf("%s: fabrics=%d want %d", name, got, wf)
		}
	}
}

func TestKernelABI_MalformedReturnsErrNoTrap(t *testing.T) {
	kernel, err := components.Kernel()
	if err != nil {
		t.Fatalf("load kernel: %v", err)
	}
	out, err := kernel.Call(components.KernelCalculate, []byte(`{"not":"a plan"}`))
	if err != nil {
		t.Fatalf("malformed input must NOT trap; got host error: %v", err)
	}
	var res calcResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("parse calc-output: %v", err)
	}
	if res.Ok != nil || res.Err == nil {
		t.Errorf("expected {\"err\":...} for malformed plan, got ok=%v err=%s", res.Ok, res.Err)
	}
}

// --- (2) round-trip for all three components through the one host ------------

func TestRoundTrip_Kernel(t *testing.T) {
	kernel, _ := components.Kernel()
	plan, err := fixtures.PlanJSON("valid", "clos-small")
	if err != nil {
		t.Fatal(err)
	}
	out, err := kernel.Call(components.KernelCalculate, plan)
	if err != nil {
		t.Fatalf("kernel round-trip: %v", err)
	}
	var res calcResult
	if err := json.Unmarshal(out, &res); err != nil || res.Ok == nil {
		t.Fatalf("kernel produced no calc-output: err=%v", err)
	}
}

func TestRoundTrip_Hhfab(t *testing.T) {
	hhfab, _ := components.Hhfab()
	ir, err := fixtures.VendoredIR("clos-small")
	if err != nil {
		t.Fatal(err)
	}
	input := mustJSON(t, map[string]any{
		"ir":      json.RawMessage(ir),
		"options": map[string]any{"fabric": nil, "split_by_fabric": false},
	})
	out, err := hhfab.Call(components.HhfabExport, input)
	if err != nil {
		t.Fatalf("hhfab round-trip: %v", err)
	}
	var res struct {
		Ok *struct {
			Documents []struct {
				Fabric string `json:"fabric"`
				YAML   string `json:"yaml"`
			} `json:"documents"`
		} `json:"ok"`
	}
	if err := json.Unmarshal(out, &res); err != nil || res.Ok == nil || len(res.Ok.Documents) == 0 {
		t.Fatalf("hhfab produced no documents: err=%v out=%s", err, out)
	}
}

func TestRoundTrip_Bom(t *testing.T) {
	bom, _ := components.Bom()
	boms, err := fixtures.VendoredBoms("clos-small")
	if err != nil {
		t.Fatal(err)
	}
	input := mustJSON(t, map[string]any{
		"boms":    json.RawMessage(boms),
		"options": map[string]any{"format": "json", "include_fleet_totals": true},
	})
	out, err := bom.Call(components.BomExport, input)
	if err != nil {
		t.Fatalf("bom round-trip: %v", err)
	}
	var res struct {
		Ok *struct {
			Format  string `json:"format"`
			Content string `json:"content"`
		} `json:"ok"`
	}
	if err := json.Unmarshal(out, &res); err != nil || res.Ok == nil || res.Ok.Content == "" {
		t.Fatalf("bom produced no content: err=%v out=%s", err, out)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
