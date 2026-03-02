package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWalletCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wallet",
		Short: "Wallet management commands",
		Long:  "Parent command for wallet management. Use subcommands to create, show, import, or fund wallets.",
	}

	cmd.AddCommand(newWalletNewCmd())
	cmd.AddCommand(newWalletShowCmd())
	cmd.AddCommand(newWalletAirdropCmd())
	cmd.AddCommand(newWalletImportCmd())

	return cmd
}

func newWalletNewCmd() *cobra.Command {
	var (
		flagForce      bool
		flagNoPassword bool
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new wallet keypair",
		Long:  "Generate a new Solana keypair and save it to the configured keypair path.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "not implemented yet")
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing keypair file")
	cmd.Flags().BoolVar(&flagNoPassword, "no-password", false, "Create keypair without password encryption")

	return cmd
}

func newWalletShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show wallet address and balance",
		Long:  "Display the public address and SOL balance of the configured wallet.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "not implemented yet")
			return nil
		},
	}

	return cmd
}

func newWalletAirdropCmd() *cobra.Command {
	var flagAmount float64

	cmd := &cobra.Command{
		Use:   "airdrop",
		Short: "Request test SOL (devnet/testnet only)",
		Long:  "Request an airdrop of test SOL tokens on devnet or testnet.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "not implemented yet")
			return nil
		},
	}

	cmd.Flags().Float64Var(&flagAmount, "amount", 1, "Amount of SOL to request")

	return cmd
}

func newWalletImportCmd() *cobra.Command {
	var flagPrivateKey string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import an existing wallet",
		Long:  "Import an existing Solana wallet from a private key.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "not implemented yet")
			return nil
		},
	}

	cmd.Flags().StringVar(&flagPrivateKey, "private-key", "", "Base58-encoded private key to import")

	return cmd
}
