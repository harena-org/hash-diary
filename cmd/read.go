package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hash-diary/hash-diary/internal/cache"
	"github.com/hash-diary/hash-diary/internal/crypto"
	"github.com/hash-diary/hash-diary/internal/output"
	"github.com/hash-diary/hash-diary/internal/rpc"
	"github.com/hash-diary/hash-diary/internal/wallet"
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
			f := newFormatter()
			ctx := cmd.Context()

			// 1. Load wallet (for address and decryption keys).
			path := expandPath(flagKeypair)
			f.VerboseLog("loading wallet from: %s", path)

			provider := buildPasswordProvider()
			kp, err := wallet.LoadWallet(path, provider)
			if err != nil {
				return fmt.Errorf("加载钱包失败: %w", err)
			}

			address := kp.Address()
			f.VerboseLog("wallet address: %s", address)

			// 2. Create RPC client, cache store, formatter.
			rpcClient := rpc.NewClient(flagURL)
			store := cache.NewStore("")

			// 3-6. Load cache, sync, save — all under file lock.
			var allEntries []cache.Entry
			err = store.WithLock(func() error {
				// 3. Load cache.
				data, loadErr := store.Load()
				if loadErr != nil {
					return fmt.Errorf("加载缓存失败: %w", loadErr)
				}

				// 4. If --force-refresh: clear wallet cache.
				if flagForceRefresh {
					f.VerboseLog("force-refresh: clearing cache for %s", address)
					cache.ClearWalletCache(data, address)
				}

				// 5. Sync new entries from chain.
				syncErr := syncEntries(ctx, f, rpcClient, kp, data, address)
				if syncErr != nil {
					return syncErr
				}

				// 6. Save updated cache.
				if saveErr := store.Save(data); saveErr != nil {
					return fmt.Errorf("保存缓存失败: %w", saveErr)
				}

				// Grab entries for filtering.
				wc := cache.GetWalletCache(data, address)
				allEntries = make([]cache.Entry, len(wc.Entries))
				copy(allEntries, wc.Entries)
				return nil
			})
			if err != nil {
				return err
			}

			// 7. Filter and display entries.
			return filterAndDisplay(f, allEntries, flagSince, flagSearch, flagBefore, flagLimit)
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 7, "Maximum number of entries to display")
	cmd.Flags().StringVar(&flagSince, "since", "", "Show entries since date (e.g. 2026-01-01)")
	cmd.Flags().StringVar(&flagSearch, "search", "", "Search entries by keyword")
	cmd.Flags().StringVar(&flagBefore, "before", "", "Cursor pagination: show entries before this transaction signature")
	cmd.Flags().BoolVar(&flagForceRefresh, "force-refresh", false, "Bypass cache and fetch from blockchain")

	return cmd
}

// syncEntries fetches new transactions from the chain and appends decoded entries to the cache.
func syncEntries(ctx context.Context, f *output.Formatter, rpcClient *rpc.Client, kp *wallet.Keypair, data *cache.CacheData, address string) error {
	lastSig := cache.GetLastSignature(data, address)
	f.VerboseLog("last cached signature: %s", lastSig)

	// Fetch all new signatures since lastSig.
	var allNewSigs []rpc.SignatureInfo
	fetchBefore := "" // cursor for paginating through RPC results
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
			return fmt.Errorf("获取交易签名失败: %w", err)
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
		f.VerboseLog("no new signatures found")
		return nil
	}

	f.VerboseLog("found %d new signature(s)", len(allNewSigs))

	// The newest signature in the original order (first element, since RPC returns newest-first).
	newestSig := allNewSigs[0].Signature

	// Process in reverse order (oldest first) to maintain chronological cache.
	reverse(allNewSigs)

	var newEntries []cache.Entry
	for _, sigInfo := range allNewSigs {
		if sigInfo.BlockTime == nil {
			continue // skip entries without blockTime
		}
		if sigInfo.Err != nil {
			continue // skip failed txs
		}

		txResult, err := rpcClient.GetTransaction(ctx, sigInfo.Signature)
		if err != nil {
			f.VerboseLog("获取交易失败: %s", err)
			continue
		}

		memos := txResult.ExtractMemoData()
		for _, memoData := range memos {
			if !strings.HasPrefix(memoData, crypto.MemoPrefix) {
				continue
			}

			content, err := crypto.DecodeMemo(memoData, kp.PublicKey, kp.PrivateKey)
			if err != nil {
				f.VerboseLog("解码失败: %s", err)
				continue
			}

			newEntries = append(newEntries, cache.Entry{
				Sig:     sigInfo.Signature,
				Ts:      *sigInfo.BlockTime,
				Content: content,
			})
		}
	}

	f.VerboseLog("decoded %d new entry(ies)", len(newEntries))

	if len(newEntries) > 0 {
		cache.AppendEntries(data, address, newEntries, newestSig)
	} else {
		// Even if no diary entries were found, update the last signature
		// so we don't re-scan these transactions next time.
		cache.AppendEntries(data, address, nil, newestSig)
	}

	return nil
}

