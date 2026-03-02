package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newReadCmd() *cobra.Command {
	var (
		flagLimit        int
		flagSince        string
		flagSearch       string
		flagBefore       string
		flagForceRefresh bool
	)

	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read diary entries from blockchain",
		Long:  "Read and display encrypted diary entries from the Solana blockchain.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "not implemented yet")
			return nil
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 7, "Maximum number of entries to display")
	cmd.Flags().StringVar(&flagSince, "since", "", "Show entries since date (e.g. 2026-01-01)")
	cmd.Flags().StringVar(&flagSearch, "search", "", "Search entries by keyword")
	cmd.Flags().StringVar(&flagBefore, "before", "", "Show entries before date (e.g. 2026-03-01)")
	cmd.Flags().BoolVar(&flagForceRefresh, "force-refresh", false, "Bypass cache and fetch from blockchain")

	return cmd
}
