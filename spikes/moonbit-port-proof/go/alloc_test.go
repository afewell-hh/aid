// Go runtime harness for the AID MoonBit feasibility spike (issue #5).
//
// It loads the MoonBit-produced core-wasm module (testdata/alloc.wasm, built
// from ../src by build.sh) through wasmtime-go, calls the exported `allocate`
// and `non_overlap_holds` functions, and measures cold-start and per-call
// latency against the project gates (cold-start < 500ms, per-call < 10ms).
package goharness

import (
	"os"
	"testing"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v45"
)

const wasmPath = "testdata/alloc.wasm"

// harness holds the instantiated module and its exported functions.
type harness struct {
	store      *wasmtime.Store
	allocate   *wasmtime.Func
	nonOverlap *wasmtime.Func
}

// load performs a full cold instantiation (engine + compile + instantiate +
// resolve exports) and returns the harness plus the measured cold-start time.
func load(t testing.TB) (*harness, time.Duration) {
	t.Helper()
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Fatalf("read wasm: %v", err)
	}
	start := time.Now()
	engine := wasmtime.NewEngine()
	module, err := wasmtime.NewModule(engine, wasmBytes)
	if err != nil {
		t.Fatalf("compile module: %v", err)
	}
	store := wasmtime.NewStore(engine)
	instance, err := wasmtime.NewInstance(store, module, []wasmtime.AsExtern{})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	allocate := instance.GetFunc(store, "allocate")
	if allocate == nil {
		t.Fatal("export 'allocate' not found")
	}
	nonOverlap := instance.GetFunc(store, "non_overlap_holds")
	if nonOverlap == nil {
		t.Fatal("export 'non_overlap_holds' not found")
	}
	// Cold start = everything required to make the first call ready.
	coldStart := time.Since(start)
	return &harness{store: store, allocate: allocate, nonOverlap: nonOverlap}, coldStart
}

func (h *harness) callAllocate(t testing.TB, capacity, requested int32) int32 {
	res, err := h.allocate.Call(h.store, capacity, requested)
	if err != nil {
		t.Fatalf("call allocate: %v", err)
	}
	return res.(int32)
}

// TestCorrectness confirms the MoonBit-compiled allocator, called from Go,
// matches the proven model.
func TestCorrectness(t *testing.T) {
	h, _ := load(t)
	cases := []struct {
		cap, req, want int32
	}{
		{48, 10, 10},  // fits -> full demand
		{48, 100, 48}, // oversubscribed -> clamps to capacity
		{48, 0, 0},    // zero demand
		{32, 32, 32},  // exact capacity
	}
	for _, c := range cases {
		if got := h.callAllocate(t, c.cap, c.req); got != c.want {
			t.Errorf("allocate(%d,%d)=%d, want %d", c.cap, c.req, got, c.want)
		}
	}

	// non_overlap_holds returns bool-as-i32 (1) for distinct offsets.
	res, err := h.nonOverlap.Call(h.store, int32(16), int32(0), int32(31))
	if err != nil {
		t.Fatalf("call non_overlap_holds: %v", err)
	}
	if res.(int32) != 1 {
		t.Errorf("non_overlap_holds(16,0,31)=%v, want 1", res)
	}
}

// TestColdStart measures one full cold instantiation and gates it < 500ms.
func TestColdStart(t *testing.T) {
	_, cold := load(t)
	t.Logf("cold-start: %v", cold)
	if cold > 500*time.Millisecond {
		t.Errorf("cold-start %v exceeds 500ms gate", cold)
	}
}

// TestPerCallLatency measures repeated calls after instantiation and gates the
// average < 10ms.
func TestPerCallLatency(t *testing.T) {
	h, cold := load(t)
	const n = 1000
	var min, max, total time.Duration
	min = time.Hour
	for i := 0; i < n; i++ {
		s := time.Now()
		_ = h.callAllocate(t, 48, int32(i%96))
		d := time.Since(s)
		total += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	avg := total / n
	t.Logf("cold-start: %v", cold)
	t.Logf("per-call over %d calls: min=%v avg=%v max=%v total=%v", n, min, avg, max, total)
	if avg > 10*time.Millisecond {
		t.Errorf("per-call avg %v exceeds 10ms gate", avg)
	}
}

// BenchmarkAllocate provides an independent throughput measurement.
func BenchmarkAllocate(b *testing.B) {
	h, _ := load(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.callAllocate(b, 48, int32(i%96))
	}
}
