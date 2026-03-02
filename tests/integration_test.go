//go:build !integration

package tests

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hash-diary/hash-diary/internal/cache"
	"github.com/hash-diary/hash-diary/internal/crypto"
	"github.com/hash-diary/hash-diary/internal/wallet"
	"github.com/mr-tron/base58"
)

// ---------------------------------------------------------------------------
// Test 1: Wallet Lifecycle
// ---------------------------------------------------------------------------

func TestWalletLifecycle_Plaintext(t *testing.T) {
	// Generate -> SavePlaintext -> LoadPlaintext -> verify address matches.
	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "plaintext-wallet.json")

	if err := wallet.SavePlaintext(kp, path); err != nil {
		t.Fatalf("SavePlaintext() error: %v", err)
	}

	loaded, err := wallet.LoadPlaintext(path)
	if err != nil {
		t.Fatalf("LoadPlaintext() error: %v", err)
	}

	if kp.Address() != loaded.Address() {
		t.Errorf("address mismatch: generated=%q, loaded=%q", kp.Address(), loaded.Address())
	}
}

func TestWalletLifecycle_Encrypted(t *testing.T) {
	// Generate -> SaveEncrypted -> LoadEncrypted -> verify address matches.
	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted-wallet.json")
	password := "integration-test-pw-!@#"

	if err := wallet.SaveEncrypted(kp, path, password); err != nil {
		t.Fatalf("SaveEncrypted() error: %v", err)
	}

	loaded, err := wallet.LoadEncrypted(path, password)
	if err != nil {
		t.Fatalf("LoadEncrypted() error: %v", err)
	}

	if kp.Address() != loaded.Address() {
		t.Errorf("address mismatch: generated=%q, loaded=%q", kp.Address(), loaded.Address())
	}
}

