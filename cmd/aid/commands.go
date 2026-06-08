package main

import (
	"errors"

	"github.com/spf13/cobra"
)

// errNotImplemented marks RED-phase command stubs.
var errNotImplemented = errors.New("not implemented (RED)")

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
		RunE:  func(cmd *cobra.Command, args []string) error { return errNotImplemented },
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
		RunE:  func(cmd *cobra.Command, args []string) error { return errNotImplemented },
	}
	calc.Flags().StringVar(&calcOut, "output", "", "write topology IR JSON to this file")

	var bomFormat string
	bom := &cobra.Command{
		Use:   "bom <file>",
		Short: "Kernel -> BOM adapter (CSV or JSON)",
		Args:  cobra.ExactArgs(1),
		RunE:  func(cmd *cobra.Command, args []string) error { return errNotImplemented },
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
		RunE:  func(cmd *cobra.Command, args []string) error { return errNotImplemented },
	}
	wiring.Flags().StringVar(&fabric, "fabric", "", "restrict to one fabric by name")
	export.AddCommand(wiring)
	return export
}
