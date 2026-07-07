package wasmhost_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/components"
)

// Issue #59: large kernel responses must survive the JSON-over-linear-memory ABI
// without truncation or trailing/corrupt bytes. Native MoonBit produces complete
// JSON for this shape; the failure is at the wasm boundary.
func TestKernelF2LargeOutputJSONOverWasmBoundary(t *testing.T) {
	const n = 1200
	plan := fmt.Sprintf(`{"switch_classes":[{"switch_class_id":"sw","override_quantity":1,"redundancy":"none","topology_mode":"clos","zones":[{"zone_name":"z","zone_type":"server","port_spec":"1-%d","breakout_logical_ports":1,"allocation_strategy":"sequential"}]}],"server_classes":[{"server_class_id":"srv","quantity":%d}],"connections":[{"connection_id":"c","server_class_id":"srv","server_quantity":%d,"nic_slot_id":"n","port_index":0,"ports_per_connection":1,"speed":1,"distribution":"same-switch","target_switch_class":"sw","target_zone":"z"}]}`, n, n, n)

	kernel, err := components.Kernel()
	if err != nil {
		t.Fatal(err)
	}
	out, err := kernel.Call(components.KernelF2Calculate, []byte(plan))
	if err != nil {
		t.Fatal(err)
	}

	var decoded calc.CalcOutput
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("large F2 output must be valid JSON across wasm boundary: %v", err)
	}
	if len(decoded.Errors) != 0 {
		t.Fatalf("stress plan should be valid, got errors: %+v", decoded.Errors)
	}
	if len(decoded.Endpoints) != n {
		t.Fatalf("decoded endpoints: got %d want %d", len(decoded.Endpoints), n)
	}
}
