package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newExplainCmd() *cobra.Command {
	var topN int

	cmd := &cobra.Command{
		Use:   "explain <path>",
		Short: "Explain a CPU profile: top functions, hot packages, PGO readiness",
		Long: `Analyse a pprof file and explain what it contains in human-readable form.

Prints the top hot functions by flat CPU share, groups them by package,
and gives a plain-English verdict on whether the profile is a good PGO
baseline (mirrors the validate gate but with prose output instead of a
score).

Designed for interactive use: pipe through less or redirect to a file.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// D7: implementation goes here.
			fmt.Fprintf(cmd.OutOrStdout(), "explain %s: not yet implemented\n", args[0])
			return nil
		},
	}
	cmd.Flags().IntVar(&topN, "top", 20, "number of top functions to show")
	return cmd
}
