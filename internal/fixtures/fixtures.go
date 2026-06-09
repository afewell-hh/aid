// Package fixtures locates the repo's test fixtures and vendored adapter
// testdata from any test working directory (via runtime.Caller).
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

// VendoredIR reads hhfab-adapter/tests/testdata/<name>.ir.json (the kernel
// encoder must reproduce these bytes — IR_CONTRACT.md).
func VendoredIR(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(Root(), "hhfab-adapter", "tests", "testdata", name+".ir.json"))
}

// VendoredBoms reads bom-adapter/tests/testdata/<name>.boms.json (the
// device-class-bom[] the bom adapter consumes — BOM_CONTRACT.md).
func VendoredBoms(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(Root(), "bom-adapter", "tests", "testdata", name+".boms.json"))
}
