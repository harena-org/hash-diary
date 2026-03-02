package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// newTempStore creates a Store backed by a temporary directory.
// The caller should defer os.RemoveAll on the returned dir.
func newTempStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	return NewStore(path), dir
}

// ---------- Load tests ----------

func TestLoad_FileNotExist(t *testing.T) {
	store, _ := newTempStore(t)

	data, err := store.Load()
	if err != nil {
		t.Fatalf("Load() returned error for non-existent file: %v", err)
	}
	if data.CacheVersion != currentCacheVersion {
		t.Errorf("CacheVersion = %d, want %d", data.CacheVersion, currentCacheVersion)
	}
	if data.Wallets == nil {
		t.Error("Wallets map is nil, want initialised map")
	}
	if len(data.Wallets) != 0 {
		t.Errorf("Wallets has %d entries, want 0", len(data.Wallets))
	}
}

func TestLoad_ValidCache(t *testing.T) {
	store, _ := newTempStore(t)

	// Prepare a valid cache file on disk.
	original := &CacheData{
		CacheVersion: currentCacheVersion,
		Wallets: map[string]*WalletCache{
			"wallet1": {
				LastSignature: "sig123",
				Entries: []Entry{
					{Sig: "s1", Ts: 1000, Content: "hello"},
				},
			},
		},
	}
	writeJSON(t, store.Path(), original)

	data, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if data.CacheVersion != currentCacheVersion {
		t.Errorf("CacheVersion = %d, want %d", data.CacheVersion, currentCacheVersion)
	}
	wc, ok := data.Wallets["wallet1"]
	if !ok {
		t.Fatal("wallet1 not found in loaded cache")
	}
	if wc.LastSignature != "sig123" {
		t.Errorf("LastSignature = %q, want %q", wc.LastSignature, "sig123")
	}
	if len(wc.Entries) != 1 {
		t.Fatalf("Entries length = %d, want 1", len(wc.Entries))
	}
	if wc.Entries[0].Content != "hello" {
		t.Errorf("Entry content = %q, want %q", wc.Entries[0].Content, "hello")
	}
}

func TestLoad_VersionMismatch_CreatesBackup(t *testing.T) {
	store, _ := newTempStore(t)

	// Write a cache file with a different version.
	bad := &CacheData{
		CacheVersion: 999,
		Wallets: map[string]*WalletCache{
			"wallet1": {LastSignature: "old"},
		},
	}
	writeJSON(t, store.Path(), bad)

	data, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// Should get empty cache.
	if len(data.Wallets) != 0 {
		t.Errorf("Wallets has %d entries after version mismatch, want 0", len(data.Wallets))
	}
	if data.CacheVersion != currentCacheVersion {
		t.Errorf("CacheVersion = %d, want %d", data.CacheVersion, currentCacheVersion)
	}
	// Backup should exist.
	bakPath := store.Path() + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		t.Error("backup file was not created on version mismatch")
	}
}

func TestLoad_CorruptJSON_CreatesBackup(t *testing.T) {
	store, _ := newTempStore(t)

	// Write garbage to the cache file.
	if err := os.WriteFile(store.Path(), []byte("not json{{{"), 0600); err != nil {
		t.Fatalf("writing corrupt file: %v", err)
	}

	data, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(data.Wallets) != 0 {
		t.Errorf("Wallets has %d entries after corrupt JSON, want 0", len(data.Wallets))
	}
	bakPath := store.Path() + ".bak"
	bakData, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("reading backup file: %v", err)
	}
	if string(bakData) != "not json{{{" {
		t.Errorf("backup content = %q, want original corrupt content", string(bakData))
	}
}

func TestLoad_EmptyFile_CreatesBackup(t *testing.T) {
	store, _ := newTempStore(t)

	// Write an empty file (invalid JSON).
	if err := os.WriteFile(store.Path(), []byte(""), 0600); err != nil {
		t.Fatalf("writing empty file: %v", err)
	}

	data, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(data.Wallets) != 0 {
		t.Errorf("Wallets should be empty for empty file, got %d", len(data.Wallets))
	}
}

