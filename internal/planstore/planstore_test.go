package planstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestIDSanitization locks the traversal defense: an id only ever names a single
// file inside the plans dir. Unsafe ids must never reach the filesystem.
func TestIDSanitization(t *testing.T) {
	unsafe := []string{
		"", "..", ".", "../x", "a/b", "/abs", "a.b", "a b",
		"../../etc/passwd", "foo/../bar", ".hidden", "a..b",
	}
	for _, id := range unsafe {
		if validID(id) {
			t.Errorf("validID(%q) = true, want false (unsafe)", id)
		}
	}
	safe := []string{"clos-small", "switch_bom", "Plan1", "a", "x-y-z", "123"}
	for _, id := range safe {
		if !validID(id) {
			t.Errorf("validID(%q) = false, want true (safe)", id)
		}
	}
}

// TestCreate_VendoredOraclePlans exercises the REAL create path with the real
// DIET plan shape: identity nested under meta.case_id / meta.name (no top-level
// id/name). This is the path the REST POST /api/plans and the GUI use — and the
// one the F7b integration tests bypassed by seeding the store dir directly, which
// hid that Create rejected every real plan with "plan has no id or name".
func TestCreate_VendoredOraclePlans(t *testing.T) {
	cases := map[string]string{
		"../../tests/oracle/xoc-64-mesh-conv-ro/training.yaml":          "training_xoc64_1xopg64_mesh_conv_ro",
		"../../tests/oracle/xoc-128-2xopg64-mesh-conv-ro/training.yaml": "training_xoc128_2xopg64_mesh_conv_ro",
		"../../tests/oracle/xoc-256-2xopg128-clos-ro/training.yaml":     "training_xoc256_2xopg128_clos_ro",
	}
	for path, wantID := range cases {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			s, err := Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			p, err := s.Create(b)
			if err != nil {
				t.Fatalf("Create(real DIET plan) failed (the canonical meta.case_id identity must be honored): %v", err)
			}
			if p.ID != wantID {
				t.Errorf("Create id = %q, want %q (from meta.case_id)", p.ID, wantID)
			}
			if p.Name == "" {
				t.Errorf("Create name empty, want meta.name resolved")
			}
			// Round-trips: the created plan must be listable and gettable by its id.
			got, err := s.Get(p.ID)
			if err != nil {
				t.Fatalf("Get(%q) after Create: %v", p.ID, err)
			}
			if got.YAML == "" {
				t.Errorf("Get returned empty YAML")
			}
			list, err := s.List()
			if err != nil || len(list) != 1 || list[0].ID != wantID {
				t.Errorf("List after Create = %+v (err %v), want one plan id %q", list, err, wantID)
			}
		})
	}
}

// TestStoreCRUDRoundtrip exercises Create→Get→List→Update→Delete on disk.
func TestStoreCRUDRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	p, err := s.Create([]byte("id: demo\nname: Demo Plan\nstatus: draft\n"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID != "demo" || p.Name != "Demo Plan" {
		t.Errorf("Create summary: %+v", p)
	}
	if _, err := os.Stat(filepath.Join(dir, "demo.yaml")); err != nil {
		t.Errorf("Create did not write demo.yaml: %v", err)
	}

	got, err := s.Get("demo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.YAML == "" || got.Status != "draft" {
		t.Errorf("Get detail missing yaml/status: %+v", got)
	}

	plans, err := s.List()
	if err != nil || len(plans) != 1 {
		t.Fatalf("List: %v, n=%d", err, len(plans))
	}
	if plans[0].YAML != "" {
		t.Errorf("List summary should omit yaml, got %q", plans[0].YAML)
	}

	if _, err := s.Update("demo", []byte("id: demo\nname: Renamed\nstatus: active\n")); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = s.Get("demo")
	if got.Name != "Renamed" || got.Status != "active" {
		t.Errorf("Update not persisted: %+v", got)
	}

	if err := s.Delete("demo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("demo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete: want ErrNotFound, got %v", err)
	}
}

// TestStoreErrors covers the sentinel error mapping at the store layer.
func TestStoreErrors(t *testing.T) {
	s, _ := Open(t.TempDir())

	if _, err := s.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get missing: want ErrNotFound, got %v", err)
	}
	if _, err := s.Update("missing", []byte("id: missing\nname: X\n")); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update missing: want ErrNotFound, got %v", err)
	}
	if err := s.Delete("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete missing: want ErrNotFound, got %v", err)
	}
	if _, err := s.Get("../escape"); !errors.Is(err, ErrInvalidID) {
		t.Errorf("Get traversal: want ErrInvalidID, got %v", err)
	}
	if _, err := s.Create([]byte("id: ../../pwned\nname: Evil\n")); !errors.Is(err, ErrInvalidID) {
		t.Errorf("Create traversal id: want ErrInvalidID, got %v", err)
	}
	if _, err := s.Create([]byte("\t\tnot: [valid")); !errors.Is(err, ErrInvalidPlan) {
		t.Errorf("Create malformed: want ErrInvalidPlan, got %v", err)
	}
}
