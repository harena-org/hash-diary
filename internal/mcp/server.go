// Package mcp provides MCP (Model Context Protocol) server for HashDiary.
//
// The server exposes diary_write and diary_read tools, allowing AI agents
// to read and write encrypted diary entries on the Solana blockchain via
// JSON-RPC over stdio.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	solanarpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/hash-diary/hash-diary/internal/cache"
	"github.com/hash-diary/hash-diary/internal/crypto"
	"github.com/hash-diary/hash-diary/internal/rpc"
	"github.com/hash-diary/hash-diary/internal/wallet"
	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Business error codes for MCP error responses.
// These codes follow the JSON-RPC standard error code range for server errors
// (-32000 to -32099). The mcp-go SDK handles JSON-RPC framing automatically,
// so these codes are embedded in tool-level error messages for structured
// error reporting.
const (
	ErrCodeWalletLocked        = -32001 // Wallet locked / cannot decrypt
	ErrCodeContentTooLarge     = -32002 // Content exceeds 512-byte memo limit
	ErrCodeInsufficientBalance = -32003 // Insufficient SOL balance
	ErrCodeNetworkError        = -32004 // Network/RPC error
)

// toolError creates a CallToolResult with a structured error message
// containing the business error code prefix.
func toolError(code int, msg string) *gomcp.CallToolResult {
	return gomcp.NewToolResultError(fmt.Sprintf("[%d] %s", code, msg))
}

// Server is the MCP server for HashDiary.
type Server struct {
	keypair  *wallet.Keypair
	endpoint string
}

// NewServer creates a new MCP server instance.
func NewServer(kp *wallet.Keypair, endpoint string) *Server {
	return &Server{
		keypair:  kp,
		endpoint: endpoint,
	}
}

// Run starts the MCP server on stdio. This blocks until the server is shut down.
// Stdin/stdout are used for JSON-RPC communication; all logging goes to stderr.
func (s *Server) Run() error {
	mcpSrv := mcpserver.NewMCPServer("hash-diary", "0.1.0")

	// Register diary_write tool.
	writeTool := gomcp.NewTool("diary_write",
		gomcp.WithDescription("Write a diary entry to Solana blockchain"),
		gomcp.WithString("text", gomcp.Required(), gomcp.Description("Diary text content")),
	)
	mcpSrv.AddTool(writeTool, s.handleDiaryWrite)

	// Register diary_read tool.
	readTool := gomcp.NewTool("diary_read",
		gomcp.WithDescription("Read diary entries from Solana blockchain"),
		gomcp.WithNumber("limit", gomcp.Description("Number of entries to return (default 7)")),
		gomcp.WithString("since", gomcp.Description("Start date filter (YYYY-MM-DD)")),
		gomcp.WithString("search", gomcp.Description("Keyword filter for entry content")),
		gomcp.WithString("before", gomcp.Description("Cursor pagination: transaction signature to paginate before")),
	)
	mcpSrv.AddTool(readTool, s.handleDiaryRead)

	return mcpserver.ServeStdio(mcpSrv)
}

