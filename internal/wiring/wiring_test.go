package wiring

// F4 RED unit fixtures (Issue #60; note §5). Each test drives the renderer on the
// committed xoc-64 inputs and asserts one slice of the IR→CRD contract
// (note §2.1–§2.6). In RED, Render is a stub returning ErrNotImplemented, so each
// test FAILS for the right reason — `F4 RED — wiring renderer not implemented` —
// at the render call, not a skip and not a compile error. GREEN implements Render
// and these assertions become the contract it must satisfy.

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/topology"
)

func repoRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(f)))
}

// xoc64Inputs ingests the committed xoc-64 training form, merges the AID optic
// overlay (so the switch Item.Model resolves to the hhfab profile — note §2.4),
// and runs the F2 calc — the same inputs internal/oracle feeds the renderer.
func xoc64Inputs(t *testing.T) (*topology.Plan, *catalog.Catalog, *calc.CalcOutput) {
	t.Helper()
	root := repoRoot()
	b, err := os.ReadFile(filepath.Join(root, "tests", "oracle", "xoc-64-mesh-conv-ro", "training.yaml"))
	if err != nil {
		t.Fatalf("read training.yaml: %v", err)
	}
	plan, cat, err := topology.IngestBundled(b)
	if err != nil {
		t.Fatalf("IngestBundled(xoc-64): %v", err)
	}
	overlay, err := catalog.Load(filepath.Join(root, "tests", "fixtures", "f3", "optic-overlay.yaml"))
	if err != nil {
		t.Fatalf("load optic overlay: %v", err)
	}
	cat.Merge(overlay)
	co, err := calc.Compute(plan, cat)
	if err != nil {
		t.Fatalf("calc.Compute(xoc-64): %v", err)
	}
	return plan, cat, co
}

// renderXOC64 runs the F4 renderer on the xoc-64 inputs. RED: Render returns
// ErrNotImplemented and this t.Fatalf's with the intended-RED reason.
func renderXOC64(t *testing.T) []Doc {
	t.Helper()
	plan, cat, co := xoc64Inputs(t)
	docs, err := Render(plan, cat, co)
	if err != nil {
		t.Fatalf("F4 RED — wiring renderer not implemented: %v", err)
	}
	return docs
}

// --- tiny multi-doc CRD reader (test-local; GREEN asserts against it) ----------

type crd struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec map[string]any `yaml:"spec"`
}

func parseCRDs(t *testing.T, y []byte) []crd {
	t.Helper()
	dec := yaml.NewDecoder(bytes.NewReader(y))
	var out []crd
	for {
		var c crd
		err := dec.Decode(&c)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("parse CRD yaml: %v", err)
		}
		if c.Kind == "" {
			continue
		}
		out = append(out, c)
	}
	return out
}

func docFor(t *testing.T, docs []Doc, fabric string) []crd {
	t.Helper()
	for _, d := range docs {
		if d.Fabric == fabric {
			return parseCRDs(t, d.YAML)
		}
	}
	t.Fatalf("no wiring doc for managed fabric %q (got %d docs)", fabric, len(docs))
	return nil
}

func switchByName(t *testing.T, crds []crd, name string) crd {
	t.Helper()
	for _, c := range crds {
		if c.Kind == "Switch" && c.Metadata.Name == name {
			return c
		}
	}
	t.Fatalf("no Switch %q in fabric", name)
	return crd{}
}

// dig walks nested string-keyed maps and returns the terminal string.
func dig(m map[string]any, keys ...string) (string, bool) {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = mm[k]
		if !ok {
			return "", false
		}
	}
	s, ok := cur.(string)
	return s, ok
}