func TestWalletLifecycle_EncryptedWrongPassword(t *testing.T) {
	// SaveEncrypted -> LoadEncrypted(wrongPw) -> verify error.
	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted-wallet.json")

	if err := wallet.SaveEncrypted(kp, path, "correct-password"); err != nil {
		t.Fatalf("SaveEncrypted() error: %v", err)
	}

	_, err = wallet.LoadEncrypted(path, "wrong-password")
	if err == nil {
		t.Fatal("LoadEncrypted() with wrong password should fail")
	}
	if !strings.Contains(err.Error(), "密码错误") {
		t.Errorf("error should mention password error, got: %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Test 2: Encrypted Wallet Full Flow (generate, save, load, use for crypto)
// ---------------------------------------------------------------------------

func TestEncryptedWalletFullFlow(t *testing.T) {
	// Generate keypair.
	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error: %v", err)
	}

	// Save encrypted.
	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted-wallet.json")
	password := "full-flow-password"

	if err := wallet.SaveEncrypted(kp, path, password); err != nil {
		t.Fatalf("SaveEncrypted() error: %v", err)
	}

	// Load encrypted.
	loaded, err := wallet.LoadEncrypted(path, password)
	if err != nil {
		t.Fatalf("LoadEncrypted() error: %v", err)
	}

	// Use the loaded keypair for crypto operations (self-encrypt diary entry).
	originalText := "Today I tested the encrypted wallet flow."

	memo, err := crypto.EncodeMemo(originalText, loaded.PublicKey, loaded.PrivateKey)
	if err != nil {
		t.Fatalf("EncodeMemo() error: %v", err)
	}

	if !strings.HasPrefix(memo, crypto.MemoPrefix) {
		t.Errorf("memo should have prefix %q, got prefix: %q", crypto.MemoPrefix, memo[:3])
	}

	decoded, err := crypto.DecodeMemo(memo, loaded.PublicKey, loaded.PrivateKey)
	if err != nil {
		t.Fatalf("DecodeMemo() error: %v", err)
	}

	if decoded != originalText {
		t.Errorf("round-trip mismatch: got %q, want %q", decoded, originalText)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Crypto -> Cache Round-Trip
// ---------------------------------------------------------------------------

func TestCryptoCacheRoundTrip(t *testing.T) {
	// Generate keypair.
	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error: %v", err)
	}

	// EncodeMemo -> DecodeMemo round-trip for various content.
	texts := []string{
		"Hello, World!",
		"Today was a great day for testing.",
		"Multi\nline\ncontent\nworks too.",
	}

	for _, original := range texts {
		memo, err := crypto.EncodeMemo(original, kp.PublicKey, kp.PrivateKey)
		if err != nil {
			t.Fatalf("EncodeMemo(%q) error: %v", original, err)
		}

		decoded, err := crypto.DecodeMemo(memo, kp.PublicKey, kp.PrivateKey)
		if err != nil {
			t.Fatalf("DecodeMemo() error: %v", err)
		}

		if decoded != original {
			t.Errorf("crypto round-trip mismatch: got %q, want %q", decoded, original)
		}
	}

	// Create cache store in temp dir, save entries, load and verify.
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")
	store := cache.NewStore(cachePath)

	address := kp.Address()

	entries := []cache.Entry{
		{Sig: "sig1", Ts: 1700000000, Content: "Diary entry 1"},
		{Sig: "sig2", Ts: 1700001000, Content: "Diary entry 2"},
		{Sig: "sig3", Ts: 1700002000, Content: "Diary entry 3"},
	}

	// Save to cache.
	data, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}

	cache.AppendEntries(data, address, entries, "sig3")

	if err := store.Save(data); err != nil {
		t.Fatalf("store.Save() error: %v", err)
	}

	// Load from cache and verify.
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() after save error: %v", err)
	}

	wc := cache.GetWalletCache(loaded, address)
	if len(wc.Entries) != 3 {
		t.Fatalf("expected 3 cache entries, got %d", len(wc.Entries))
	}

	for i, entry := range wc.Entries {
		if entry.Sig != entries[i].Sig {
			t.Errorf("entry[%d] sig mismatch: got %q, want %q", i, entry.Sig, entries[i].Sig)
		}
		if entry.Ts != entries[i].Ts {
			t.Errorf("entry[%d] ts mismatch: got %d, want %d", i, entry.Ts, entries[i].Ts)
		}
		if entry.Content != entries[i].Content {
			t.Errorf("entry[%d] content mismatch: got %q, want %q", i, entry.Content, entries[i].Content)
		}
	}

	if wc.LastSignature != "sig3" {
		t.Errorf("LastSignature mismatch: got %q, want %q", wc.LastSignature, "sig3")
	}
}

// ---------------------------------------------------------------------------
// Test 4: Read Filtering Logic
//
// Replicates the filtering pipeline from cmd/read.go's filterAndDisplay:
//   1. Sort by timestamp descending (sig desc as tiebreaker)
//   2. --since filter
//   3. --search filter
//   4. --before cursor pagination
//   5. --limit
// ---------------------------------------------------------------------------

// filterEntries replicates the read command's filtering pipeline for testing.
func filterEntries(entries []cache.Entry, since string, search string, before string, limit int) []cache.Entry {
	// Make a copy to avoid mutating the original.
	result := make([]cache.Entry, len(entries))
	copy(result, entries)

	// Sort by timestamp descending; signature descending as tiebreaker.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Ts != result[j].Ts {
			return result[i].Ts > result[j].Ts
		}
		return result[i].Sig > result[j].Sig
	})

	// --since filter.
	if since != "" {
		sinceTime, err := time.ParseInLocation("2006-01-02", since, time.Now().Location())
		if err == nil {
			sinceUnix := sinceTime.Unix()
			filtered := make([]cache.Entry, 0)
			for _, e := range result {
				if e.Ts >= sinceUnix {
					filtered = append(filtered, e)
				}
			}
			result = filtered
		}
	}

	// --search filter.
	if search != "" {
		searchLower := strings.ToLower(search)
		filtered := make([]cache.Entry, 0)
		for _, e := range result {
			if strings.Contains(strings.ToLower(e.Content), searchLower) {
				filtered = append(filtered, e)
			}
		}
		result = filtered
	}

	// --before cursor pagination.
	if before != "" {
		idx := -1
		for i, e := range result {
			if e.Sig == before {
				idx = i
				break
			}
		}
		if idx >= 0 && idx+1 < len(result) {
			result = result[idx+1:]
		} else {
			result = nil
		}
	}

	// --limit.
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

