// Package rpc provides a Solana RPC client wrapper for HashDiary.
//
// It wraps the gagliardetto/solana-go SDK with retry logic, network detection,
// and convenience methods for the operations HashDiary needs.
package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	solanarpc "github.com/gagliardetto/solana-go/rpc"
)

const (
	// DefaultEndpoint is the default Solana RPC endpoint (devnet).
	DefaultEndpoint = "https://api.devnet.solana.com"

	// LamportsPerSOL is the number of lamports in one SOL.
	LamportsPerSOL = 1_000_000_000

	// DefaultConfirmTimeout is the default timeout for transaction confirmation.
	DefaultConfirmTimeout = 60 * time.Second

	// confirmPollInterval is the interval between confirmation polls.
	confirmPollInterval = 2 * time.Second

	// maxSendRetries is the maximum number of retries for sending a transaction.
	maxSendRetries = 3

	// StatusConfirmed indicates a transaction has been confirmed.
	StatusConfirmed = "confirmed"

	// StatusTimeout indicates a transaction confirmation timed out.
	StatusTimeout = "timeout"

	// NetworkDevnet is the devnet network name.
	NetworkDevnet = "devnet"
	// NetworkTestnet is the testnet network name.
	NetworkTestnet = "testnet"
	// NetworkMainnetBeta is the mainnet-beta network name.
	NetworkMainnetBeta = "mainnet-beta"
	// NetworkCustom is a custom/unknown network name.
	NetworkCustom = "custom"
)

// sendBackoffDurations defines the backoff durations for send retries.
var sendBackoffDurations = []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}

// ErrMainnetAirdrop is returned when an airdrop is attempted on mainnet.
var ErrMainnetAirdrop = errors.New("airdrop is not available on mainnet-beta; use devnet or testnet")

// ErrConfirmTimeout is returned when transaction confirmation times out.
// The caller can still use the signature to check the transaction later.
var ErrConfirmTimeout = errors.New("transaction confirmation timed out")

// SignatureOpts holds options for fetching transaction signatures.
type SignatureOpts struct {
	Limit  int
	Before string
	Until  string
}

// SignatureInfo holds information about a transaction signature.
type SignatureInfo struct {
	Signature string
	BlockTime *int64
	Err       interface{}
}

// TransactionResult holds full transaction data including log messages.
type TransactionResult struct {
	Slot        uint64
	BlockTime   *int64
	LogMessages []string
	Err         interface{}
}

// ExtractMemoData searches the log messages for Memo program data.
// Memo program logs have the format: "Program log: Memo (len N): <data>"
// or simply the data logged via the memo program.
func (tr *TransactionResult) ExtractMemoData() []string {
	if tr == nil {
		return nil
	}
	var memos []string
	for _, log := range tr.LogMessages {
		// The Memo program logs data in the format:
		// "Program log: Memo (len <N>): <data>"
		if strings.Contains(log, "Memo (len") {
			// Extract the data after the last ": "
			idx := strings.LastIndex(log, "): ")
			if idx >= 0 {
				memos = append(memos, log[idx+3:])
			}
		}
	}
	return memos
}

// sleepFunc is a function that can be overridden in tests.
var sleepFunc = time.Sleep

// RPCClientInterface defines the interface for the underlying Solana RPC client.
// This allows for mocking in tests.
type RPCClientInterface interface {
	GetBalance(ctx context.Context, publicKey solana.PublicKey, commitment solanarpc.CommitmentType) (*solanarpc.GetBalanceResult, error)
	RequestAirdrop(ctx context.Context, account solana.PublicKey, lamports uint64, commitment solanarpc.CommitmentType) (solana.Signature, error)
	SendTransactionWithOpts(ctx context.Context, transaction *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error)
	GetSignatureStatuses(ctx context.Context, searchTransactionHistory bool, transactionSignatures ...solana.Signature) (*solanarpc.GetSignatureStatusesResult, error)
	GetSignaturesForAddressWithOpts(ctx context.Context, account solana.PublicKey, opts *solanarpc.GetSignaturesForAddressOpts) ([]*solanarpc.TransactionSignature, error)
	GetTransaction(ctx context.Context, txSig solana.Signature, opts *solanarpc.GetTransactionOpts) (*solanarpc.GetTransactionResult, error)
}

// Client is a Solana RPC client wrapper with retry logic and convenience methods.
type Client struct {
	endpoint string
	network  string
	rpc      RPCClientInterface
}

