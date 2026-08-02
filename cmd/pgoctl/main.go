package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	profiletypes "github.com/Better-Go-Labs/pgoctl/internal/profile"
	"github.com/Better-Go-Labs/pgoctl/internal/validate"
	"github.com/spf13/cobra"
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
	var targetSamples int64
	var targetDuration float64
	var minStackDepth float64

	cmd := &cobra.Command{
		Use:   "validate <path>",
		Short: "Score a CPU pprof for quality before merging",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := validate.Options{
				MinSamples:         minSamples,
				MinDurationSeconds: minDuration,
				MinScore:           minScore,
				TargetSamples:      targetSamples,
				TargetDuration:     targetDuration,
				MinStackDepth:      minStackDepth,
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

			printQualityReport(report, jsonOutput)

			if !report.Valid {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&minSamples, "min-samples", 10000, "minimum sample count")
	cmd.Flags().Float64Var(&minDuration, "min-duration", 10.0, "minimum duration in seconds")
	cmd.Flags().Float64Var(&minScore, "min-score", 0.6, "minimum quality score (0.0-1.0)")
	cmd.Flags().Int64Var(&targetSamples, "target-samples", 50000, "target sample count for quality scoring")
	cmd.Flags().Float64Var(&targetDuration, "target-duration", 30.0, "target duration in seconds for quality scoring")
	cmd.Flags().Float64Var(&minStackDepth, "min-stack-depth", 2.0, "minimum average stack depth")
	return cmd
}

func printQualityReport(report *profiletypes.QualityReport, jsonOutput bool) {
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
}