func TestReadFilteringLogic_SortOrder(t *testing.T) {
	entries := []cache.Entry{
		{Sig: "sigA", Ts: 1000, Content: "oldest"},
		{Sig: "sigC", Ts: 3000, Content: "newest"},
		{Sig: "sigB", Ts: 2000, Content: "middle"},
	}

	result := filterEntries(entries, "", "", "", 0)

	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	if result[0].Sig != "sigC" {
		t.Errorf("first entry should be sigC (newest), got %q", result[0].Sig)
	}
	if result[1].Sig != "sigB" {
		t.Errorf("second entry should be sigB (middle), got %q", result[1].Sig)
	}
	if result[2].Sig != "sigA" {
		t.Errorf("third entry should be sigA (oldest), got %q", result[2].Sig)
	}
}

func TestReadFilteringLogic_SortOrder_SameTimestamp(t *testing.T) {
	// When timestamps are equal, sort by signature descending.
	entries := []cache.Entry{
		{Sig: "aaa", Ts: 1000, Content: "entry a"},
		{Sig: "ccc", Ts: 1000, Content: "entry c"},
		{Sig: "bbb", Ts: 1000, Content: "entry b"},
	}

	result := filterEntries(entries, "", "", "", 0)

	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	// Descending sig order: ccc > bbb > aaa
	if result[0].Sig != "ccc" {
		t.Errorf("first entry should be ccc, got %q", result[0].Sig)
	}
	if result[1].Sig != "bbb" {
		t.Errorf("second entry should be bbb, got %q", result[1].Sig)
	}
	if result[2].Sig != "aaa" {
		t.Errorf("third entry should be aaa, got %q", result[2].Sig)
	}
}

func TestReadFilteringLogic_SinceFilter(t *testing.T) {
	// Use fixed timestamps. 2024-01-01 00:00:00 UTC = 1704067200.
	entries := []cache.Entry{
		{Sig: "s1", Ts: 1703980800, Content: "Dec 31 2023"},     // 2023-12-31
		{Sig: "s2", Ts: 1704067200, Content: "Jan 1 2024"},      // 2024-01-01
		{Sig: "s3", Ts: 1704153600, Content: "Jan 2 2024"},      // 2024-01-02
		{Sig: "s4", Ts: 1704240000, Content: "Jan 3 2024"},      // 2024-01-03
	}

	// The --since filter uses local timezone parsing. We need to pick a date
	// that reliably filters. Use a date in the middle.
	// We filter "since 2024-01-02" and expect entries on or after Jan 2.
	// Since time parsing is local-timezone-dependent, we compute the threshold
	// the same way the production code does.
	sinceDate := "2024-01-02"
	sinceTime, _ := time.ParseInLocation("2006-01-02", sinceDate, time.Now().Location())
	sinceUnix := sinceTime.Unix()

	result := filterEntries(entries, sinceDate, "", "", 0)

	for _, e := range result {
		if e.Ts < sinceUnix {
			t.Errorf("entry %q (ts=%d) should have been filtered out by --since %s (threshold=%d)",
				e.Sig, e.Ts, sinceDate, sinceUnix)
		}
	}

	// At minimum, entries at or after the threshold should be present.
	if len(result) == 0 {
		t.Error("expected at least one entry after --since filter")
	}
}

