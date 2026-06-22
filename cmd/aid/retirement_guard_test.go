package main

// F7d retirement guard (Issue #64 / #35). This pins the END-STATE of the
// deletion phase so the old orchestrate + Rust-adapter path cannot creep back:
// the retired dirs/files are gone, nothing live imports the retired packages, and
// the components package no longer exposes the old adapter surface — while the
// proved kernel + the rebuilt engine are kept.
//
// RED: fails now because the old path still exists. GREEN: passes once F7d deletes
// it. The real proof that nothing live depended on the removed code is the full
// scoped suite + the oracle suite (mesh + Clos, real hhfab validate) + moon prove
// all staying green AFTER deletion.

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// retiredPaths must NOT exist after F7d.
var retiredPaths = []string{
	"internal/orchestrate",
	"internal/plan",
	"hhfab-adapter",
	"bom-adapter",
	"embed/hhfab.wasm",
	"embed/bom.wasm",
}

// keptPaths must survive F7d (the proved kernel + the rebuilt engine).
var keptPaths = []string{
	"embed/kernel.wasm",
	"internal/design",
	"internal/calc",
	"internal/wiring",
	"internal/wasmhost",
	"kernel/proofs",
}

// retiredImports must not be imported by any live (non-gitignored) .go file.
var retiredImports = []string{
	"github.com/afewell-hh/aid/internal/orchestrate",
	"github.com/afewell-hh/aid/internal/plan",
}

// retiredComponentSymbols must no longer appear in internal/components.
var retiredComponentSymbols = []string{
	"func Hhfab(", "func Bom(",
	"KernelCalculate", "KernelValidate", "HhfabExport", "BomExport",
}

// retiredBuildConfig pins the CI/Makefile retirement surfaces the F7 note lists
// for F7d: the Rust-adapter cargo-test steps + cache, the hhfab-wasm/bom-wasm Make
// targets, the embed-check entries for those two, and the old orchestrate
// golden-path CI gate (re-pointed at the oracle wiring test). Each token must be
// ABSENT from the file after F7d.
var retiredBuildConfig = map[string][]string{
	"Makefile": {"hhfab-wasm", "bom-wasm", "hhfab-adapter", "bom-adapter", "hhfab.wasm", "bom.wasm"},
	filepath.Join(".github", "workflows", "ci.yml"): {
		"hhfab-adapter", "bom-adapter", "cargo test",
		"TestGolden_WiringValidatesWithHhfab", "internal/orchestrate",
	},
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	return root
}

func TestRetirement_OldPathRemoved(t *testing.T) {
	root := repoRoot(t)

	for _, p := range retiredPaths {
		if _, err := os.Stat(filepath.Join(root, p)); !os.IsNotExist(err) {
			t.Errorf("retired path still present (F7d must delete it): %s", p)
		}
	}
	for _, p := range keptPaths {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("kept path missing (F7d must NOT remove it): %s (%v)", p, err)
		}
	}

	// No live .go file may import a retired package. (Compilation already enforces
	// this once the dirs are gone; this also catches a stray re-add.)
	const self = "retirement_guard_test.go"
	fset := token.NewFileSet()
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "gitignored" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, self) {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		for _, imp := range f.Imports {
			ip := strings.Trim(imp.Path.Value, `"`)
			for _, r := range retiredImports {
				if ip == r {
					rel, _ := filepath.Rel(root, path)
					t.Errorf("%s imports retired package %q", rel, r)
				}
			}
		}
		return nil
	})

	// The components package must no longer expose the old adapter surface.
	b, err := os.ReadFile(filepath.Join(root, "internal", "components", "components.go"))
	if err != nil {
		t.Fatalf("read components.go: %v", err)
	}
	for _, sym := range retiredComponentSymbols {
		if strings.Contains(string(b), sym) {
			t.Errorf("components.go still references retired symbol %q (F7d must remove it)", sym)
		}
	}

	// The fixtures package must no longer expose the retired adapter-testdata
	// helpers (they read the deleted hhfab-adapter/ + bom-adapter/ dirs, so they
	// are dead exported APIs guaranteed to fail).
	fb, err := os.ReadFile(filepath.Join(root, "internal", "fixtures", "fixtures.go"))
	if err != nil {
		t.Fatalf("read fixtures.go: %v", err)
	}
	for _, sym := range []string{"func VendoredIR", "func VendoredBoms"} {
		if strings.Contains(string(fb), sym) {
			t.Errorf("fixtures.go still defines retired helper %q (F7d must remove it)", sym)
		}
	}

	// CI/Makefile must no longer carry the adapter build/test surfaces or the old
	// orchestrate golden-path gate.
	for rel, tokens := range retiredBuildConfig {
		cb, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Errorf("read %s: %v", rel, err)
			continue
		}
		for _, tok := range tokens {
			if strings.Contains(string(cb), tok) {
				t.Errorf("%s still contains retired build-config token %q (F7d must remove it)", rel, tok)
			}
		}
	}
}
