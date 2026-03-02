package cmd

import (
	"fmt"
	"os"

	hdmcp "github.com/hash-diary/hash-diary/internal/mcp"
	"github.com/hash-diary/hash-diary/internal/wallet"
	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run as MCP Server",
		Long:  "Start HashDiary as a Model Context Protocol (MCP) server for AI assistant integration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Resolve keypair path.
			expandedPath := expandPath(flagKeypair)

			// 2. Build password provider (NO interactive prompt).
			//    In MCP mode, stdin/stdout is used for JSON-RPC communication.
			//    An interactive password prompt would corrupt the protocol.
			//    Only --password flag or HASH_DIARY_PASSWORD env var are accepted.
			var providers []wallet.PasswordProvider
			if flagPassword != "" {
				providers = append(providers, wallet.StaticPassword(flagPassword))
			}
			providers = append(providers, wallet.EnvPassword("HASH_DIARY_PASSWORD"))

			// If the wallet is encrypted and no password source is available,
			// we need to detect this early and exit with a clear error to stderr.
			provider := wallet.ChainedProvider(providers...)

			// 3. Load wallet.
			kp, err := wallet.LoadWallet(expandedPath, provider)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to load wallet: %s\n", err)
				fmt.Fprintf(os.Stderr, "Hint: for encrypted wallets in MCP mode, provide password via --password flag or HASH_DIARY_PASSWORD env var\n")
				return err
			}

			fmt.Fprintf(os.Stderr, "[mcp] wallet loaded: %s\n", kp.Address())
			fmt.Fprintf(os.Stderr, "[mcp] endpoint: %s\n", flagURL)
			fmt.Fprintf(os.Stderr, "[mcp] starting MCP server on stdio...\n")

			// 4. Create MCP server.
			srv := hdmcp.NewServer(kp, flagURL)

			// 5. Run server on stdio.
			return srv.Run()
		},
	}

	return cmd
}
