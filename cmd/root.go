// Package cmd contains the CLI command definitions for HashDiary.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hash-diary",
	Short: "HashDiary - A Solana blockchain diary tool",
	Long:  "HashDiary is a CLI tool that writes encrypted diary entries to the Solana blockchain using the Memo program.",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