func TestReadFilteringLogic_SearchFilter(t *testing.T) {
	entries := []cache.Entry{
		{Sig: "s1", Ts: 3000, Content: "Today I went to the park"},
		{Sig: "s2", Ts: 2000, Content: "Read a BOOK about history"},
		{Sig: "s3", Ts: 1000, Content: "Nothing special happened"},
	}

	// Case-insensitive search for "book".
	result := filterEntries(entries, "", "book", "", 0)

	if len(result) != 1 {
		t.Fatalf("expected 1 entry matching 'book', got %d", len(result))
	}
	if result[0].Sig != "s2" {
		t.Errorf("expected entry s2, got %q", result[0].Sig)
	}
}

func TestReadFilteringLogic_SearchFilter_NoMatch(t *testing.T) {
	entries := []cache.Entry{
		{Sig: "s1", Ts: 3000, Content: "Today I went to the park"},
		{Sig: "s2", Ts: 2000, Content: "Read a book about history"},
	}

	result := filterEntries(entries, "", "xyz-nonexistent", "", 0)

	if len(result) != 0 {
		t.Errorf("expected 0 entries for non-matching search, got %d", len(result))
	}
}

func TestReadFilteringLogic_SearchFilter_CaseInsensitive(t *testing.T) {
	entries := []cache.Entry{
		{Sig: "s1", Ts: 2000, Content: "UPPERCASE content"},
		{Sig: "s2", Ts: 1000, Content: "lowercase content"},
	}

	result := filterEntries(entries, "", "CONTENT", "", 0)

	if len(result) != 2 {
		t.Errorf("expected 2 entries for case-insensitive search, got %d", len(result))
	}
}

func TestReadFilteringLogic_Limit(t *testing.T) {
	entries := []cache.Entry{
		{Sig: "s1", Ts: 5000, Content: "e1"},
		{Sig: "s2", Ts: 4000, Content: "e2"},
		{Sig: "s3", Ts: 3000, Content: "e3"},
		{Sig: "s4", Ts: 2000, Content: "e4"},
		{Sig: "s5", Ts: 1000, Content: "e5"},
	}

	result := filterEntries(entries, "", "", "", 3)

	if len(result) != 3 {
		t.Fatalf("expected 3 entries with limit=3, got %d", len(result))
	}
	// Should be the 3 newest (sorted desc).
	if result[0].Sig != "s1" {
		t.Errorf("first entry should be s1 (newest), got %q", result[0].Sig)
	}
	if result[2].Sig != "s3" {
		t.Errorf("third entry should be s3, got %q", result[2].Sig)
	}
}

func TestReadFilteringLogic_Limit_LargerThanEntries(t *testing.T) {
	entries := []cache.Entry{
		{Sig: "s1", Ts: 2000, Content: "e1"},
		{Sig: "s2", Ts: 1000, Content: "e2"},
	}

	result := filterEntries(entries, "", "", "", 10)

	if len(result) != 2 {
		t.Errorf("limit larger than entries should return all, got %d", len(result))
	}
}

func TestReadFilteringLogic_BeforeCursor(t *testing.T) {
	entries := []cache.Entry{
		{Sig: "s1", Ts: 5000, Content: "newest"},
		{Sig: "s2", Ts: 4000, Content: "second"},
		{Sig: "s3", Ts: 3000, Content: "third"},
		{Sig: "s4", Ts: 2000, Content: "fourth"},
		{Sig: "s5", Ts: 1000, Content: "oldest"},
	}

	// After sorting desc: s1, s2, s3, s4, s5.
	// --before s3 should return entries after s3 in the sorted list: s4, s5.
	result := filterEntries(entries, "", "", "s3", 0)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries after cursor s3, got %d", len(result))
	}
	if result[0].Sig != "s4" {
		t.Errorf("first entry after cursor should be s4, got %q", result[0].Sig)
	}
	if result[1].Sig != "s5" {
		t.Errorf("second entry after cursor should be s5, got %q", result[1].Sig)
	}
}