// handleDiaryWrite handles the diary_write tool call.
// It encodes the text as an encrypted memo, builds a Solana transaction, sends it,
// and waits for confirmation.
func (s *Server) handleDiaryWrite(ctx context.Context, request gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	text, err := request.RequireString("text")
	if err != nil {
		return gomcp.NewToolResultError("missing required parameter 'text'"), nil
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return gomcp.NewToolResultError("text content cannot be empty"), nil
	}

	// Encode memo (compress + encrypt + base64 + prefix).
	encodedMemo, err := crypto.EncodeMemo(text, s.keypair.PublicKey, s.keypair.PrivateKey)
	if err != nil {
		if strings.Contains(err.Error(), "memo") && strings.Contains(err.Error(), "超过上限") {
			return toolError(ErrCodeContentTooLarge, fmt.Sprintf("failed to encode memo: %s", err)), nil
		}
		return gomcp.NewToolResultError(fmt.Sprintf("failed to encode memo: %s", err)), nil
	}

	// Build transaction.
	sdkClient := solanarpc.New(s.endpoint)
	recent, err := sdkClient.GetLatestBlockhash(ctx, solanarpc.CommitmentFinalized)
	if err != nil {
		return toolError(ErrCodeNetworkError, fmt.Sprintf("failed to get latest blockhash: %s", err)), nil
	}

	memoProgramID := solana.MustPublicKeyFromBase58("Memo4c2pN8afCj432Lb7RMVKi9PbQnnW7ewFFaV3oAH")
	walletPubKey := solana.PublicKeyFromBytes(s.keypair.PublicKey)

	instruction := solana.NewInstruction(
		memoProgramID,
		solana.AccountMetaSlice{
			solana.NewAccountMeta(walletPubKey, false, true),
		},
		[]byte(encodedMemo),
	)

	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		recent.Value.Blockhash,
		solana.TransactionPayer(walletPubKey),
	)
	if err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("failed to build transaction: %s", err)), nil
	}

	// Sign transaction.
	privKey := solana.PrivateKey(s.keypair.PrivateKey)
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(walletPubKey) {
			return &privKey
		}
		return nil
	})
	if err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("failed to sign transaction: %s", err)), nil
	}

	// Send and confirm transaction.
	rpcClient := rpc.NewClient(s.endpoint)
	sig, status, err := rpcClient.SendAndConfirmTransaction(ctx, tx)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "insufficient") || strings.Contains(errMsg, "Insufficient") {
			return toolError(ErrCodeInsufficientBalance, fmt.Sprintf("failed to send transaction: %s", err)), nil
		}
		return toolError(ErrCodeNetworkError, fmt.Sprintf("failed to send transaction: %s", err)), nil
	}

	resultText := fmt.Sprintf("Diary entry written successfully.\nSignature: %s\nStatus: %s", sig, status)
	return gomcp.NewToolResultText(resultText), nil
}

// handleDiaryRead handles the diary_read tool call.
// It syncs entries from the blockchain, filters them, and returns a JSON array.
func (s *Server) handleDiaryRead(ctx context.Context, request gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	// Parse optional parameters with defaults.
	limit := request.GetInt("limit", 7)
	since := request.GetString("since", "")
	search := request.GetString("search", "")
	before := request.GetString("before", "")

	if limit <= 0 {
		limit = 7
	}

	address := s.keypair.Address()
	rpcClient := rpc.NewClient(s.endpoint)
	store := cache.NewStore("")

	// Sync and filter entries under file lock.
	var allEntries []cache.Entry
	err := store.WithLock(func() error {
		data, loadErr := store.Load()
		if loadErr != nil {
			return fmt.Errorf("failed to load cache: %w", loadErr)
		}

		// Sync new entries from chain.
		syncErr := s.syncEntries(ctx, rpcClient, s.keypair, data, address)
		if syncErr != nil {
			return syncErr
		}

		// Save updated cache.
		if saveErr := store.Save(data); saveErr != nil {
			return fmt.Errorf("failed to save cache: %w", saveErr)
		}

		wc := cache.GetWalletCache(data, address)
		allEntries = make([]cache.Entry, len(wc.Entries))
		copy(allEntries, wc.Entries)
		return nil
	})
	if err != nil {
		return toolError(ErrCodeNetworkError, fmt.Sprintf("failed to sync entries: %s", err)), nil
	}

	// Sort by timestamp descending (newest first).
	sort.Slice(allEntries, func(i, j int) bool {
		if allEntries[i].Ts != allEntries[j].Ts {
			return allEntries[i].Ts > allEntries[j].Ts
		}
		return allEntries[i].Sig > allEntries[j].Sig
	})

	// Apply --since filter.
	if since != "" {
		sinceTime, err := time.ParseInLocation("2006-01-02", since, time.Now().Location())
		if err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid date format for 'since': %s", err)), nil
		}
		sinceUnix := sinceTime.Unix()
		filtered := allEntries[:0]
		for _, e := range allEntries {
			if e.Ts >= sinceUnix {
				filtered = append(filtered, e)
			}
		}
		allEntries = filtered
	}

	// Apply --search filter.
	if search != "" {
		searchLower := strings.ToLower(search)
		filtered := allEntries[:0]
		for _, e := range allEntries {
			if strings.Contains(strings.ToLower(e.Content), searchLower) {
				filtered = append(filtered, e)
			}
		}
		allEntries = filtered
	}

	// Apply --before cursor pagination.
	if before != "" {
		idx := -1
		for i, e := range allEntries {
			if e.Sig == before {
				idx = i
				break
			}
		}
		if idx >= 0 && idx+1 < len(allEntries) {
			allEntries = allEntries[idx+1:]
		} else {
			allEntries = nil
		}
	}

	// Apply limit.
	if limit > 0 && len(allEntries) > limit {
		allEntries = allEntries[:limit]
	}

	// Build result as JSON array.
	type entryResult struct {
		Timestamp string `json:"timestamp"`
		Content   string `json:"content"`
		Signature string `json:"signature"`
	}

	results := make([]entryResult, len(allEntries))
	for i, e := range allEntries {
		results[i] = entryResult{
			Timestamp: time.Unix(e.Ts, 0).UTC().Format(time.RFC3339),
			Content:   e.Content,
			Signature: e.Sig,
		}
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %s", err)), nil
	}

	return gomcp.NewToolResultText(string(jsonBytes)), nil
}

