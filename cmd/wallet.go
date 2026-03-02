package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hash-diary/hash-diary/internal/rpc"
	"github.com/hash-diary/hash-diary/internal/wallet"
	"github.com/spf13/cobra"
)

// expandPath expands a leading ~ to the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// buildPasswordProvider creates a chained password provider from global flags.
func buildPasswordProvider() wallet.PasswordProvider {
	var providers []wallet.PasswordProvider
	if flagPassword != "" {
		providers = append(providers, wallet.StaticPassword(flagPassword))
	}
	providers = append(providers, wallet.EnvPassword("HASH_DIARY_PASSWORD"))
	providers = append(providers, wallet.InteractivePassword(os.Stdin, os.Stderr))
	return wallet.ChainedProvider(providers...)
}

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
			f := newFormatter()
			path := expandPath(flagKeypair)

			f.VerboseLog("keypair path: %s", path)

			// Check if file already exists.
			if _, err := os.Stat(path); err == nil && !flagForce {
				if f.Format == "json" {
					return fmt.Errorf("密钥文件已存在: %s，使用 --force 覆盖", path)
				}
				f.Prompt("密钥文件已存在，是否覆盖？(y/N) ")
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(answer)
				if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
					f.VerboseLog("user declined overwrite, aborting")
					return fmt.Errorf("已取消")
				}
			}

			// Handle password prompt.
			var password string
			if !flagNoPassword {
				if flagPassword != "" {
					password = flagPassword
				} else {
					pw := os.Getenv("HASH_DIARY_PASSWORD")
					if pw != "" {
						password = pw
					} else {
						f.Prompt("请输入钱包密码（可选，直接回车跳过）: ")
						reader := bufio.NewReader(os.Stdin)
						line, _ := reader.ReadString('\n')
						password = strings.TrimSpace(line)
					}
				}
			}

			// Generate keypair.
			kp, err := wallet.GenerateKeypair()
			if err != nil {
				return fmt.Errorf("生成密钥对失败: %w", err)
			}

			f.VerboseLog("generated keypair with address: %s", kp.Address())

			// Ensure parent directory exists.
			if err := wallet.EnsureDir(path); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}

			// Save keypair.
			if password != "" {
				f.VerboseLog("saving encrypted keypair")
				if err := wallet.SaveEncrypted(kp, path, password); err != nil {
					return fmt.Errorf("保存加密密钥对失败: %w", err)
				}
			} else {
				f.VerboseLog("saving plaintext keypair")
				if err := wallet.SavePlaintext(kp, path); err != nil {
					return fmt.Errorf("保存密钥对失败: %w", err)
				}
			}

			// Output result.
			if f.Format == "json" {
				f.Success(map[string]string{
					"address": kp.Address(),
					"keypair": path,
				})
			} else {
				f.Success(fmt.Sprintf("钱包已创建，地址: %s\n密钥文件已保存至: %s", kp.Address(), path))
			}

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
			f := newFormatter()
			path := expandPath(flagKeypair)

			f.VerboseLog("loading wallet from: %s", path)

			// Load wallet with password provider chain.
			provider := buildPasswordProvider()
			kp, err := wallet.LoadWallet(path, provider)
			if err != nil {
				return fmt.Errorf("加载钱包失败: %w", err)
			}

			address := kp.Address()
			f.VerboseLog("wallet address: %s", address)

			// Create RPC client and get balance.
			client := rpc.NewClient(flagURL)
			ctx := cmd.Context()

			balance, err := client.GetBalance(ctx, address)
			if err != nil {
				return fmt.Errorf("获取余额失败: %w", err)
			}

			network := rpc.DetectNetwork(flagURL)
			balanceStr := fmt.Sprintf("%.9f", balance)

			f.PrintWalletInfo(address, balanceStr, network)

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
			f := newFormatter()
			path := expandPath(flagKeypair)

			f.VerboseLog("loading wallet from: %s", path)

			// Load wallet to get address.
			provider := buildPasswordProvider()
			kp, err := wallet.LoadWallet(path, provider)
			if err != nil {
				return fmt.Errorf("加载钱包失败: %w", err)
			}

			address := kp.Address()
			f.VerboseLog("wallet address: %s", address)

			// Create RPC client.
			client := rpc.NewClient(flagURL)
			ctx := cmd.Context()

			// Check network — block mainnet.
			network := rpc.DetectNetwork(flagURL)
			if network == rpc.NetworkMainnetBeta {
				return fmt.Errorf("airdrop 仅支持 devnet/testnet")
			}

			f.VerboseLog("requesting airdrop of %.2f SOL on %s", flagAmount, network)

			// Request airdrop.
			_, err = client.RequestAirdrop(ctx, address, flagAmount)
			if err != nil {
				if errors.Is(err, rpc.ErrMainnetAirdrop) {
					return fmt.Errorf("airdrop 仅支持 devnet/testnet")
				}
				return fmt.Errorf("airdrop 请求失败: %w", err)
			}

			// Get new balance after airdrop.
			balance, err := client.GetBalance(ctx, address)
			if err != nil {
				return fmt.Errorf("获取余额失败: %w", err)
			}

			// Output result.
			if f.Format == "json" {
				f.Success(map[string]interface{}{
					"airdrop": flagAmount,
					"balance": balance,
				})
			} else {
				f.Success(fmt.Sprintf("已领取 %.2f SOL，当前余额: %.9f SOL", flagAmount, balance))
			}

			return nil
		},
	}

	cmd.Flags().Float64Var(&flagAmount, "amount", 1, "Amount of SOL to request")

	return cmd
}