func TestReadFilteringLogic_BeforeCursor_NotFound(t *testing.T) {
	entries := []cache.Entry{
		{Sig: "s1", Ts: 2000, Content: "e1"},
		{Sig: "s2", Ts: 1000, Content: "e2"},
	}

	result := filterEntries(entries, "", "", "nonexistent", 0)

	if len(result) != 0 {
		t.Errorf("expected 0 entries for non-existent cursor, got %d", len(result))
	}
}

func TestReadFilteringLogic_BeforeCursor_LastEntry(t *testing.T) {
	entries := []cache.Entry{
		{Sig: "s1", Ts: 2000, Content: "e1"},
		{Sig: "s2", Ts: 1000, Content: "e2"},
	}

	// Cursor points to the last entry in desc order (s2), so no entries after it.
	result := filterEntries(entries, "", "", "s2", 0)

	if len(result) != 0 {
		t.Errorf("expected 0 entries after last cursor, got %d", len(result))
	}
}

func TestReadFilteringLogic_CombinedFilters(t *testing.T) {
	// Combine --search + --limit to test filter chaining.
	entries := []cache.Entry{
		{Sig: "s1", Ts: 5000, Content: "park visit"},
		{Sig: "s2", Ts: 4000, Content: "park reading"},
		{Sig: "s3", Ts: 3000, Content: "movie night"},
		{Sig: "s4", Ts: 2000, Content: "park walk"},
		{Sig: "s5", Ts: 1000, Content: "park run"},
	}

	// Search for "park" (4 results), then limit to 2.
	result := filterEntries(entries, "", "park", "", 2)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries with search+limit, got %d", len(result))
	}
	// Should be the 2 newest "park" entries.
	if result[0].Sig != "s1" {
		t.Errorf("first should be s1, got %q", result[0].Sig)
	}
	if result[1].Sig != "s2" {
		t.Errorf("second should be s2, got %q", result[1].Sig)
	}
}

func TestReadFilteringLogic_BeforeCursorWithLimit(t *testing.T) {
	entries := []cache.Entry{
		{Sig: "s1", Ts: 5000, Content: "e1"},
		{Sig: "s2", Ts: 4000, Content: "e2"},
		{Sig: "s3", Ts: 3000, Content: "e3"},
		{Sig: "s4", Ts: 2000, Content: "e4"},
		{Sig: "s5", Ts: 1000, Content: "e5"},
	}

	// --before s2 gives s3, s4, s5. --limit 2 gives s3, s4.
	result := filterEntries(entries, "", "", "s2", 2)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries with cursor+limit, got %d", len(result))
	}
	if result[0].Sig != "s3" {
		t.Errorf("first should be s3, got %q", result[0].Sig)
	}
	if result[1].Sig != "s4" {
		t.Errorf("second should be s4, got %q", result[1].Sig)
	}
}

