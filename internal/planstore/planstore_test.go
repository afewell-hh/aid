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
