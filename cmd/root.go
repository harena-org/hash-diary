// Package cmd contains the CLI command definitions for HashDiary.
package cmd

import (
	"fmt"
	"os"

	"github.com/hash-diary/hash-diary/internal/output"
	"github.com/spf13/cobra"
)

// Global flag variables.
var (
	flagURL      string
	flagKeypair  string
	flagPassword string
	flagFormat   string
	flagVerbose  bool
)

var rootCmd = &cobra.Command{
	Use:     "hash-diary",
	Short:   "HashDiary - A Solana blockchain diary tool",
	Long:    "HashDiary is a CLI tool that writes encrypted diary entries to the Solana blockchain using the Memo program.",
	Version: "0.1.0",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagURL, "url", "u", "https://api.devnet.solana.com", "Solana RPC endpoint URL")
	rootCmd.PersistentFlags().StringVar(&flagKeypair, "keypair", "~/.hash-diary/id.json", "Wallet keypair file path")
	rootCmd.PersistentFlags().StringVar(&flagPassword, "password", "", "Wallet password for encrypted keypairs")
	rootCmd.PersistentFlags().StringVar(&flagFormat, "format", "text", "Output format: text | json")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Show detailed logs")

	// Register subcommands.
	rootCmd.AddCommand(newWalletCmd())
	rootCmd.AddCommand(newWriteCmd())
	rootCmd.AddCommand(newReadCmd())
	rootCmd.AddCommand(newMCPCmd())
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newFormatter creates an output.Formatter from the current global flags.
func newFormatter() *output.Formatter {
	return &output.Formatter{
		Format:    flagFormat,
		Verbose:   flagVerbose,
		Writer:    os.Stdout,
		ErrWriter: os.Stderr,
	}
}