func TestReadFilteringLogic_EmptyEntries(t *testing.T) {
	var entries []cache.Entry
	result := filterEntries(entries, "", "", "", 7)

	if len(result) != 0 {
		t.Errorf("expected 0 entries for empty input, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// Test 5: Force-Refresh (cache clear and re-populate)
// ---------------------------------------------------------------------------

func TestForceRefresh(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")
	store := cache.NewStore(cachePath)

	address := "TestWalletAddress123"

	// Populate cache with some entries.
	data, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}

	initialEntries := []cache.Entry{
		{Sig: "old1", Ts: 1000, Content: "old entry 1"},
		{Sig: "old2", Ts: 2000, Content: "old entry 2"},
	}
	cache.AppendEntries(data, address, initialEntries, "old2")

	if err := store.Save(data); err != nil {
		t.Fatalf("store.Save() error: %v", err)
	}

	// Verify entries exist.
	data, err = store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	wc := cache.GetWalletCache(data, address)
	if len(wc.Entries) != 2 {
		t.Fatalf("expected 2 entries before clear, got %d", len(wc.Entries))
	}

	// Simulate --force-refresh: clear the wallet cache.
	cache.ClearWalletCache(data, address)

	// Verify entries are gone.
	wc = cache.GetWalletCache(data, address)
	if len(wc.Entries) != 0 {
		t.Errorf("expected 0 entries after clear, got %d", len(wc.Entries))
	}
	if wc.LastSignature != "" {
		t.Errorf("expected empty LastSignature after clear, got %q", wc.LastSignature)
	}

	// Re-populate with new entries (simulating a fresh sync).
	newEntries := []cache.Entry{
		{Sig: "new1", Ts: 3000, Content: "fresh entry 1"},
		{Sig: "new2", Ts: 4000, Content: "fresh entry 2"},
		{Sig: "new3", Ts: 5000, Content: "fresh entry 3"},
	}
	cache.AppendEntries(data, address, newEntries, "new3")

	if err := store.Save(data); err != nil {
		t.Fatalf("store.Save() after re-populate error: %v", err)
	}

	// Verify new entries persisted.
	data, err = store.Load()
	if err != nil {
		t.Fatalf("store.Load() after re-populate error: %v", err)
	}
	wc = cache.GetWalletCache(data, address)
	if len(wc.Entries) != 3 {
		t.Fatalf("expected 3 entries after re-populate, got %d", len(wc.Entries))
	}
	if wc.LastSignature != "new3" {
		t.Errorf("expected LastSignature=new3, got %q", wc.LastSignature)
	}
	if wc.Entries[0].Content != "fresh entry 1" {
		t.Errorf("first re-populated entry content mismatch: got %q", wc.Entries[0].Content)
	}
}

func TestForceRefresh_PreservesOtherWallets(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")
	store := cache.NewStore(cachePath)

	// Populate two wallets.
	data, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}

	cache.AppendEntries(data, "wallet-A", []cache.Entry{
		{Sig: "a1", Ts: 1000, Content: "wallet A entry"},
	}, "a1")
	cache.AppendEntries(data, "wallet-B", []cache.Entry{
		{Sig: "b1", Ts: 2000, Content: "wallet B entry"},
	}, "b1")

	if err := store.Save(data); err != nil {
		t.Fatalf("store.Save() error: %v", err)
	}

	// Force-refresh only wallet-A.
	data, err = store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	cache.ClearWalletCache(data, "wallet-A")

	if err := store.Save(data); err != nil {
		t.Fatalf("store.Save() error: %v", err)
	}

	// Verify wallet-A is cleared but wallet-B is preserved.
	data, err = store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}

	wcA := cache.GetWalletCache(data, "wallet-A")
	if len(wcA.Entries) != 0 {
		t.Errorf("wallet-A should be cleared, got %d entries", len(wcA.Entries))
	}

	wcB := cache.GetWalletCache(data, "wallet-B")
	if len(wcB.Entries) != 1 {
		t.Errorf("wallet-B should be preserved, got %d entries", len(wcB.Entries))
	}
	if wcB.Entries[0].Content != "wallet B entry" {
		t.Errorf("wallet-B entry content mismatch: got %q", wcB.Entries[0].Content)
	}
}

// ---------------------------------------------------------------------------
// Test 6: Import from Base58 Private Key
// ---------------------------------------------------------------------------

