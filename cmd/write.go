package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gagliardetto/solana-go"
	solanarpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/hash-diary/hash-diary/internal/crypto"
	"github.com/hash-diary/hash-diary/internal/rpc"
	"github.com/hash-diary/hash-diary/internal/wallet"
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
			f := newFormatter()

			// --- Input Processing ---
			var text string
			if len(args) > 0 {
				text = args[0]
			} else {
				// Check if stdin is a pipe/redirected (not a terminal).
				stdinInfo, err := os.Stdin.Stat()
				if err == nil && (stdinInfo.Mode()&os.ModeCharDevice) == 0 {
					data, err := io.ReadAll(os.Stdin)
					if err != nil {
						return fmt.Errorf("读取 stdin 失败: %w", err)
					}
					text = string(data)
				}
			}

			text = strings.TrimSpace(text)
			if text == "" {
				return fmt.Errorf("请提供日记内容（通过命令行参数或 stdin）")
			}

			// --- Load Wallet ---
			expandedPath := expandPath(flagKeypair)
			provider := buildPasswordProvider()
			kp, err := wallet.LoadWallet(expandedPath, provider)
			if err != nil {
				return fmt.Errorf("加载钱包失败: %w", err)
			}

			f.VerboseLog("钱包地址: %s", kp.Address())

			// --- Encode Memo ---
			encodedMemo, err := crypto.EncodeMemo(text, kp.PublicKey, kp.PrivateKey)
			if err != nil {
				return fmt.Errorf("编码日记内容失败: %w", err)
			}

			f.VerboseLog("编码后 memo 长度: %d 字节", len(encodedMemo))

			// --- Build Transaction ---
			ctx := context.Background()

			// Get recent blockhash using solana-go SDK directly.
			sdkClient := solanarpc.New(flagURL)
			recent, err := sdkClient.GetLatestBlockhash(ctx, solanarpc.CommitmentFinalized)
			if err != nil {
				return fmt.Errorf("获取最新区块哈希失败: %w", err)
			}

			// Build memo instruction manually (raw data, no length prefix).
			memoProgramID := solana.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")
			walletPubKey := solana.PublicKeyFromBytes(kp.PublicKey)

			instruction := solana.NewInstruction(
				memoProgramID,
				solana.AccountMetaSlice{
					solana.NewAccountMeta(walletPubKey, false, true),
				},
				[]byte(encodedMemo),
			)

			// Create transaction.
			tx, err := solana.NewTransaction(
				[]solana.Instruction{instruction},
				recent.Value.Blockhash,
				solana.TransactionPayer(walletPubKey),
			)
			if err != nil {
				return fmt.Errorf("构建交易失败: %w", err)
			}

			// Sign transaction.
			privKey := solana.PrivateKey(kp.PrivateKey)
			_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
				if key.Equals(walletPubKey) {
					return &privKey
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("签名交易失败: %w", err)
			}

			// --- Send Transaction ---
			client := rpc.NewClient(flagURL)

			if flagNoWait {
				sig, err := client.SendTransaction(ctx, tx)
				if err != nil {
					return fmt.Errorf("发送交易失败: %w", err)
				}

				if f.Format == "json" {
					f.Success(map[string]string{
						"signature": sig,
					})
				} else {
					fmt.Fprintf(f.Writer, "交易已发送，签名: %s\n", sig)
				}
				return nil
			}

			// Wait for confirmation.
			if f.Format == "text" {
				fmt.Fprint(f.Writer, "交易已发送，等待确认...\n")
			}

			sig, status, err := client.SendAndConfirmTransaction(ctx, tx)
			if err != nil {
				return fmt.Errorf("发送交易失败: %w", err)
			}

			if f.Format == "json" {
				result := map[string]string{
					"signature": sig,
				}
				if status == rpc.StatusTimeout {
					result["status"] = "timeout"
				}
				f.Success(result)
			} else {
				if status == rpc.StatusConfirmed {
					fmt.Fprintf(f.Writer, "交易确认成功 (Confirmed)，签名: %s\n", sig)
				} else if status == rpc.StatusTimeout {
					fmt.Fprintf(f.Writer, "交易确认超时，签名: %s\n", sig)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&flagNoWait, "no-wait", false, "Don't wait for transaction confirmation")

	return cmd
}
