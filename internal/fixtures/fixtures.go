// Package fixtures locates the repo's test fixtures from any test working
// directory (via runtime.Caller).
package fixtures

import (
	"os"
	"path/filepath"
	"runtime"
)

// Root returns the repository root (this file is at <root>/internal/fixtures/).
func Root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// PlanJSON reads tests/fixtures/<kind>/<name>/plan.json.
func PlanJSON(kind, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(Root(), "tests", "fixtures", kind, name, "plan.json"))
}

// PlanYAML reads tests/fixtures/<kind>/<name>/plan.yaml.
func PlanYAML(kind, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(Root(), "tests", "fixtures", kind, name, "plan.yaml"))
}

// PlanYAMLPath returns the path to tests/fixtures/<kind>/<name>/plan.yaml.
func PlanYAMLPath(kind, name string) string {
	return filepath.Join(Root(), "tests", "fixtures", kind, name, "plan.yaml")
}

// Expected reads tests/fixtures/valid/<name>/expected.json.
func Expected(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(Root(), "tests", "fixtures", "valid", name, "expected.json"))
}

// (F7d retired VendoredIR/VendoredBoms — they read the now-deleted
// hhfab-adapter/ and bom-adapter/ testdata; no live caller remained — #64/#35.)