func TestImportFromBase58(t *testing.T) {
	// Generate keypair.
	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error: %v", err)
	}

	// Encode private key to Base58.
	b58PrivKey := base58.Encode(kp.PrivateKey)

	// Import from Base58.
	imported, err := wallet.ImportFromBase58PrivateKey(b58PrivKey)
	if err != nil {
		t.Fatalf("ImportFromBase58PrivateKey() error: %v", err)
	}

	// Verify same address.
	if kp.Address() != imported.Address() {
		t.Errorf("address mismatch after import: original=%q, imported=%q", kp.Address(), imported.Address())
	}

	// Verify the imported keypair can be used for crypto operations.
	original := "Diary entry with imported key"
	memo, err := crypto.EncodeMemo(original, imported.PublicKey, imported.PrivateKey)
	if err != nil {
		t.Fatalf("EncodeMemo() with imported key error: %v", err)
	}

	decoded, err := crypto.DecodeMemo(memo, imported.PublicKey, imported.PrivateKey)
	if err != nil {
		t.Fatalf("DecodeMemo() with imported key error: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip with imported key mismatch: got %q, want %q", decoded, original)
	}
}

func TestImportFromBase58_ThenSaveAndLoad(t *testing.T) {
	// Generate, export to Base58, import, then save/load as both formats.
	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error: %v", err)
	}

	b58PrivKey := base58.Encode(kp.PrivateKey)
	imported, err := wallet.ImportFromBase58PrivateKey(b58PrivKey)
	if err != nil {
		t.Fatalf("ImportFromBase58PrivateKey() error: %v", err)
	}

	dir := t.TempDir()

	// Save as plaintext, then load.
	plainPath := filepath.Join(dir, "plain.json")
	if err := wallet.SavePlaintext(imported, plainPath); err != nil {
		t.Fatalf("SavePlaintext() error: %v", err)
	}
	loadedPlain, err := wallet.LoadPlaintext(plainPath)
	if err != nil {
		t.Fatalf("LoadPlaintext() error: %v", err)
	}
	if loadedPlain.Address() != kp.Address() {
		t.Errorf("plaintext round-trip address mismatch: got %q, want %q", loadedPlain.Address(), kp.Address())
	}

	// Save as encrypted, then load.
	encPath := filepath.Join(dir, "encrypted.json")
	password := "import-test-pw"
	if err := wallet.SaveEncrypted(imported, encPath, password); err != nil {
		t.Fatalf("SaveEncrypted() error: %v", err)
	}
	loadedEnc, err := wallet.LoadEncrypted(encPath, password)
	if err != nil {
		t.Fatalf("LoadEncrypted() error: %v", err)
	}
	if loadedEnc.Address() != kp.Address() {
		t.Errorf("encrypted round-trip address mismatch: got %q, want %q", loadedEnc.Address(), kp.Address())
	}
}

// ---------------------------------------------------------------------------
// Test: Full pipeline (wallet + crypto + cache)
// ---------------------------------------------------------------------------

