// Command aid is the AID CLI: it validates plans, calculates topology, and
// exports BOM and hhfab wiring by orchestrating the three WASM components over
// the D16 boundary. Single static binary (D4).
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
