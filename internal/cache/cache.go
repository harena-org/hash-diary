// Package cache provides local caching of diary entries for HashDiary.
//
// Cache files are stored at ~/.hash-diary/cache.json by default, with
// strict file permissions (600 for files, 700 for directories) to protect
// decrypted diary content.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// currentCacheVersion is the expected cache format version.
// When the on-disk version differs, the file is backed up and a fresh
// cache is returned.
const currentCacheVersion = 1

// Entry represents a single cached diary entry.
type Entry struct {
	Sig     string `json:"sig"`
	Ts      int64  `json:"ts"`
	Content string `json:"content"`
}

// WalletCache holds cached entries and sync state for a single wallet.
type WalletCache struct {
	LastSignature string  `json:"last_signature"`
	Entries       []Entry `json:"entries"`
}

// CacheData is the top-level on-disk cache structure.
type CacheData struct {
	CacheVersion int                      `json:"cache_version"`
	Wallets      map[string]*WalletCache  `json:"wallets"`
}

// Store manages reading and writing the cache file, including file locking.
type Store struct {
	path     string
	lockPath string
}

// defaultCachePath returns ~/.hash-diary/cache.json.
func defaultCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".hash-diary", "cache.json"), nil
}

// NewStore creates a cache store.  If path is empty the default
// (~/.hash-diary/cache.json) is used.
func NewStore(path string) *Store {
	if path == "" {
		p, err := defaultCachePath()
		if err != nil {
			// Fall back to a path that will fail gracefully later.
			p = filepath.Join(os.TempDir(), "hash-diary-cache.json")
		}
		path = p
	}
	return &Store{
		path:     path,
		lockPath: path + ".lock",
	}
}

// Path returns the cache file path managed by this store.
func (s *Store) Path() string {
	return s.path
}

// Load reads and parses the cache file.
//
// Behaviour on edge cases:
//   - File does not exist: returns an empty CacheData (no error).
//   - JSON parse fails: backs up to cache.json.bak, returns empty cache.
//   - cache_version mismatch: backs up to cache.json.bak, returns empty cache.
func (s *Store) Load() (*CacheData, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyCacheData(), nil
		}
		return nil, fmt.Errorf("reading cache file: %w", err)
	}

	var cd CacheData
	if err := json.Unmarshal(data, &cd); err != nil {
		// Corrupt JSON — backup and return empty.
		_ = s.backup()
		return emptyCacheData(), nil
	}

	if cd.CacheVersion != currentCacheVersion {
		_ = s.backup()
		return emptyCacheData(), nil
	}

	// Ensure the Wallets map is never nil.
	if cd.Wallets == nil {
		cd.Wallets = make(map[string]*WalletCache)
	}

	return &cd, nil
}

// Save writes the cache data to disk atomically.
//
// It ensures the parent directory exists (mode 700) and writes the file
// with mode 600.  The write is atomic: data is first written to a
// temporary file in the same directory, then renamed into place.
func (s *Store) Save(data *CacheData) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling cache data: %w", err)
	}

	// Atomic write: temp file + rename.
	tmp, err := os.CreateTemp(dir, "cache-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("setting temp file permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// WithLock executes fn while holding an exclusive file lock on the cache.
// The lock file is separate from the cache file itself (<path>.lock).
func (s *Store) WithLock(fn func() error) error {
	dir := filepath.Dir(s.lockPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	fl := flock.New(s.lockPath)
	if err := fl.Lock(); err != nil {
		return fmt.Errorf("acquiring file lock: %w", err)
	}
	defer fl.Unlock()

	return fn()
}

// GetWalletCache returns the cache for a specific wallet address.
// If no cache exists for the address, an empty WalletCache is returned
// (it is NOT inserted into data.Wallets).
func GetWalletCache(data *CacheData, address string) *WalletCache {
	if data.Wallets == nil {
		return &WalletCache{}
	}
	wc, ok := data.Wallets[address]
	if !ok || wc == nil {
		return &WalletCache{}
	}
	return wc
}

// GetLastSignature returns the last synced transaction signature for the
// given wallet address, or "" if no cache exists.
func GetLastSignature(data *CacheData, address string) string {
	return GetWalletCache(data, address).LastSignature
}

// AppendEntries appends new entries to the wallet's cache and updates
// LastSignature.  If the wallet has no existing cache entry, one is created.
func AppendEntries(data *CacheData, address string, entries []Entry, lastSig string) {
	if data.Wallets == nil {
		data.Wallets = make(map[string]*WalletCache)
	}
	wc, ok := data.Wallets[address]
	if !ok || wc == nil {
		wc = &WalletCache{}
		data.Wallets[address] = wc
	}
	wc.Entries = append(wc.Entries, entries...)
	wc.LastSignature = lastSig
}

// ClearWalletCache removes all cached data for the given wallet address.
func ClearWalletCache(data *CacheData, address string) {
	if data.Wallets == nil {
		return
	}
	delete(data.Wallets, address)
}

// backup copies the current cache file to <path>.bak.
func (s *Store) backup() error {
	bakPath := s.path + ".bak"
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return os.WriteFile(bakPath, data, 0600)
}

// emptyCacheData returns a fresh, initialised CacheData.
func emptyCacheData() *CacheData {
	return &CacheData{
		CacheVersion: currentCacheVersion,
		Wallets:      make(map[string]*WalletCache),
	}
}