func TestFullPipeline_WalletCryptoCache(t *testing.T) {
	// This test exercises the complete local pipeline:
	//   1. Generate a wallet
	//   2. Save it encrypted
	//   3. Load it back
	//   4. Use it to encode several diary entries
	//   5. Store them in cache
	//   6. Load cache and apply read filters
	//   7. Verify the content and ordering

	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error: %v", err)
	}

	dir := t.TempDir()

	// Save and load encrypted wallet.
	walletPath := filepath.Join(dir, "wallet.json")
	password := "pipeline-test"
	if err := wallet.SaveEncrypted(kp, walletPath, password); err != nil {
		t.Fatalf("SaveEncrypted() error: %v", err)
	}
	loaded, err := wallet.LoadEncrypted(walletPath, password)
	if err != nil {
		t.Fatalf("LoadEncrypted() error: %v", err)
	}

	// Encode several diary entries.
	diaryTexts := []string{
		"Day 1: Started the project.",
		"Day 2: Implemented encryption.",
		"Day 3: Added cache support.",
		"Day 4: Wrote integration tests.",
		"Day 5: Final review and polish.",
	}

	var entries []cache.Entry
	baseTs := int64(1700000000)
	for i, text := range diaryTexts {
		memo, err := crypto.EncodeMemo(text, loaded.PublicKey, loaded.PrivateKey)
		if err != nil {
			t.Fatalf("EncodeMemo(%q) error: %v", text, err)
		}

		// Verify round-trip.
		decoded, err := crypto.DecodeMemo(memo, loaded.PublicKey, loaded.PrivateKey)
		if err != nil {
			t.Fatalf("DecodeMemo() error: %v", err)
		}
		if decoded != text {
			t.Errorf("round-trip mismatch for entry %d: got %q, want %q", i, decoded, text)
		}

		entries = append(entries, cache.Entry{
			Sig:     "sig" + string(rune('A'+i)),
			Ts:      baseTs + int64(i*86400), // one day apart
			Content: text,
		})
	}

	// Store in cache.
	cachePath := filepath.Join(dir, "cache.json")
	store := cache.NewStore(cachePath)
	data, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	cache.AppendEntries(data, loaded.Address(), entries, entries[len(entries)-1].Sig)
	if err := store.Save(data); err != nil {
		t.Fatalf("store.Save() error: %v", err)
	}

	// Load and filter.
	data, err = store.Load()
	if err != nil {
		t.Fatalf("store.Load() after save error: %v", err)
	}
	wc := cache.GetWalletCache(data, loaded.Address())

	// Apply search filter for "cache".
	filtered := filterEntries(wc.Entries, "", "cache", "", 0)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry matching 'cache', got %d", len(filtered))
	}
	if filtered[0].Content != "Day 3: Added cache support." {
		t.Errorf("filtered content mismatch: got %q", filtered[0].Content)
	}

	// Apply limit.
	limited := filterEntries(wc.Entries, "", "", "", 2)
	if len(limited) != 2 {
		t.Fatalf("expected 2 entries with limit=2, got %d", len(limited))
	}
	// Should be newest first.
	if limited[0].Content != "Day 5: Final review and polish." {
		t.Errorf("first limited entry should be Day 5, got %q", limited[0].Content)
	}
}

// ---------------------------------------------------------------------------
// Test: Cache file locking during force-refresh workflow
// ---------------------------------------------------------------------------

func TestForceRefresh_WithFileLock(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")
	store := cache.NewStore(cachePath)

	address := "LockTestWallet"

	// Simulate the full force-refresh workflow under file lock, as the real
	// read command does it.
	err := store.WithLock(func() error {
		// Load cache.
		data, err := store.Load()
		if err != nil {
			return err
		}

		// Populate initial data.
		cache.AppendEntries(data, address, []cache.Entry{
			{Sig: "init1", Ts: 1000, Content: "initial"},
		}, "init1")

		return store.Save(data)
	})
	if err != nil {
		t.Fatalf("initial populate under lock error: %v", err)
	}

	// Now force-refresh under lock.
	err = store.WithLock(func() error {
		data, err := store.Load()
		if err != nil {
			return err
		}

		// Clear (force-refresh).
		cache.ClearWalletCache(data, address)

		// Re-populate.
		cache.AppendEntries(data, address, []cache.Entry{
			{Sig: "fresh1", Ts: 2000, Content: "refreshed"},
		}, "fresh1")

		return store.Save(data)
	})
	if err != nil {
		t.Fatalf("force-refresh under lock error: %v", err)
	}

	// Verify final state.
	data, err := store.Load()
	if err != nil {
		t.Fatalf("final load error: %v", err)
	}
	wc := cache.GetWalletCache(data, address)
	if len(wc.Entries) != 1 {
		t.Fatalf("expected 1 entry after force-refresh, got %d", len(wc.Entries))
	}
	if wc.Entries[0].Content != "refreshed" {
		t.Errorf("expected 'refreshed', got %q", wc.Entries[0].Content)
	}
	if wc.LastSignature != "fresh1" {
		t.Errorf("expected LastSignature=fresh1, got %q", wc.LastSignature)
	}
}