// syncEntries fetches new transactions from the chain and appends decoded
// entries to the cache. This replicates the logic from cmd/read.go.
func (s *Server) syncEntries(ctx context.Context, rpcClient *rpc.Client, kp *wallet.Keypair, data *cache.CacheData, address string) error {
	lastSig := cache.GetLastSignature(data, address)

	// Fetch all new signatures since lastSig.
	var allNewSigs []rpc.SignatureInfo
	fetchBefore := ""
	for {
		opts := rpc.SignatureOpts{Limit: 1000}
		if lastSig != "" {
			opts.Until = lastSig
		}
		if fetchBefore != "" {
			opts.Before = fetchBefore
		}

		sigs, err := rpcClient.GetSignaturesForAddress(ctx, address, opts)
		if err != nil {
			return fmt.Errorf("failed to get signatures: %w", err)
		}

		if len(sigs) == 0 {
			break
		}

		allNewSigs = append(allNewSigs, sigs...)
		fetchBefore = sigs[len(sigs)-1].Signature

		if len(sigs) < 1000 {
			break
		}
	}

	if len(allNewSigs) == 0 {
		return nil
	}

	newestSig := allNewSigs[0].Signature

	// Process in reverse order (oldest first).
	for i, j := 0, len(allNewSigs)-1; i < j; i, j = i+1, j-1 {
		allNewSigs[i], allNewSigs[j] = allNewSigs[j], allNewSigs[i]
	}

	var newEntries []cache.Entry
	for _, sigInfo := range allNewSigs {
		if sigInfo.BlockTime == nil {
			continue
		}
		if sigInfo.Err != nil {
			continue
		}

		txResult, err := rpcClient.GetTransaction(ctx, sigInfo.Signature)
		if err != nil {
			continue
		}

		memos := txResult.ExtractMemoData()
		for _, memoData := range memos {
			if !strings.HasPrefix(memoData, crypto.MemoPrefix) {
				continue
			}

			content, err := crypto.DecodeMemo(memoData, kp.PublicKey, kp.PrivateKey)
			if err != nil {
				continue
			}

			newEntries = append(newEntries, cache.Entry{
				Sig:     sigInfo.Signature,
				Ts:      *sigInfo.BlockTime,
				Content: content,
			})
		}
	}

	// Update cache regardless of whether diary entries were found,
	// to avoid re-scanning these transactions next time.
	cache.AppendEntries(data, address, newEntries, newestSig)

	return nil
}