func strMap(v any) map[string]string {
	out := map[string]string{}
	if m, ok := v.(map[string]any); ok {
		for k, vv := range m {
			if s, ok := vv.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

// hasUnbundled reports whether some Connection has an unbundled link with the
// given server/switch ports.
func hasUnbundled(crds []crd, serverPort, switchPort string) bool {
	for _, c := range crds {
		if c.Kind != "Connection" {
			continue
		}
		sp, ok1 := dig(c.Spec, "unbundled", "link", "server", "port")
		wp, ok2 := dig(c.Spec, "unbundled", "link", "switch", "port")
		if ok1 && ok2 && sp == serverPort && wp == switchPort {
			return true
		}
	}
	return false
}

// hasMeshLink reports whether some mesh Connection pairs the two given ports
// (order-insensitive on leaf1/leaf2).
func hasMeshLink(crds []crd, a, b string) bool {
	for _, c := range crds {
		if c.Kind != "Connection" {
			continue
		}
		mesh, ok := c.Spec["mesh"].(map[string]any)
		if !ok {
			continue
		}
		links, ok := mesh["links"].([]any)
		if !ok {
			continue
		}
		for _, l := range links {
			lm, ok := l.(map[string]any)
			if !ok {
				continue
			}
			p1, _ := dig(lm, "leaf1", "port")
			p2, _ := dig(lm, "leaf2", "port")
			if (p1 == a && p2 == b) || (p1 == b && p2 == a) {
				return true
			}
		}
	}
	return false
}

func countKind(crds []crd, kind string) int {
	n := 0
	for _, c := range crds {
		if c.Kind == kind {
			n++
		}
	}
	return n
}

// --- §2.1 fabric grouping ------------------------------------------------------

func TestRender_FabricGrouping(t *testing.T) {
	docs := renderXOC64(t)
	got := map[string]bool{}
	for _, d := range docs {
		got[d.Fabric] = true
	}
	for _, want := range []string{"soc-storage-scale-out", "inb-mgmt"} {
		if !got[want] {
			t.Errorf("§2.1: missing managed-fabric doc %q", want)
		}
	}
	if got["oob-mgmt"] {
		t.Errorf("§2.1: oob-mgmt is unmanaged and must NOT produce a wiring doc")
	}
	if len(docs) != 2 {
		t.Errorf("§2.1: want exactly 2 managed-fabric docs, got %d", len(docs))
	}
}

// --- §2.2 device-name normalization -------------------------------------------

func TestRender_DeviceNameNormalization(t *testing.T) {
	docs := renderXOC64(t)
	soc := docFor(t, docs, "soc-storage-scale-out")
	switchByName(t, soc, "soc-storage-scale-out-leaf-01") // hyphenated switch device name
	if !hasUnbundled(soc, "compute-xpu-001/scale_out-so0", "soc-storage-scale-out-leaf-01/E1/1/1") {
		t.Errorf("§2.2: server port must preserve underscores in the nic-slot suffix " +
			"(compute-xpu-001/scale_out-so0 -> soc-storage-scale-out-leaf-01/E1/1/1)")
	}
}

// --- §2.3 Switch spec: profile + role -----------------------------------------

func TestRender_SwitchProfileRole(t *testing.T) {
	docs := renderXOC64(t)
	soc := switchByName(t, docFor(t, docs, "soc-storage-scale-out"), "soc-storage-scale-out-leaf-01")
	if p, _ := dig(soc.Spec, "profile"); p != "celestica-ds5000" {
		t.Errorf("§2.3: soc switch spec.profile=%q want celestica-ds5000", p)
	}
	if r, _ := dig(soc.Spec, "role"); r != "server-leaf" {
		t.Errorf("§2.3: soc switch spec.role=%q want server-leaf", r)
	}
	if _, hasEcmp := soc.Spec["ecmp"]; hasEcmp {
		t.Errorf("§2.3: Switch spec must NOT carry ecmp (no empty ecmp: {})")
	}
	inb := switchByName(t, docFor(t, docs, "inb-mgmt"), "inb-mgmt-leaf-01")
	if p, _ := dig(inb.Spec, "profile"); p != "celestica-ds2000" {
		t.Errorf("§2.3: inb switch spec.profile=%q want celestica-ds2000", p)
	}
}

// --- §2.4 boot.mac (verified SHA256 formula) -----------------------------------

func TestRender_BootMAC(t *testing.T) {
	docs := renderXOC64(t)
	soc := docFor(t, docs, "soc-storage-scale-out")
	want := map[string]string{
		"soc-storage-scale-out-leaf-01": "02:d1:30:5d:84:0c",
		"soc-storage-scale-out-leaf-02": "02:b7:11:db:8a:74",
	}
	for name, mac := range want {
		sw := switchByName(t, soc, name)
		if got, _ := dig(sw.Spec, "boot", "mac"); got != mac {
			t.Errorf("§2.4: %s boot.mac=%q want %q", name, got, mac)
		}
	}
	inb := switchByName(t, docFor(t, docs, "inb-mgmt"), "inb-mgmt-leaf-01")
	if got, _ := dig(inb.Spec, "boot", "mac"); got != "02:95:80:2f:70:b5" {
		t.Errorf("§2.4: inb-mgmt-leaf-01 boot.mac=%q want 02:95:80:2f:70:b5", got)
	}
}

// --- §2.5 portBreakouts / portSpeeds -------------------------------------------

func TestRender_BreakoutSpeedMaps(t *testing.T) {
	docs := renderXOC64(t)
	soc := switchByName(t, docFor(t, docs, "soc-storage-scale-out"), "soc-storage-scale-out-leaf-01")
	pb := strMap(soc.Spec["portBreakouts"])
	wantB := map[string]string{
		"E1/1": "2x400G", "E1/16": "2x400G", // scale_out server zone (2x400G)
		"E1/27": "4x200G", "E1/37": "4x200G", // soc_storage server zone (4x200G)
		"E1/26": "1x800G", "E1/63": "1x800G", // uplink/mesh zones (1x800G)
	}
	for k, v := range wantB {
		if pb[k] != v {
			t.Errorf("§2.5: soc portBreakouts[%s]=%q want %q", k, pb[k], v)
		}
	}
	if _, hasSpeeds := soc.Spec["portSpeeds"]; hasSpeeds {
		t.Errorf("§2.5: ds5000 switch must use portBreakouts only (no portSpeeds)")
	}
	inb := switchByName(t, docFor(t, docs, "inb-mgmt"), "inb-mgmt-leaf-01")
	ps := strMap(inb.Spec["portSpeeds"])
	if ps["E1/1"] != "25G" || ps["E1/24"] != "25G" {
		t.Errorf("§2.5: inb portSpeeds must map E1/1..24 -> 25G (got E1/1=%q E1/24=%q)", ps["E1/1"], ps["E1/24"])
	}
	if _, hasBreakouts := inb.Spec["portBreakouts"]; hasBreakouts {
		t.Errorf("§2.5: ds2000 switch must use portSpeeds only (no portBreakouts)")
	}
}

// --- §2.6 Connection variants (unbundled + mesh) -------------------------------

func TestRender_ConnectionVariants(t *testing.T) {
	docs := renderXOC64(t)
	soc := docFor(t, docs, "soc-storage-scale-out")

	// unbundled server link (breakout lane suffix)
	if !hasUnbundled(soc, "compute-xpu-001/soc_storage-ss0", "soc-storage-scale-out-leaf-01/E1/27/1") {
		t.Errorf("§2.6: missing unbundled soc_storage-ss0 -> ...leaf-01/E1/27/1")
	}
	// mesh links: same port paired leaf-01 <-> leaf-02 on the mesh-zone ports 26,28
	for _, p := range []string{"E1/26", "E1/28"} {
		if !hasMeshLink(soc,
			"soc-storage-scale-out-leaf-01/"+p,
			"soc-storage-scale-out-leaf-02/"+p) {
			t.Errorf("§2.6: missing mesh link leaf-01/%s <-> leaf-02/%s", p, p)
		}
	}
	if countKind(soc, "Connection") != 93 {
		t.Errorf("§2.6: soc fabric Connection count=%d want 93 (92 unbundled + 1 mesh)", countKind(soc, "Connection"))
	}

	inb := docFor(t, docs, "inb-mgmt")
	if !hasUnbundled(inb, "compute-xpu-001/inb_mgmt-mgmt0", "inb-mgmt-leaf-01/E1/1") {
		t.Errorf("§2.6: missing unbundled inb_mgmt-mgmt0 -> inb-mgmt-leaf-01/E1/1 (non-breakout port)")
	}
}
