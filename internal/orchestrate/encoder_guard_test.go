package orchestrate_test

// Encoder guard (RED): the canonical kernel calc-output encoder must reproduce
// the bytes the adapters consume — the vendored topology-ir JSON
// (IR_CONTRACT.md) and device-class-bom[] JSON (BOM_CONTRACT.md). This is the
// D16 consolidation gate: kernel/src/encode.mbt becomes the single source for
// the IR/BOM wire shapes. We compare SEMANTICALLY (whitespace-insignificant).
//
// We do NOT touch hhfab-adapter/tools/ir-gen or bom-adapter/tools/bom-gen
// (retirement tracked as #35, post-merge).

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/afewell-hh/aid/internal/fixtures"
	"github.com/afewell-hh/aid/internal/orchestrate"
)

func semanticEqual(a, b []byte) (bool, error) {
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, &y); err != nil {
		return false, err
	}
	return reflect.DeepEqual(x, y), nil
}

func TestEncoderGuard_IRMatchesVendored(t *testing.T) {
	for _, name := range []string{"clos-small", "mesh-two-switch", "switch-bom"} {
		plan, err := fixtures.PlanJSON("valid", name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		res, err := orchestrate.Calculate(plan)
		if err != nil {
			t.Fatalf("%s: Calculate: %v", name, err)
		}
		if res.Ok == nil {
			t.Fatalf("%s: kernel returned err: %s", name, res.Err)
		}
		got, err := json.Marshal(res.Ok.IR)
		if err != nil {
			t.Fatalf("%s: marshal IR: %v", name, err)
		}
		want, err := fixtures.VendoredIR(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		eq, err := semanticEqual(got, want)
		if err != nil {
			t.Fatalf("%s: compare: %v", name, err)
		}
		if !eq {
			t.Errorf("%s: kernel-encoded IR does not match vendored %s.ir.json", name, name)
		}
	}
}

func TestEncoderGuard_BomsMatchVendored(t *testing.T) {
	for _, name := range []string{"clos-small", "mesh-two-switch", "switch-bom"} {
		plan, err := fixtures.PlanJSON("valid", name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		res, err := orchestrate.Calculate(plan)
		if err != nil {
			t.Fatalf("%s: Calculate: %v", name, err)
		}
		if res.Ok == nil {
			t.Fatalf("%s: kernel returned err: %s", name, res.Err)
		}
		got, err := json.Marshal(res.Ok.Boms)
		if err != nil {
			t.Fatalf("%s: marshal boms: %v", name, err)
		}
		want, err := fixtures.VendoredBoms(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		eq, err := semanticEqual(got, want)
		if err != nil {
			t.Fatalf("%s: compare: %v", name, err)
		}
		if !eq {
			t.Errorf("%s: kernel-encoded boms do not match vendored %s.boms.json", name, name)
		}
	}
}