// filterAndDisplay applies filters to cached entries and displays them.
func filterAndDisplay(f *output.Formatter, entries []cache.Entry, since, search, before string, limit int) error {
	// Sort by timestamp descending (newest first).
	// If timestamps are equal, sort by signature descending (lexicographic).
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Ts != entries[j].Ts {
			return entries[i].Ts > entries[j].Ts
		}
		return entries[i].Sig > entries[j].Sig
	})

	// Apply --since filter: parse date as local timezone.
	if since != "" {
		sinceTime, err := time.ParseInLocation("2006-01-02", since, time.Now().Location())
		if err != nil {
			return fmt.Errorf("无效的日期格式 (--since): %w", err)
		}
		sinceUnix := sinceTime.Unix()
		filtered := entries[:0]
		for _, e := range entries {
			if e.Ts >= sinceUnix {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Apply --search filter: case-insensitive substring match on content.
	if search != "" {
		searchLower := strings.ToLower(search)
		filtered := entries[:0]
		for _, e := range entries {
			if strings.Contains(strings.ToLower(e.Content), searchLower) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Apply --before cursor: find the entry with matching signature,
	// return only entries after it (older entries in our desc-sorted list).
	if before != "" {
		idx := -1
		for i, e := range entries {
			if e.Sig == before {
				idx = i
				break
			}
		}
		if idx >= 0 && idx+1 < len(entries) {
			entries = entries[idx+1:]
		} else {
			// Cursor not found or no entries after it.
			entries = nil
		}
	}

	// Apply --limit: take first N entries.
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	// Handle no entries case.
	if len(entries) == 0 {
		if f.Format == "json" {
			f.PrintDiaryEntries([]output.DiaryEntry{})
		} else {
			fmt.Fprintln(f.Writer, "暂无日记记录")
		}
		return nil
	}

	// Convert to output format.
	diaryEntries := make([]output.DiaryEntry, len(entries))
	for i, e := range entries {
		diaryEntries[i] = output.DiaryEntry{
			Timestamp: time.Unix(e.Ts, 0).Local().Format("2006-01-02 15:04:05 MST"),
			Content:   e.Content,
			Signature: e.Sig,
		}
	}

	f.PrintDiaryEntries(diaryEntries)

	// Pagination hint: if returned count == limit, show hint.
	if limit > 0 && len(entries) == limit && f.Format != "json" {
		lastSig := entries[len(entries)-1].Sig
		fmt.Fprintf(f.Writer, "-- 更多记录: hash-diary read --before %s\n", lastSig)
	}

	return nil
}

// reverse reverses a slice of SignatureInfo in place.
func reverse(sigs []rpc.SignatureInfo) {
	for i, j := 0, len(sigs)-1; i < j; i, j = i+1, j-1 {
		sigs[i], sigs[j] = sigs[j], sigs[i]
	}
}
