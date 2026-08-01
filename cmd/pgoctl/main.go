package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/Better-Go-Labs/pgoctl/internal/validate"
)

const version = "0.0.1-wip"

var jsonOutput bool

func main() {
	root := &cobra.Command{
		Use:           "pgoctl",
		Short:         "PGO profile management for Go applications",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "JSON output")
	root.AddCommand(newValidateCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}

func newValidateCmd() *cobra.Command {
	var minSamples int64
	var minDuration float64
	var minScore float64

	cmd := &cobra.Command{
		Use:   "validate <path>",
		Short: "Score a CPU pprof for quality before merging",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := validate.Options{
				MinSamples:  minSamples,
				MinDuration: minDuration,
				MinScore:    minScore,
			}
			report, err := validate.ValidateFile(args[0], opts)
			if err != nil {
				if jsonOutput {
					json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
				} else {
					fmt.Fprintf(os.Stderr, "error: %s\n", err)
				}
				os.Exit(2)
			}

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				enc.Encode(report)
			} else {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "valid\t%v\n", report.Valid)
				fmt.Fprintf(w, "quality_score\t%.3f\n", report.QualityScore)
				fmt.Fprintf(w, "samples\t%d\n", report.Samples)
				fmt.Fprintf(w, "unique_stacks\t%d\n", report.UniqueStacks)
				for _, e := range report.Errors {
					fmt.Fprintf(w, "error\t%s\n", e)
				}
				for _, wn := range report.Warnings {
					fmt.Fprintf(w, "warning\t%s\n", wn)
				}
				w.Flush()
			}

			if !report.Valid {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&minSamples, "min-samples", 10000, "minimum sample count")
	cmd.Flags().Float64Var(&minDuration, "min-duration", 10.0, "minimum duration in seconds")
	cmd.Flags().Float64Var(&minScore, "min-score", 0.6, "minimum quality score (0.0-1.0)")
	return cmd
}
