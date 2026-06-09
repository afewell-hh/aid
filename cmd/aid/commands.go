package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/afewell-hh/aid/internal/orchestrate"
	"github.com/afewell-hh/aid/internal/plan"
	"github.com/spf13/cobra"
)

// (RED-phase errNotImplemented removed — all commands are wired.)

// loadPlanJSON reads a plan file and returns the snake_case plan JSON the kernel
// expects. YAML is the canonical input (D9); a .json file is passed through.
func loadPlanJSON(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(path, ".json") {
		return raw, nil
	}
	return plan.YAMLToJSON(raw)
}

// newRootCmd builds the full command tree: the four subcommands + serve stub.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "aid",
		Short:         "AID — AI-cluster topology design tool",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newPlanCmd(), newTopologyCmd(), newExportCmd(), newServeCmd())
	return root
}

func newPlanCmd() *cobra.Command {
	plan := &cobra.Command{Use: "plan", Short: "Plan operations"}
	validate := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a plan; print a human-readable error per constraint violation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planJSON, err := loadPlanJSON(args[0])
			if err != nil {
				return err
			}
			res, err := orchestrate.Validate(planJSON)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if res.IsValid {
				fmt.Fprintln(out, "✓ plan is valid")
				if len(res.Warnings) > 0 {
					for _, w := range res.Warnings {
						fmt.Fprintf(out, "  ⚠ [%s] %s\n", w.Code, w.Message)
					}
				}
				return nil
			}
			for _, e := range res.Errors {
				fmt.Fprintf(out, "✗ [%s] %s\n", e.Code, e.Message)
			}
			return fmt.Errorf("plan is invalid: %d error(s)", len(res.Errors))
		},
	}
	plan.AddCommand(validate)
	return plan
}

func newTopologyCmd() *cobra.Command {
	topo := &cobra.Command{Use: "topology", Short: "Topology calculation"}

	var calcOut string
	calc := &cobra.Command{
		Use:   "calc <file>",
		Short: "Run the kernel; emit IR (+ validation summary)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planJSON, err := loadPlanJSON(args[0])
			if err != nil {
				return err
			}
			res, err := orchestrate.Calculate(planJSON)
			if err != nil {
				return err
			}
			if res.Ok == nil {
				return fmt.Errorf("kernel rejected plan: %s", string(res.Err))
			}
			ir := res.Ok.IR
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "topology: %d nodes, %d edges, %d fabric(s) — valid: %v\n",
				len(ir.Nodes), len(ir.Edges), len(ir.Fabrics), res.Ok.Validation.IsValid)
			for _, e := range res.Ok.Validation.Errors {
				fmt.Fprintf(out, "  ✗ [%s] %s\n", e.Code, e.Message)
			}
			if calcOut != "" {
				irJSON, err := json.MarshalIndent(ir, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(calcOut, append(irJSON, '\n'), 0o644); err != nil {
					return err
				}
				fmt.Fprintf(out, "wrote IR to %s\n", calcOut)
			}
			return nil
		},
	}
	calc.Flags().StringVar(&calcOut, "output", "", "write topology IR JSON to this file")

	var bomFormat string
	bom := &cobra.Command{
		Use:   "bom <file>",
		Short: "Kernel -> BOM adapter (CSV or JSON)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planJSON, err := loadPlanJSON(args[0])
			if err != nil {
				return err
			}
			out, err := orchestrate.ExportBOM(planJSON, bomFormat)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out.Content)
			if !strings.HasSuffix(out.Content, "\n") {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	bom.Flags().StringVar(&bomFormat, "format", "csv", "csv|json")

	topo.AddCommand(calc, bom)
	return topo
}

func newExportCmd() *cobra.Command {
	export := &cobra.Command{Use: "export", Short: "Export adapters"}
	var fabric string
	wiring := &cobra.Command{
		Use:   "wiring <file>",
		Short: "Kernel -> hhfab adapter; wiring YAML per fabric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planJSON, err := loadPlanJSON(args[0])
			if err != nil {
				return err
			}
			docs, err := orchestrate.ExportWiring(planJSON, fabric)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for i, d := range docs {
				if i > 0 {
					fmt.Fprintln(out, "---")
				}
				fmt.Fprint(out, d.YAML)
				if !strings.HasSuffix(d.YAML, "\n") {
					fmt.Fprintln(out)
				}
			}
			return nil
		},
	}
	wiring.Flags().StringVar(&fabric, "fabric", "", "restrict to one fabric by name")
	export.AddCommand(wiring)
	return export
}