// NewClient creates a new RPC client. If endpoint is empty, the default devnet
// endpoint is used.
func NewClient(endpoint string) *Client {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	rpcClient := solanarpc.New(endpoint)
	return &Client{
		endpoint: endpoint,
		network:  DetectNetwork(endpoint),
		rpc:      rpcClient,
	}
}

// newClientWithRPC creates a Client with a custom RPCClientInterface (for testing).
func newClientWithRPC(endpoint string, rpcClient RPCClientInterface) *Client {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return &Client{
		endpoint: endpoint,
		network:  DetectNetwork(endpoint),
		rpc:      rpcClient,
	}
}

// Endpoint returns the RPC endpoint URL.
func (c *Client) Endpoint() string {
	return c.endpoint
}

// Network returns the detected network name.
func (c *Client) Network() string {
	return c.network
}

// GetBalance queries the SOL balance for an address. Returns the balance in SOL (not lamports).
func (c *Client) GetBalance(ctx context.Context, address string) (float64, error) {
	pubKey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return 0, fmt.Errorf("invalid address %q: %w", address, err)
	}

	result, err := c.rpc.GetBalance(ctx, pubKey, solanarpc.CommitmentConfirmed)
	if err != nil {
		return 0, wrapNetworkError(err)
	}

	return float64(result.Value) / float64(LamportsPerSOL), nil
}

// RequestAirdrop requests an airdrop on devnet/testnet. Returns the transaction signature.
// Returns an error if the endpoint is mainnet-beta.
func (c *Client) RequestAirdrop(ctx context.Context, address string, amountSOL float64) (string, error) {
	if c.network == NetworkMainnetBeta {
		return "", ErrMainnetAirdrop
	}

	pubKey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return "", fmt.Errorf("invalid address %q: %w", address, err)
	}

	lamports := uint64(amountSOL * float64(LamportsPerSOL))
	sig, err := c.rpc.RequestAirdrop(ctx, pubKey, lamports, solanarpc.CommitmentConfirmed)
	if err != nil {
		return "", wrapNetworkError(err)
	}

	return sig.String(), nil
}

// SendTransaction sends a signed transaction with retry logic.
// On failure, retries up to 3 times with backoff of 500ms, 1s, 2s.
// Returns the transaction signature string.
func (c *Client) SendTransaction(ctx context.Context, tx *solana.Transaction) (string, error) {
	if tx == nil {
		return "", fmt.Errorf("transaction is nil")
	}

	opts := solanarpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: solanarpc.CommitmentConfirmed,
	}

	var lastErr error
	for attempt := 0; attempt <= maxSendRetries; attempt++ {
		if attempt > 0 {
			backoff := sendBackoffDurations[attempt-1]
			sleepFunc(backoff)
		}

		sig, err := c.rpc.SendTransactionWithOpts(ctx, tx, opts)
		if err == nil {
			return sig.String(), nil
		}

		lastErr = err

		// Check for context cancellation -- do not retry.
		if ctx.Err() != nil {
			return "", fmt.Errorf("transaction send cancelled: %w", ctx.Err())
		}
	}

	return "", wrapNetworkError(fmt.Errorf("failed to send transaction after %d attempts: %w", maxSendRetries+1, lastErr))
}

// ConfirmTransaction waits for a transaction to reach confirmed status.
// If timeout is 0, DefaultConfirmTimeout (60s) is used.
// On timeout, returns the signature along with ErrConfirmTimeout so the caller
// can still use the signature.
func (c *Client) ConfirmTransaction(ctx context.Context, signature string, timeout time.Duration) (string, error) {
	if timeout == 0 {
		timeout = DefaultConfirmTimeout
	}

	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return signature, fmt.Errorf("invalid signature %q: %w", signature, err)
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(confirmPollInterval)
	defer ticker.Stop()

	for {
		result, err := c.rpc.GetSignatureStatuses(ctx, false, sig)
		if err == nil && result != nil && len(result.Value) > 0 && result.Value[0] != nil {
			status := result.Value[0]
			if status.ConfirmationStatus == solanarpc.ConfirmationStatusConfirmed ||
				status.ConfirmationStatus == solanarpc.ConfirmationStatusFinalized {
				return signature, nil
			}
		}

		if time.Now().After(deadline) {
			return signature, ErrConfirmTimeout
		}

		select {
		case <-ctx.Done():
			return signature, fmt.Errorf("confirmation cancelled: %w", ctx.Err())
		case <-ticker.C:
			// continue polling
		}
	}
}

