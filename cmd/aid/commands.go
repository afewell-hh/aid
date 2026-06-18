package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/afewell-hh/aid/internal/bom"
	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/design"
	"github.com/spf13/cobra"
)

// F7a: the four commands route through internal/design (the rebuilt engine), not
// internal/orchestrate (retired in F7d). Input is a DIET/training bundle (HNP's
// authoring format, D25) + an optional AID optic overlay via --overlay.

// resolve reads the training bundle (+ optional overlay) and runs the coordinator.
// A structural failure (unparseable / unresolved / kernel infra) is returned as a
// Go error; calc constraint violations come back as data on the Resolved.
func resolve(file, overlay string) (*design.Resolved, error) {
	training, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	in := design.Inputs{TrainingYAML: training}
	if overlay != "" {
		ov, err := os.ReadFile(overlay)
		if err != nil {
			return nil, err
		}
		in.OverlayYAML = ov
	}
	return design.Resolve(in)
}

// printViolations writes the calc constraint violations and returns the invalid
// error (shared by the commands that refuse to proceed on an invalid plan).
func printViolations(cmd *cobra.Command, res *design.Resolved) error {
	out := cmd.OutOrStdout()
	for _, e := range res.Calc.Errors {
		fmt.Fprintf(out, "✗ [%s] %s\n", e.Code, e.Message)
	}
	return fmt.Errorf("plan is invalid: %d error(s)", len(res.Calc.Errors))
}

// printCalcSummary writes the CalcOutput surface for a valid plan: per-class
// switch/server quantities plus an endpoint count and a transceiver-verdict
// breakdown by outcome (note §3.1). Shared by `topology calc` and `aid design`.
func printCalcSummary(out io.Writer, res *design.Resolved) {
	fmt.Fprintln(out, "switch quantities:")
	for _, q := range sortedQty(res.Calc.SwitchQuantity) {
		fmt.Fprintf(out, "  %s: %d\n", q.ClassID, q.Quantity)
	}
	fmt.Fprintln(out, "server quantities:")
	for _, q := range sortedQty(res.Calc.ServerQuantity) {
		fmt.Fprintf(out, "  %s: %d\n", q.ClassID, q.Quantity)
	}
	fmt.Fprintf(out, "endpoints: %d\n", len(res.Calc.Endpoints))

	byOutcome := map[string]int{}
	for _, v := range res.Calc.TransceiverVerdicts {
		byOutcome[v.Outcome]++
	}
	fmt.Fprintf(out, "transceiver verdicts: %d", len(res.Calc.TransceiverVerdicts))
	if len(byOutcome) > 0 {
		outcomes := make([]string, 0, len(byOutcome))
		for o := range byOutcome {
			outcomes = append(outcomes, o)
		}
		sort.Strings(outcomes)
		parts := make([]string, 0, len(outcomes))
		for _, o := range outcomes {
			parts = append(parts, fmt.Sprintf("%s: %d", o, byOutcome[o]))
		}
		fmt.Fprintf(out, " (%s)", strings.Join(parts, ", "))
	}
	fmt.Fprintln(out)
}

// bomDataLines counts the BOM data rows (excluding the header and the
// "# suppressed_..." footer) for the design summary.
func bomDataLines(rows [][]string) int {
	n := 0
	for i, r := range rows {
		if i == 0 {
			continue // header
		}
		if len(r) > 0 && strings.HasPrefix(r[0], "#") {
			continue // footer note
		}
		n++
	}
	return n
}

// csvString renders CSV rows as text. The committed bom.csv files contain no
// quoted fields, so encoding/csv (which quotes only when required) reproduces them
// byte-for-byte.
func csvString(rows [][]string) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.WriteAll(rows); err != nil {
		return "", err
	}
	return buf.String(), w.Error()
}

// newRootCmd builds the full command tree: the four subcommands + serve.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "aid",
		Short:         "AID — AI-cluster topology design tool",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newPlanCmd(), newTopologyCmd(), newExportCmd(), newServeCmd(), newDesignCmd())
	return root
}

// newDesignCmd is the one-shot coordinator surface (note §5): resolve a plan
// end-to-end through the rebuilt engine and print a consolidated summary —
// validity, computed quantities + endpoint/verdict summary, BOM line count, and
// the managed wiring fabrics.
func newDesignCmd() *cobra.Command {
	var overlay string
	cmd := &cobra.Command{
		Use:   "design <file>",
		Short: "Resolve a plan end-to-end (quantities + BOM + wiring summary)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := resolve(args[0], overlay)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if !res.Valid() {
				fmt.Fprintln(out, "✗ plan is invalid")
				return printViolations(cmd, res)
			}
			fmt.Fprintln(out, "✓ plan is valid")
			printCalcSummary(out, res)
			rows, err := bom.RenderProjection(res.BOM)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "bom: %d line(s)\n", bomDataLines(rows))
			docs, err := res.Wiring("")
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "wiring fabrics: %d\n", len(docs))
			for _, d := range docs {
				fmt.Fprintf(out, "  %s\n", d.Fabric)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&overlay, "overlay", "", "AID optic/identity overlay (YAML)")
	return cmd
}

