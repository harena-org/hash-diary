package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWriteCmd() *cobra.Command {
	var flagNoWait bool

	cmd := &cobra.Command{
		Use:   "write [text]",
		Short: "Write a diary entry to blockchain",
		Long:  "Write an encrypted diary entry to the Solana blockchain using the Memo program.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "not implemented yet")
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagNoWait, "no-wait", false, "Don't wait for transaction confirmation")

	return cmd
}