// SendAndConfirmTransaction combines send and confirm. Returns:
//   - (sig, "confirmed", nil) on success
//   - (sig, "timeout", nil) on confirmation timeout
//   - ("", "", error) on send failure
func (c *Client) SendAndConfirmTransaction(ctx context.Context, tx *solana.Transaction) (string, string, error) {
	sig, err := c.SendTransaction(ctx, tx)
	if err != nil {
		return "", "", err
	}

	_, confirmErr := c.ConfirmTransaction(ctx, sig, DefaultConfirmTimeout)
	if confirmErr != nil {
		if errors.Is(confirmErr, ErrConfirmTimeout) {
			return sig, StatusTimeout, nil
		}
		return sig, "", confirmErr
	}

	return sig, StatusConfirmed, nil
}

// GetSignaturesForAddress fetches transaction signatures for an address.
func (c *Client) GetSignaturesForAddress(ctx context.Context, address string, opts SignatureOpts) ([]SignatureInfo, error) {
	pubKey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", address, err)
	}

	rpcOpts := &solanarpc.GetSignaturesForAddressOpts{
		Commitment: solanarpc.CommitmentConfirmed,
	}

	if opts.Limit > 0 {
		rpcOpts.Limit = &opts.Limit
	}

	if opts.Before != "" {
		beforeSig, err := solana.SignatureFromBase58(opts.Before)
		if err != nil {
			return nil, fmt.Errorf("invalid 'before' signature %q: %w", opts.Before, err)
		}
		rpcOpts.Before = beforeSig
	}

	if opts.Until != "" {
		untilSig, err := solana.SignatureFromBase58(opts.Until)
		if err != nil {
			return nil, fmt.Errorf("invalid 'until' signature %q: %w", opts.Until, err)
		}
		rpcOpts.Until = untilSig
	}

	result, err := c.rpc.GetSignaturesForAddressWithOpts(ctx, pubKey, rpcOpts)
	if err != nil {
		return nil, wrapNetworkError(err)
	}

	infos := make([]SignatureInfo, 0, len(result))
	for _, txSig := range result {
		info := SignatureInfo{
			Signature: txSig.Signature.String(),
			Err:       txSig.Err,
		}
		if txSig.BlockTime != nil {
			bt := int64(*txSig.BlockTime)
			info.BlockTime = &bt
		}
		infos = append(infos, info)
	}

	return infos, nil
}

// GetTransaction gets full transaction data including log messages.
func (c *Client) GetTransaction(ctx context.Context, signature string) (*TransactionResult, error) {
	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature %q: %w", signature, err)
	}

	maxVersion := uint64(0)
	result, err := c.rpc.GetTransaction(ctx, sig, &solanarpc.GetTransactionOpts{
		Commitment:                     solanarpc.CommitmentConfirmed,
		MaxSupportedTransactionVersion: &maxVersion,
	})
	if err != nil {
		return nil, wrapNetworkError(err)
	}

	txResult := &TransactionResult{
		Slot: result.Slot,
	}

	if result.BlockTime != nil {
		bt := int64(*result.BlockTime)
		txResult.BlockTime = &bt
	}

	if result.Meta != nil {
		txResult.LogMessages = result.Meta.LogMessages
		txResult.Err = result.Meta.Err
	}

	return txResult, nil
}

// DetectNetwork detects the network name from the endpoint URL.
// Returns "devnet", "testnet", "mainnet-beta", or "custom".
func DetectNetwork(endpoint string) string {
	lower := strings.ToLower(endpoint)
	switch {
	case strings.Contains(lower, "devnet"):
		return NetworkDevnet
	case strings.Contains(lower, "testnet"):
		return NetworkTestnet
	case strings.Contains(lower, "mainnet"):
		return NetworkMainnetBeta
	default:
		return NetworkCustom
	}
}

// wrapNetworkError wraps network-related errors with user-friendly messages.
func wrapNetworkError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Check for common network error patterns.
	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "no such host") ||
		strings.Contains(errMsg, "dial tcp") ||
		strings.Contains(errMsg, "i/o timeout") {
		return fmt.Errorf("网络错误: %w。请检查网络连接或尝试其他 RPC 端点（--url）", err)
	}

	if strings.Contains(errMsg, "429") || strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "Too Many Requests") {
		return fmt.Errorf("RPC 请求频率受限: %w。请稍后重试或切换 RPC 端点", err)
	}

	// Check for insufficient balance errors.
	if strings.Contains(errMsg, "insufficient funds") ||
		strings.Contains(errMsg, "Insufficient") ||
		strings.Contains(errMsg, "0x1") { // InsufficientFunds error code
		return fmt.Errorf("余额不足: %w。请通过 `hash-diary wallet airdrop`（devnet/testnet）充值", err)
	}

	return err
}