func newWalletImportCmd() *cobra.Command {
	var (
		flagPrivateKey string
		flagMnemonic   string
		flagPassphrase string
		flagNoPassword bool
	)

	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import an existing wallet",
		Long: `Import an existing Solana wallet from a keypair file, Base58-encoded private key, or BIP-39 mnemonic.

Three import methods (mutually exclusive):
  1. Positional argument: path to a Solana CLI keypair JSON file
  2. --private-key: Base58-encoded 64-byte Ed25519 private key
  3. --mnemonic: BIP-39 mnemonic phrase (12 or 24 words), derives via m/44'/501'/0'/0'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			f := newFormatter()
			path := expandPath(flagKeypair)

			// Mutual exclusion: count how many input methods are specified.
			inputCount := 0
			if flagPrivateKey != "" {
				inputCount++
			}
			if flagMnemonic != "" {
				inputCount++
			}
			if len(args) > 0 {
				inputCount++
			}
			if inputCount > 1 {
				return fmt.Errorf("请只使用一种导入方式：文件路径、--private-key 或 --mnemonic")
			}
			if inputCount == 0 {
				return fmt.Errorf("请提供密钥文件路径，或使用 --private-key / --mnemonic 标志")
			}

			var kp *wallet.Keypair
			var err error

			// Parse input: --mnemonic, --private-key, or positional arg (file path).
			if flagMnemonic != "" {
				f.VerboseLog("importing from BIP-39 mnemonic")
				kp, err = wallet.ImportFromMnemonic(flagMnemonic, flagPassphrase)
				if err != nil {
					return fmt.Errorf("导入助记词失败: %w", err)
				}
			} else if flagPrivateKey != "" {
				f.VerboseLog("importing from base58 private key")
				kp, err = wallet.ImportFromBase58PrivateKey(flagPrivateKey)
				if err != nil {
					return fmt.Errorf("导入私钥失败: %w", err)
				}
			} else {
				srcPath := expandPath(args[0])
				f.VerboseLog("importing from file: %s", srcPath)
				kp, err = wallet.ImportFromFile(srcPath)
				if err != nil {
					return fmt.Errorf("导入密钥文件失败: %w", err)
				}
			}

			f.VerboseLog("imported keypair with address: %s", kp.Address())

			// Handle password prompt (same as wallet new).
			var password string
			if !flagNoPassword {
				if flagPassword != "" {
					password = flagPassword
				} else {
					pw := os.Getenv("HASH_DIARY_PASSWORD")
					if pw != "" {
						password = pw
					} else {
						f.Prompt("请输入钱包密码（可选，直接回车跳过）: ")
						reader := bufio.NewReader(os.Stdin)
						line, _ := reader.ReadString('\n')
						password = strings.TrimSpace(line)
					}
				}
			}

			// Ensure parent directory exists.
			if err := wallet.EnsureDir(path); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}

			// Save keypair.
			if password != "" {
				f.VerboseLog("saving encrypted keypair")
				if err := wallet.SaveEncrypted(kp, path, password); err != nil {
					return fmt.Errorf("保存加密密钥对失败: %w", err)
				}
			} else {
				f.VerboseLog("saving plaintext keypair")
				if err := wallet.SavePlaintext(kp, path); err != nil {
					return fmt.Errorf("保存密钥对失败: %w", err)
				}
			}

			// Output result.
			if f.Format == "json" {
				f.Success(map[string]string{
					"address": kp.Address(),
					"keypair": path,
				})
			} else {
				f.Success(fmt.Sprintf("钱包已创建，地址: %s\n密钥文件已保存至: %s", kp.Address(), path))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&flagPrivateKey, "private-key", "", "Base58-encoded private key to import")
	cmd.Flags().StringVar(&flagMnemonic, "mnemonic", "", "BIP-39 mnemonic phrase (12 or 24 words)")
	cmd.Flags().StringVar(&flagPassphrase, "passphrase", "", "Optional BIP-39 passphrase (not the wallet encryption password)")
	cmd.Flags().BoolVar(&flagNoPassword, "no-password", false, "Import keypair without password encryption")

	return cmd
}