// ---------- Save tests ----------

func TestSave_CreatesDirectoryAndFile(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")
	path := filepath.Join(nested, "cache.json")
	store := NewStore(path)

	data := emptyCacheData()
	data.Wallets["w1"] = &WalletCache{LastSignature: "abc", Entries: []Entry{
		{Sig: "s1", Ts: 42, Content: "test"},
	}}

	if err := store.Save(data); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Directory should exist.
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("Stat directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}

	// File should exist and be parseable.
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after Save(): %v", err)
	}
	if loaded.Wallets["w1"].LastSignature != "abc" {
		t.Errorf("round-trip: LastSignature = %q, want %q", loaded.Wallets["w1"].LastSignature, "abc")
	}
}

func TestSave_FilePermissions(t *testing.T) {
	store, _ := newTempStore(t)

	if err := store.Save(emptyCacheData()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestSave_AtomicWrite_NoCorruptionOnPartialData(t *testing.T) {
	store, _ := newTempStore(t)

	// Save initial data.
	initial := emptyCacheData()
	initial.Wallets["w1"] = &WalletCache{LastSignature: "first"}
	if err := store.Save(initial); err != nil {
		t.Fatalf("Save(initial) error: %v", err)
	}

	// Save updated data (simulates the atomic overwrite).
	updated := emptyCacheData()
	updated.Wallets["w1"] = &WalletCache{LastSignature: "second"}
	if err := store.Save(updated); err != nil {
		t.Fatalf("Save(updated) error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Wallets["w1"].LastSignature != "second" {
		t.Errorf("LastSignature = %q, want %q", loaded.Wallets["w1"].LastSignature, "second")
	}
}

// ---------- GetWalletCache / GetLastSignature ----------

func TestGetWalletCache_Missing(t *testing.T) {
	data := emptyCacheData()
	wc := GetWalletCache(data, "nonexistent")
	if wc == nil {
		t.Fatal("GetWalletCache returned nil")
	}
	if wc.LastSignature != "" {
		t.Errorf("LastSignature = %q, want empty", wc.LastSignature)
	}
	if len(wc.Entries) != 0 {
		t.Errorf("Entries length = %d, want 0", len(wc.Entries))
	}
}

func TestGetWalletCache_Existing(t *testing.T) {
	data := emptyCacheData()
	data.Wallets["w1"] = &WalletCache{
		LastSignature: "sig",
		Entries:       []Entry{{Sig: "a", Ts: 1, Content: "x"}},
	}

	wc := GetWalletCache(data, "w1")
	if wc.LastSignature != "sig" {
		t.Errorf("LastSignature = %q, want %q", wc.LastSignature, "sig")
	}
	if len(wc.Entries) != 1 {
		t.Errorf("Entries length = %d, want 1", len(wc.Entries))
	}
}

func TestGetLastSignature_EmptyCache(t *testing.T) {
	data := emptyCacheData()
	sig := GetLastSignature(data, "w1")
	if sig != "" {
		t.Errorf("GetLastSignature = %q, want empty", sig)
	}
}

func TestGetLastSignature_NilWallets(t *testing.T) {
	data := &CacheData{CacheVersion: 1, Wallets: nil}
	sig := GetLastSignature(data, "w1")
	if sig != "" {
		t.Errorf("GetLastSignature with nil Wallets = %q, want empty", sig)
	}
}

// ---------- AppendEntries ----------

func TestAppendEntries_NewWallet(t *testing.T) {
	data := emptyCacheData()
	entries := []Entry{
		{Sig: "s1", Ts: 100, Content: "entry1"},
		{Sig: "s2", Ts: 200, Content: "entry2"},
	}

	AppendEntries(data, "w1", entries, "s2")

	wc := data.Wallets["w1"]
	if wc == nil {
		t.Fatal("wallet cache not created")
	}
	if wc.LastSignature != "s2" {
		t.Errorf("LastSignature = %q, want %q", wc.LastSignature, "s2")
	}
	if len(wc.Entries) != 2 {
		t.Fatalf("Entries length = %d, want 2", len(wc.Entries))
	}
	if wc.Entries[0].Content != "entry1" {
		t.Errorf("Entries[0].Content = %q, want %q", wc.Entries[0].Content, "entry1")
	}
}

func TestAppendEntries_ExistingWallet(t *testing.T) {
	data := emptyCacheData()
	data.Wallets["w1"] = &WalletCache{
		LastSignature: "old",
		Entries:       []Entry{{Sig: "s0", Ts: 50, Content: "existing"}},
	}

	newEntries := []Entry{{Sig: "s1", Ts: 100, Content: "new"}}
	AppendEntries(data, "w1", newEntries, "s1")

	wc := data.Wallets["w1"]
	if len(wc.Entries) != 2 {
		t.Fatalf("Entries length = %d, want 2", len(wc.Entries))
	}
	if wc.Entries[0].Content != "existing" {
		t.Errorf("first entry should be preserved")
	}
	if wc.Entries[1].Content != "new" {
		t.Errorf("new entry not appended")
	}
	if wc.LastSignature != "s1" {
		t.Errorf("LastSignature = %q, want %q", wc.LastSignature, "s1")
	}
}

func TestAppendEntries_NilWalletsMap(t *testing.T) {
	data := &CacheData{CacheVersion: 1, Wallets: nil}
	AppendEntries(data, "w1", []Entry{{Sig: "s1", Ts: 1, Content: "c"}}, "s1")

	if data.Wallets == nil {
		t.Fatal("Wallets map should have been initialised")
	}
	if len(data.Wallets["w1"].Entries) != 1 {
		t.Errorf("expected 1 entry")
	}
}

// ---------- ClearWalletCache ----------

func TestClearWalletCache_ExistingWallet(t *testing.T) {
	data := emptyCacheData()
	data.Wallets["w1"] = &WalletCache{LastSignature: "sig", Entries: []Entry{{Sig: "s1", Ts: 1, Content: "c"}}}
	data.Wallets["w2"] = &WalletCache{LastSignature: "sig2", Entries: []Entry{{Sig: "s2", Ts: 2, Content: "d"}}}

	ClearWalletCache(data, "w1")

	if _, ok := data.Wallets["w1"]; ok {
		t.Error("w1 should have been removed")
	}
	if _, ok := data.Wallets["w2"]; !ok {
		t.Error("w2 should NOT have been removed")
	}
}

func TestClearWalletCache_NonexistentWallet(t *testing.T) {
	data := emptyCacheData()
	data.Wallets["w1"] = &WalletCache{LastSignature: "sig"}

	// Should not panic.
	ClearWalletCache(data, "w999")

	if _, ok := data.Wallets["w1"]; !ok {
		t.Error("existing wallet should not be affected")
	}
}

func TestClearWalletCache_NilWallets(t *testing.T) {
	data := &CacheData{CacheVersion: 1, Wallets: nil}
	// Should not panic.
	ClearWalletCache(data, "w1")
}

// ---------- Multi-wallet isolation ----------

func TestMultiWallet_Isolation(t *testing.T) {
	data := emptyCacheData()

	AppendEntries(data, "alice", []Entry{{Sig: "a1", Ts: 1, Content: "alice diary"}}, "a1")
	AppendEntries(data, "bob", []Entry{{Sig: "b1", Ts: 2, Content: "bob diary"}}, "b1")

	aliceCache := GetWalletCache(data, "alice")
	bobCache := GetWalletCache(data, "bob")

	if len(aliceCache.Entries) != 1 || aliceCache.Entries[0].Content != "alice diary" {
		t.Error("alice entries contaminated")
	}
	if len(bobCache.Entries) != 1 || bobCache.Entries[0].Content != "bob diary" {
		t.Error("bob entries contaminated")
	}

	ClearWalletCache(data, "alice")

	if _, ok := data.Wallets["alice"]; ok {
		t.Error("alice should be cleared")
	}
	bobAfter := GetWalletCache(data, "bob")
	if len(bobAfter.Entries) != 1 {
		t.Error("bob should be unaffected by clearing alice")
	}
}

// ---------- WithLock ----------

func TestWithLock_BasicExecution(t *testing.T) {
	store, _ := newTempStore(t)

	executed := false
	err := store.WithLock(func() error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock() error: %v", err)
	}
	if !executed {
		t.Error("function was not executed under lock")
	}
}

func TestWithLock_PropagatesError(t *testing.T) {
	store, _ := newTempStore(t)

	err := store.WithLock(func() error {
		return os.ErrPermission
	})
	if err == nil {
		t.Fatal("expected error from WithLock")
	}
	if err != os.ErrPermission {
		t.Errorf("error = %v, want os.ErrPermission", err)
	}
}

// ---------- Round-trip (Save + Load) ----------

func TestRoundTrip_SaveAndLoad(t *testing.T) {
	store, _ := newTempStore(t)

	original := emptyCacheData()
	original.Wallets["w1"] = &WalletCache{
		LastSignature: "latest",
		Entries: []Entry{
			{Sig: "s1", Ts: 1000, Content: "first entry"},
			{Sig: "s2", Ts: 2000, Content: "second entry"},
		},
	}
	original.Wallets["w2"] = &WalletCache{
		LastSignature: "w2sig",
		Entries:       []Entry{},
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.CacheVersion != currentCacheVersion {
		t.Errorf("CacheVersion = %d, want %d", loaded.CacheVersion, currentCacheVersion)
	}

	w1 := loaded.Wallets["w1"]
	if w1 == nil {
		t.Fatal("w1 missing after round-trip")
	}
	if w1.LastSignature != "latest" {
		t.Errorf("w1.LastSignature = %q, want %q", w1.LastSignature, "latest")
	}
	if len(w1.Entries) != 2 {
		t.Fatalf("w1.Entries length = %d, want 2", len(w1.Entries))
	}
	if w1.Entries[1].Content != "second entry" {
		t.Errorf("second entry content = %q", w1.Entries[1].Content)
	}

	w2 := loaded.Wallets["w2"]
	if w2 == nil {
		t.Fatal("w2 missing after round-trip")
	}
	if w2.LastSignature != "w2sig" {
		t.Errorf("w2.LastSignature = %q, want %q", w2.LastSignature, "w2sig")
	}
}

// ---------- NewStore default path ----------

func TestNewStore_EmptyPath_UsesDefault(t *testing.T) {
	store := NewStore("")
	if store.Path() == "" {
		t.Error("store path should not be empty with default")
	}
	// Should contain "cache.json" somewhere.
	if filepath.Base(store.Path()) != "cache.json" {
		t.Errorf("default path base = %q, want cache.json", filepath.Base(store.Path()))
	}
}

func TestNewStore_CustomPath(t *testing.T) {
	store := NewStore("/tmp/custom/cache.json")
	if store.Path() != "/tmp/custom/cache.json" {
		t.Errorf("path = %q, want /tmp/custom/cache.json", store.Path())
	}
}

// ---------- WithLock + Save/Load integration ----------

func TestWithLock_SaveAndLoad(t *testing.T) {
	store, _ := newTempStore(t)

	err := store.WithLock(func() error {
		data := emptyCacheData()
		AppendEntries(data, "w1", []Entry{{Sig: "s1", Ts: 1, Content: "locked write"}}, "s1")
		return store.Save(data)
	})
	if err != nil {
		t.Fatalf("WithLock(Save) error: %v", err)
	}

	var loaded *CacheData
	err = store.WithLock(func() error {
		var lerr error
		loaded, lerr = store.Load()
		return lerr
	})
	if err != nil {
		t.Fatalf("WithLock(Load) error: %v", err)
	}
	if loaded.Wallets["w1"].Entries[0].Content != "locked write" {
		t.Error("data not persisted through locked operations")
	}
}

// ---------- helpers ----------

func writeJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