func newPlanCmd() *cobra.Command {
	plan := &cobra.Command{Use: "plan", Short: "Plan operations"}
	var overlay string
	validate := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a plan; print a human-readable error per constraint violation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := resolve(args[0], overlay)
			if err != nil {
				return err
			}
			if res.Valid() {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ plan is valid")
				return nil
			}
			return printViolations(cmd, res)
		},
	}
	validate.Flags().StringVar(&overlay, "overlay", "", "AID optic/identity overlay (YAML)")
	plan.AddCommand(validate)
	return plan
}

func newTopologyCmd() *cobra.Command {
	topo := &cobra.Command{Use: "topology", Short: "Topology calculation"}

	var calcOverlay string
	calcCmd := &cobra.Command{
		Use:   "calc <file>",
		Short: "Run the rebuilt engine; print computed switch/server quantities",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := resolve(args[0], calcOverlay)
			if err != nil {
				return err
			}
			if !res.Valid() {
				return printViolations(cmd, res)
			}
			printCalcSummary(cmd.OutOrStdout(), res)
			return nil
		},
	}
	calcCmd.Flags().StringVar(&calcOverlay, "overlay", "", "AID optic/identity overlay (YAML)")

	var bomOverlay, bomFormat string
	var bomFull bool
	bomCmd := &cobra.Command{
		Use:   "bom <file>",
		Short: "Compute the BOM through the rebuilt engine (CSV or JSON)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := resolve(args[0], bomOverlay)
			if err != nil {
				return err
			}
			if !res.Valid() {
				return printViolations(cmd, res)
			}
			out := cmd.OutOrStdout()
			switch bomFormat {
			case "json":
				b, err := bom.RenderJSON(res.BOM, bomFull)
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
			case "csv", "":
				rows, err := renderBOMRows(res.BOM, bomFull)
				if err != nil {
					return err
				}
				s, err := csvString(rows)
				if err != nil {
					return err
				}
				fmt.Fprint(out, s)
			default:
				return fmt.Errorf("unknown format %q (csv|json)", bomFormat)
			}
			return nil
		},
	}
	bomCmd.Flags().StringVar(&bomOverlay, "overlay", "", "AID optic/identity overlay (YAML)")
	bomCmd.Flags().StringVar(&bomFormat, "format", "csv", "csv|json")
	bomCmd.Flags().BoolVar(&bomFull, "full", false, "render the full purchasable BOM instead of the projection")

	topo.AddCommand(calcCmd, bomCmd)
	return topo
}

func newExportCmd() *cobra.Command {
	export := &cobra.Command{Use: "export", Short: "Export adapters"}
	var fabric, overlay string
	wiring := &cobra.Command{
		Use:   "wiring <file>",
		Short: "Render hhfab wiring YAML per managed fabric through the rebuilt engine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := resolve(args[0], overlay)
			if err != nil {
				return err
			}
			docs, err := res.Wiring(fabric)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for i, d := range docs {
				if i > 0 {
					fmt.Fprintln(out, "---")
				}
				y := string(d.YAML)
				fmt.Fprint(out, y)
				if !strings.HasSuffix(y, "\n") {
					fmt.Fprintln(out)
				}
			}
			return nil
		},
	}
	wiring.Flags().StringVar(&fabric, "fabric", "", "restrict to one managed fabric by name")
	wiring.Flags().StringVar(&overlay, "overlay", "", "AID optic/identity overlay (YAML)")
	export.AddCommand(wiring)
	return export
}

// renderBOMRows picks the projection or full-BOM renderer.
func renderBOMRows(m *bom.ResolvedModel, full bool) ([][]string, error) {
	if full {
		return bom.RenderFullBOM(m)
	}
	return bom.RenderProjection(m)
}

// sortedQty returns the class quantities sorted by class id for deterministic CLI
// output (the kernel emits them in allocation order).
func sortedQty(qs []calc.ClassQty) []calc.ClassQty {
	out := append([]calc.ClassQty(nil), qs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ClassID < out[j].ClassID })
	return out
}
