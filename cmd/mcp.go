package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run as MCP Server",
		Long:  "Start HashDiary as a Model Context Protocol (MCP) server for AI assistant integration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "not implemented yet")
			return nil
		},
	}

	return cmd
}
