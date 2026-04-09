package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/sglavoie/dev-helpers/go/jsonc2json/internal/strip"
	"github.com/spf13/cobra"
)

var outputFile string

var rootCmd = &cobra.Command{
	Use:          "jsonc2json [file]",
	Short:        "Strip comments and trailing commas from JSONC files",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var input []byte
		var err error

		if len(args) == 1 {
			input, err = os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
		} else {
			input, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
		}

		result, err := strip.Strip(input)
		if err != nil {
			return fmt.Errorf("stripping JSONC: %w", err)
		}

		if outputFile != "" {
			if err := os.WriteFile(outputFile, result, 0644); err != nil {
				return fmt.Errorf("writing output file: %w", err)
			}
		} else {
			if _, err := os.Stdout.Write(result); err != nil {
				return fmt.Errorf("writing stdout: %w", err)
			}
		}

		return nil
	},
}

// Execute runs the root command.
func Execute() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file path (default: stdout)")
}
