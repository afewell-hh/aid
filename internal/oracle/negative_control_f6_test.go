package oracle

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/topology"
)

// F6 negative controls (Issue #63, Clos derivation + §2.8 comparator extension).
//
// Two families, both with REAL teeth:
//   - Derivation sensitivity: the DERIVED spine count must react to its inputs
//     (spine fabric capacity, leaf uplink demand). A count hardwired/ignored
//     would not move when the input changes — this catches that.
//   - Comparator coverage: the §2.8 additions (SwitchGroup kind+identity,
//     per-switch redundancy/groups, fabric-link pairings) must each make
//     CompareWiringHhfab non-Equal when violated — proving the structural bar,
//     not just hhfab validate, is load-bearing for Clos wiring.
//
// RED note: until the GREEN spine pass lands, the SpineDerivation* cases are
// expected to FAIL (the derived count is absent, so it does not move with the
// input) — that is the engine-RED signal. The comparator-coverage cases operate
// on committed wiring and have teeth independent of the renderer.

const xoc256 = "xoc-256-2xopg128-clos-ro"

// deriveQty recomputes the derived quantity for one switch class from a
// (possibly mutated) plan.
func deriveQty(t *testing.T, c Composition, mutate func(p *topology.Plan), class string) int {
	t.Helper()
	plan, cat := ingest(t, c)
	if mutate != nil {
		mutate(plan)
	}
	sw, _, err := calc.DeriveQuantities(plan, cat)
	if err != nil {
		t.Fatalf("DeriveQuantities(%s): %v", c.Name, err)
	}
	return sw[class]
}

// setPortSpec mutates the port_spec of a (switch class, zone) in place.
func setPortSpec(t *testing.T, plan *topology.Plan, class, zone, spec string) {
	t.Helper()
	for i := range plan.Spec.PortZones {
		z := &plan.Spec.PortZones[i]
		if z.SwitchClassID == class && z.ZoneName == zone {
			z.PortSpec = spec
			return
		}
	}
	t.Fatalf("zone %s/%s not found in plan", class, zone)
}

// TestNegative_F6_SpineDerivationReactsToFabricCapacity: halving the be-spine
// fabric (downlink) capacity must change the derived be-spine count
// (ceil(128/64)=2 → ceil(128/32)=4). If the count is hardwired or the fabric
// zone is ignored, baseline == perturbed and this fails.
func TestNegative_F6_SpineDerivationReactsToFabricCapacity(t *testing.T) {
	c := compByName(t, xoc256)

	base := deriveQty(t, c, nil, "be-spine")
	perturbed := deriveQty(t, c, func(p *topology.Plan) {
		setPortSpec(t, p, "be-spine", "be-fabric-downlinks", "1-32") // halve downlink capacity
	}, "be-spine")

	if base == perturbed {
		t.Fatalf("negative control FAILED: be-spine count did not react to halving its fabric capacity "+
			"(base=%d, perturbed=%d) — derivation ignores the spine fabric zone or is hardwired", base, perturbed)
	}
}

// TestNegative_F6_SpineDerivationReactsToLeafUplinks: widening the leaf uplink
// zone increases per-leaf uplink demand and must change the derived be-spine
// count. Guards the §1.3 demand term.
func TestNegative_F6_SpineDerivationReactsToLeafUplinks(t *testing.T) {
	c := compByName(t, xoc256)

	base := deriveQty(t, c, nil, "be-spine")
	perturbed := deriveQty(t, c, func(p *topology.Plan) {
		// 4 leaves × 64 uplinks / 64 = 4 spines (was 2). Any change proves sensitivity.
		setPortSpec(t, p, "be-rail-leaf", "be-uplinks", "1-64")
	}, "be-spine")

	if base == perturbed {
		t.Fatalf("negative control FAILED: be-spine count did not react to widening the leaf uplink zone "+
			"(base=%d, perturbed=%d) — derivation ignores leaf uplink demand", base, perturbed)
	}
}

