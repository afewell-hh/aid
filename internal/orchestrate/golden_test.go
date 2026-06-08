package orchestrate_test

// Golden-path acceptance (RED) — the Phase-6 exit gate:
//   1. aid topology calc <fixture> && aid export wiring <fixture> --fabric <name>
//      yields wiring YAML that `hhfab validate` ACCEPTS (genuinely shelled out).
//   2. aid topology bom <fixture> reproduces expected.json bom_totals.
//   3. aid plan validate emits a human-readable error for every invalid fixture.
//
// Fails at RED (orchestrate stubs); passes at GREEN.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afewell-hh/aid/internal/fixtures"
	"github.com/afewell-hh/aid/internal/orchestrate"
)

// hhfabValidate replicates the adapter's flow: `hhfab init --dev`, write the
// wiring to include/wiring.yaml, then `hhfab validate --brief`. Returns the
// combined log and success.
func hhfabValidate(t *testing.T, wiringYAML string) (bool, string) {
	t.Helper()
	if _, err := exec.LookPath("hhfab"); err != nil {
		t.Skip("hhfab not on PATH; skipping golden hhfab validate")
	}
	dir := t.TempDir()
	if out, err := runIn(dir, "hhfab", "init", "--dev"); err != nil {
		t.Fatalf("hhfab init: %v\n%s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(dir, "include"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "include", "wiring.yaml"), []byte(wiringYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runIn(dir, "hhfab", "validate", "--brief")
	t.Logf("hhfab validate (exit ok=%v):\n%s", err == nil, out)
	return err == nil, out
}

func runIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	return string(b), err
}

func TestGolden_WiringValidatesWithHhfab(t *testing.T) {
	plan, err := fixtures.PlanJSON("valid", "clos-small")
	if err != nil {
		t.Fatal(err)
	}
	// calc happens inside ExportWiring (kernel -> hhfab).
	docs, err := orchestrate.ExportWiring(plan, "frontend")
	if err != nil {
		t.Fatalf("export wiring: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("no wiring documents produced")
	}
	for _, d := range docs {
		ok, log := hhfabValidate(t, d.YAML)
		if !ok {
			t.Errorf("hhfab validate rejected fabric %q:\n%s", d.Fabric, log)
		}
	}
}

// device-class-bom[] shape needed to reconstruct totals.
type bomEntry struct {
	EntryID      string `json:"entry_id"`
	PlanQuantity int    `json:"plan_quantity"`
	LineItems    []struct {
		DeviceClass struct {
			Slug string `json:"slug"`
		} `json:"device_class"`
		QuantityPerUnit int `json:"quantity_per_unit"`
		FleetQuantity   int `json:"fleet_quantity"`
	} `json:"line_items"`
}

type expectedTotals struct {
	BomTotals map[string]struct {
		PlanQuantity int            `json:"plan_quantity"`
		PerUnit      map[string]int `json:"per_unit"`
		Fleet        map[string]int `json:"fleet"`
	} `json:"bom_totals"`
}

func TestGolden_BomReproducesExpectedTotals(t *testing.T) {
	plan, err := fixtures.PlanJSON("valid", "clos-small")
	if err != nil {
		t.Fatal(err)
	}
	// `aid topology bom` renders JSON via the bom adapter; smoke-check it runs.
	if _, err := orchestrate.ExportBOM(plan, "json"); err != nil {
		t.Fatalf("export bom: %v", err)
	}
	// The totals originate in the kernel-computed device-class-bom[]; verify
	// they reproduce expected.json bom_totals.
	res, err := orchestrate.Calculate(plan)
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if res.Ok == nil {
		t.Fatalf("kernel returned err: %s", res.Err)
	}
	rawExp, err := fixtures.Expected("clos-small")
	if err != nil {
		t.Fatal(err)
	}
	var exp expectedTotals
	if err := json.Unmarshal(rawExp, &exp); err != nil {
		t.Fatal(err)
	}
	byEntry := map[string]bomEntry{}
	for _, rb := range res.Ok.Boms {
		var be bomEntry
		if err := json.Unmarshal(rb, &be); err != nil {
			t.Fatalf("parse bom: %v", err)
		}
		byEntry[be.EntryID] = be
	}
	for entryID, want := range exp.BomTotals {
		got, ok := byEntry[entryID]
		if !ok {
			t.Errorf("missing BOM for entry %q", entryID)
			continue
		}
		if got.PlanQuantity != want.PlanQuantity {
			t.Errorf("%s: plan_quantity=%d want %d", entryID, got.PlanQuantity, want.PlanQuantity)
		}
		perUnit := map[string]int{}
		fleet := map[string]int{}
		for _, li := range got.LineItems {
			perUnit[li.DeviceClass.Slug] = li.QuantityPerUnit
			fleet[li.DeviceClass.Slug] = li.FleetQuantity
		}
		for slug, q := range want.PerUnit {
			if perUnit[slug] != q {
				t.Errorf("%s: per_unit[%s]=%d want %d", entryID, slug, perUnit[slug], q)
			}
		}
		for slug, q := range want.Fleet {
			if fleet[slug] != q {
				t.Errorf("%s: fleet[%s]=%d want %d", entryID, slug, fleet[slug], q)
			}
		}
	}
}

func TestGolden_PlanValidateHumanErrors(t *testing.T) {
	cases := []struct {
		fixture  string
		wantCode string
		wantSub  string // substring of the human-readable message
	}{
		{"mclag-odd-count", "MCLAG_SWITCH_COUNT", "switch_count"},
		{"mesh-four-switch", "MESH_SWITCH_COUNT", "mesh fabric"},
	}
	for _, c := range cases {
		plan, err := fixtures.PlanJSON("invalid", c.fixture)
		if err != nil {
			t.Fatalf("%s: %v", c.fixture, err)
		}
		res, err := orchestrate.Validate(plan)
		if err != nil {
			t.Fatalf("%s: validate: %v", c.fixture, err)
		}
		if res.IsValid {
			t.Errorf("%s: expected invalid", c.fixture)
			continue
		}
		found := false
		for _, e := range res.Errors {
			if e.Code == c.wantCode {
				found = true
				if !strings.Contains(strings.ToLower(e.Message), strings.ToLower(c.wantSub)) {
					t.Errorf("%s: message %q lacks %q", c.fixture, e.Message, c.wantSub)
				}
			}
		}
		if !found {
			t.Errorf("%s: no error with code %s; got %+v", c.fixture, c.wantCode, res.Errors)
		}
	}
}