// TestNegative_F6_ComparatorCoversClos proves the §2.8 comparator additions have
// teeth: each targeted mutation of the COMPUTED frontend wiring must make
// CompareWiringHhfab non-Equal (baseline committed-vs-itself is Equal).
func TestNegative_F6_ComparatorCoversClos(t *testing.T) {
	c := compByName(t, xoc256)
	wiringDir := filepath.Join(c.Dir(), "wiring")

	committed := readCommittedWiring(t, wiringDir)

	// Baseline: committed vs itself is Equal.
	if diff, err := CompareWiringHhfab(cloneWiring(committed), wiringDir); err != nil {
		t.Fatalf("baseline CompareWiringHhfab: %v", err)
	} else if !diff.Equal {
		t.Fatalf("baseline not Equal (committed vs itself): %v", diff.Details)
	}

	cases := []struct {
		name   string
		mutate func(docs []map[string]any)
	}{
		{
			name: "drop SwitchGroup",
			mutate: func(docs []map[string]any) {
				dropDoc(docs, func(d map[string]any) bool { return d["kind"] == "SwitchGroup" })
			},
		},
		{
			name: "strip per-switch redundancy+groups",
			mutate: func(docs []map[string]any) {
				for _, d := range docs {
					if d["kind"] == "Switch" {
						if spec, ok := d["spec"].(map[string]any); ok {
							delete(spec, "redundancy")
							delete(spec, "groups")
						}
					}
				}
			},
		},
		{
			name: "corrupt a fabric link spine port",
			mutate: func(docs []map[string]any) {
				for _, d := range docs {
					if d["kind"] != "Connection" {
						continue
					}
					spec, _ := d["spec"].(map[string]any)
					fab, _ := spec["fabric"].(map[string]any)
					links, _ := fab["links"].([]any)
					if len(links) == 0 {
						continue
					}
					lm, _ := links[0].(map[string]any)
					if sp, ok := lm["spine"].(map[string]any); ok {
						sp["port"] = "WRONG/E1/999"
						return
					}
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			computed := cloneWiring(committed)
			docs := decodeDocs(t, computed["frontend"])
			tc.mutate(docs)
			computed["frontend"] = encodeDocs(t, docs)

			diff, err := CompareWiringHhfab(computed, wiringDir)
			if err != nil {
				t.Fatalf("CompareWiringHhfab: %v", err)
			}
			if diff.Equal {
				t.Fatalf("negative control FAILED: comparator passed after mutation %q — §2.8 check is not load-bearing", tc.name)
			}
		})
	}
}

// --- wiring-doc helpers (test only) -------------------------------------------

func readCommittedWiring(t *testing.T, wiringDir string) map[string][]byte {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join(wiringDir, "wiring-*.yaml"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("glob committed wiring: entries=%d err=%v", len(entries), err)
	}
	out := map[string][]byte{}
	for _, p := range entries {
		base := filepath.Base(p)
		fabric := base[len("wiring-") : len(base)-len(".yaml")]
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		out[fabric] = b
	}
	return out
}

func cloneWiring(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		c := make([]byte, len(v))
		copy(c, v)
		out[k] = c
	}
	return out
}

func decodeDocs(t *testing.T, b []byte) []map[string]any {
	t.Helper()
	dec := yaml.NewDecoder(bytes.NewReader(b))
	var docs []map[string]any
	for {
		var d map[string]any
		if err := dec.Decode(&d); err != nil {
			break
		}
		if d != nil {
			docs = append(docs, d)
		}
	}
	if len(docs) == 0 {
		t.Fatal("decodeDocs: no documents")
	}
	return docs
}

func encodeDocs(t *testing.T, docs []map[string]any) []byte {
	t.Helper()
	var buf []byte
	for _, d := range docs {
		if d == nil {
			continue
		}
		y, err := yaml.Marshal(d)
		if err != nil {
			t.Fatalf("marshal doc: %v", err)
		}
		buf = append(buf, []byte("---\n")...)
		buf = append(buf, y...)
	}
	return buf
}

// dropDoc nils out the first doc matching pred (encodeDocs skips nil docs).
func dropDoc(docs []map[string]any, pred func(map[string]any) bool) {
	for i, d := range docs {
		if pred(d) {
			docs[i] = nil
			return
		}
	}
}
