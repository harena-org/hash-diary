package wallet

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mr-tron/base58"
)

// --- GenerateKeypair tests ---

func TestGenerateKeypair(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	// Check private key length (64 bytes).
	if len(kp.PrivateKey) != ed25519.PrivateKeySize {
		t.Errorf("PrivateKey length = %d, want %d", len(kp.PrivateKey), ed25519.PrivateKeySize)
	}

	// Check public key length (32 bytes).
	if len(kp.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("PublicKey length = %d, want %d", len(kp.PublicKey), ed25519.PublicKeySize)
	}

	// Verify signing works.
	msg := []byte("test message")
	sig := ed25519.Sign(kp.PrivateKey, msg)
	if !ed25519.Verify(kp.PublicKey, msg, sig) {
		t.Error("generated keypair fails sign/verify")
	}
}

func TestGenerateKeypair_Unique(t *testing.T) {
	kp1, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	kp2, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	if bytes.Equal(kp1.PrivateKey, kp2.PrivateKey) {
		t.Error("two generated keypairs have identical private keys")
	}
	if bytes.Equal(kp1.PublicKey, kp2.PublicKey) {
		t.Error("two generated keypairs have identical public keys")
	}
}

func TestKeypair_Address(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	addr := kp.Address()
	if addr == "" {
		t.Fatal("Address() returned empty string")
	}

	// Decode the address back and verify it matches the public key.
	decoded, err := base58.Decode(addr)
	if err != nil {
		t.Fatalf("failed to decode address: %v", err)
	}
	if !bytes.Equal(decoded, kp.PublicKey) {
		t.Error("decoded address does not match public key")
	}
}

// --- Plaintext save/load tests ---

func TestSavePlaintext_LoadPlaintext_RoundTrip(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test-wallet.json")

	// Save.
	if err := SavePlaintext(kp, path); err != nil {
		t.Fatalf("SavePlaintext() error = %v", err)
	}

	// Load.
	loaded, err := LoadPlaintext(path)
	if err != nil {
		t.Fatalf("LoadPlaintext() error = %v", err)
	}

	// Compare.
	if !bytes.Equal(kp.PrivateKey, loaded.PrivateKey) {
		t.Error("loaded private key does not match original")
	}
	if !bytes.Equal(kp.PublicKey, loaded.PublicKey) {
		t.Error("loaded public key does not match original")
	}
	if kp.Address() != loaded.Address() {
		t.Error("loaded address does not match original")
	}
}

func TestSavePlaintext_SolanaFormat(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test-wallet.json")

	if err := SavePlaintext(kp, path); err != nil {
		t.Fatalf("SavePlaintext() error = %v", err)
	}

	// Read the raw file and verify it's a JSON array of 64 integers.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var values []int
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("failed to unmarshal as int array: %v", err)
	}

	if len(values) != 64 {
		t.Errorf("plaintext array length = %d, want 64", len(values))
	}

	// Verify each value is a valid byte.
	for i, v := range values {
		if v < 0 || v > 255 {
			t.Errorf("value[%d] = %d, out of byte range", i, v)
		}
	}
}

func TestLoadPlaintext_NotFound(t *testing.T) {
	_, err := LoadPlaintext("/nonexistent/path/wallet.json")
	if err == nil {
		t.Fatal("LoadPlaintext() expected error for nonexistent file")
	}
}

func TestLoadPlaintext_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0600)

	_, err := LoadPlaintext(path)
	if err == nil {
		t.Fatal("LoadPlaintext() expected error for invalid JSON")
	}
}

func TestLoadPlaintext_WrongLength(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "short.json")
	// Write a JSON array with only 32 bytes.
	short := make([]byte, 32)
	data, _ := json.Marshal(short)
	os.WriteFile(path, data, 0600)

	_, err := LoadPlaintext(path)
	if err == nil {
		t.Fatal("LoadPlaintext() expected error for wrong length")
	}
	if !strings.Contains(err.Error(), "invalid plaintext keypair length") {
		t.Errorf("error message = %q, want to contain 'invalid plaintext keypair length'", err.Error())
	}
}

// --- Encrypted save/load tests ---

func TestSaveEncrypted_LoadEncrypted_RoundTrip(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted-wallet.json")
	password := "test-password-123"

	// Save encrypted.
	if err := SaveEncrypted(kp, path, password); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	// Load encrypted.
	loaded, err := LoadEncrypted(path, password)
	if err != nil {
		t.Fatalf("LoadEncrypted() error = %v", err)
	}

	// Compare.
	if !bytes.Equal(kp.PrivateKey, loaded.PrivateKey) {
		t.Error("loaded private key does not match original")
	}
	if !bytes.Equal(kp.PublicKey, loaded.PublicKey) {
		t.Error("loaded public key does not match original")
	}
	if kp.Address() != loaded.Address() {
		t.Error("loaded address does not match original")
	}
}

func TestSaveEncrypted_JSONStructure(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted-wallet.json")

	if err := SaveEncrypted(kp, path, "password"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var wallet encryptedWallet
	if err := json.Unmarshal(data, &wallet); err != nil {
		t.Fatalf("failed to unmarshal encrypted wallet: %v", err)
	}

	// Verify all fields are present.
	if wallet.Address == "" {
		t.Error("encrypted wallet missing address")
	}
	if wallet.Address != kp.Address() {
		t.Errorf("encrypted wallet address = %q, want %q", wallet.Address, kp.Address())
	}
	if wallet.Encrypted == "" {
		t.Error("encrypted wallet missing encrypted field")
	}
	if wallet.Nonce == "" {
		t.Error("encrypted wallet missing nonce")
	}
	if wallet.Salt == "" {
		t.Error("encrypted wallet missing salt")
	}
	if wallet.Scrypt.N != scryptN {
		t.Errorf("scrypt N = %d, want %d", wallet.Scrypt.N, scryptN)
	}
	if wallet.Scrypt.R != scryptR {
		t.Errorf("scrypt r = %d, want %d", wallet.Scrypt.R, scryptR)
	}
	if wallet.Scrypt.P != scryptP {
		t.Errorf("scrypt p = %d, want %d", wallet.Scrypt.P, scryptP)
	}
}

func TestLoadEncrypted_WrongPassword(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted-wallet.json")

	if err := SaveEncrypted(kp, path, "correct-password"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	_, err = LoadEncrypted(path, "wrong-password")
	if err == nil {
		t.Fatal("LoadEncrypted() expected error for wrong password")
	}
	if !strings.Contains(err.Error(), "密码错误") {
		t.Errorf("error message = %q, want to contain '密码错误'", err.Error())
	}
}

func TestLoadEncrypted_NotFound(t *testing.T) {
	_, err := LoadEncrypted("/nonexistent/path/wallet.json", "password")
	if err == nil {
		t.Fatal("LoadEncrypted() expected error for nonexistent file")
	}
}

// --- Unified loader (LoadWallet) tests ---

func TestLoadWallet_Plaintext(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "plain.json")

	if err := SavePlaintext(kp, path); err != nil {
		t.Fatalf("SavePlaintext() error = %v", err)
	}

	// LoadWallet should detect plaintext and not require a password.
	loaded, err := LoadWallet(path, nil)
	if err != nil {
		t.Fatalf("LoadWallet() error = %v", err)
	}

	if !bytes.Equal(kp.PrivateKey, loaded.PrivateKey) {
		t.Error("loaded private key does not match original")
	}
}

func TestLoadWallet_Encrypted(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted.json")
	password := "wallet-password"

	if err := SaveEncrypted(kp, path, password); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	// LoadWallet should detect encrypted and use the password provider.
	loaded, err := LoadWallet(path, StaticPassword(password))
	if err != nil {
		t.Fatalf("LoadWallet() error = %v", err)
	}

	if !bytes.Equal(kp.PrivateKey, loaded.PrivateKey) {
		t.Error("loaded private key does not match original")
	}
}

func TestLoadWallet_EncryptedNoProvider(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted.json")

	if err := SaveEncrypted(kp, path, "password"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	_, err = LoadWallet(path, nil)
	if err == nil {
		t.Fatal("LoadWallet() expected error when no password provider for encrypted wallet")
	}
	if !strings.Contains(err.Error(), "--password") {
		t.Errorf("error message = %q, want to contain '--password'", err.Error())
	}
}

func TestLoadWallet_InvalidFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	os.WriteFile(path, []byte(`{"foo":"bar"}`), 0600)

	_, err := LoadWallet(path, nil)
	if err == nil {
		t.Fatal("LoadWallet() expected error for unrecognized format")
	}
}

// --- IsEncrypted tests ---

func TestIsEncrypted_Plaintext(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "plain.json")

	if err := SavePlaintext(kp, path); err != nil {
		t.Fatalf("SavePlaintext() error = %v", err)
	}

	encrypted, err := IsEncrypted(path)
	if err != nil {
		t.Fatalf("IsEncrypted() error = %v", err)
	}
	if encrypted {
		t.Error("IsEncrypted() = true for plaintext wallet")
	}
}

func TestIsEncrypted_Encrypted(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted.json")

	if err := SaveEncrypted(kp, path, "password"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	encrypted, err := IsEncrypted(path)
	if err != nil {
		t.Fatalf("IsEncrypted() error = %v", err)
	}
	if !encrypted {
		t.Error("IsEncrypted() = false for encrypted wallet")
	}
}

func TestIsEncrypted_UnknownFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unknown.json")
	os.WriteFile(path, []byte("garbage"), 0600)

	_, err := IsEncrypted(path)
	if err == nil {
		t.Fatal("IsEncrypted() expected error for unknown format")
	}
}

// --- Password Provider tests ---

func TestStaticPassword(t *testing.T) {
	p := StaticPassword("my-password")
	pw, err := p.GetPassword()
	if err != nil {
		t.Fatalf("GetPassword() error = %v", err)
	}
	if pw != "my-password" {
		t.Errorf("GetPassword() = %q, want %q", pw, "my-password")
	}
}

func TestStaticPassword_Empty(t *testing.T) {
	p := StaticPassword("")
	_, err := p.GetPassword()
	if err == nil {
		t.Fatal("GetPassword() expected error for empty password")
	}
}

func TestEnvPassword(t *testing.T) {
	const envKey = "HASH_DIARY_TEST_PASSWORD_XYZ123"
	os.Setenv(envKey, "env-password")
	defer os.Unsetenv(envKey)

	p := EnvPassword(envKey)
	pw, err := p.GetPassword()
	if err != nil {
		t.Fatalf("GetPassword() error = %v", err)
	}
	if pw != "env-password" {
		t.Errorf("GetPassword() = %q, want %q", pw, "env-password")
	}
}

func TestEnvPassword_NotSet(t *testing.T) {
	p := EnvPassword("NONEXISTENT_ENV_VAR_FOR_TEST")
	_, err := p.GetPassword()
	if err == nil {
		t.Fatal("GetPassword() expected error for unset env var")
	}
}

func TestInteractivePassword(t *testing.T) {
	input := strings.NewReader("interactive-password\n")
	output := &bytes.Buffer{}

	p := InteractivePassword(input, output)
	pw, err := p.GetPassword()
	if err != nil {
		t.Fatalf("GetPassword() error = %v", err)
	}
	if pw != "interactive-password" {
		t.Errorf("GetPassword() = %q, want %q", pw, "interactive-password")
	}
	if !strings.Contains(output.String(), "Enter wallet password") {
		t.Errorf("prompt output = %q, want to contain 'Enter wallet password'", output.String())
	}
}

func TestInteractivePassword_Empty(t *testing.T) {
	input := strings.NewReader("\n")
	output := &bytes.Buffer{}

	p := InteractivePassword(input, output)
	_, err := p.GetPassword()
	if err == nil {
		t.Fatal("GetPassword() expected error for empty interactive password")
	}
}

func TestChainedProvider_StaticFirst(t *testing.T) {
	p := ChainedProvider(
		StaticPassword("static-pw"),
		EnvPassword("NONEXISTENT_ENV"),
	)
	pw, err := p.GetPassword()
	if err != nil {
		t.Fatalf("GetPassword() error = %v", err)
	}
	if pw != "static-pw" {
		t.Errorf("GetPassword() = %q, want %q", pw, "static-pw")
	}
}

func TestChainedProvider_FallbackToEnv(t *testing.T) {
	const envKey = "HASH_DIARY_TEST_CHAIN_PW"
	os.Setenv(envKey, "env-pw")
	defer os.Unsetenv(envKey)

	p := ChainedProvider(
		StaticPassword(""), // will fail (empty)
		EnvPassword(envKey),
	)
	pw, err := p.GetPassword()
	if err != nil {
		t.Fatalf("GetPassword() error = %v", err)
	}
	if pw != "env-pw" {
		t.Errorf("GetPassword() = %q, want %q", pw, "env-pw")
	}
}

func TestChainedProvider_FallbackToInteractive(t *testing.T) {
	input := strings.NewReader("interactive-pw\n")
	output := &bytes.Buffer{}

	p := ChainedProvider(
		StaticPassword(""),                    // will fail (empty)
		EnvPassword("NONEXISTENT_ENV_VAR_XY"), // will fail
		InteractivePassword(input, output),
	)
	pw, err := p.GetPassword()
	if err != nil {
		t.Fatalf("GetPassword() error = %v", err)
	}
	if pw != "interactive-pw" {
		t.Errorf("GetPassword() = %q, want %q", pw, "interactive-pw")
	}
}

func TestChainedProvider_AllFail(t *testing.T) {
	p := ChainedProvider(
		StaticPassword(""),
		EnvPassword("NONEXISTENT_ENV_VAR_AA"),
	)
	_, err := p.GetPassword()
	if err == nil {
		t.Fatal("GetPassword() expected error when all providers fail")
	}
	if !strings.Contains(err.Error(), "--password") {
		t.Errorf("error message = %q, want to contain '--password'", err.Error())
	}
}

// --- Key Import tests ---

func TestImportFromFile(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "import-src.json")

	if err := SavePlaintext(kp, path); err != nil {
		t.Fatalf("SavePlaintext() error = %v", err)
	}

	imported, err := ImportFromFile(path)
	if err != nil {
		t.Fatalf("ImportFromFile() error = %v", err)
	}

	if !bytes.Equal(kp.PrivateKey, imported.PrivateKey) {
		t.Error("imported private key does not match original")
	}
	if !bytes.Equal(kp.PublicKey, imported.PublicKey) {
		t.Error("imported public key does not match original")
	}
}

func TestImportFromBase58PrivateKey(t *testing.T) {
	// Generate a keypair, encode its private key as Base58, then import.
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	b58 := base58.Encode(kp.PrivateKey)

	imported, err := ImportFromBase58PrivateKey(b58)
	if err != nil {
		t.Fatalf("ImportFromBase58PrivateKey() error = %v", err)
	}

	if !bytes.Equal(kp.PrivateKey, imported.PrivateKey) {
		t.Error("imported private key does not match original")
	}
	if !bytes.Equal(kp.PublicKey, imported.PublicKey) {
		t.Error("imported public key does not match original")
	}
	if kp.Address() != imported.Address() {
		t.Error("imported address does not match original")
	}
}

func TestImportFromBase58PrivateKey_Invalid(t *testing.T) {
	_, err := ImportFromBase58PrivateKey("notavalidbase58key!!!")
	if err == nil {
		t.Fatal("ImportFromBase58PrivateKey() expected error for invalid Base58")
	}
}

func TestImportFromBase58PrivateKey_WrongLength(t *testing.T) {
	// Encode only 32 bytes (too short for a private key).
	short := make([]byte, 32)
	b58 := base58.Encode(short)

	_, err := ImportFromBase58PrivateKey(b58)
	if err == nil {
		t.Fatal("ImportFromBase58PrivateKey() expected error for wrong length")
	}
	if !strings.Contains(err.Error(), "无效的私钥长度") {
		t.Errorf("error message = %q, want to contain '无效的私钥长度'", err.Error())
	}
}

// --- Path Helper tests ---

func TestDefaultKeypairPath(t *testing.T) {
	path := DefaultKeypairPath()
	if path == "" {
		t.Fatal("DefaultKeypairPath() returned empty string")
	}
	if !strings.HasSuffix(path, filepath.Join(".hash-diary", "id.json")) {
		t.Errorf("DefaultKeypairPath() = %q, want suffix %q", path, filepath.Join(".hash-diary", "id.json"))
	}
}

func TestEnsureDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "wallet.json")

	if err := EnsureDir(path); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	parentDir := filepath.Dir(path)
	info, err := os.Stat(parentDir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Error("parent directory was not created")
	}
}

// --- File permissions tests ---

func TestSavePlaintext_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission test not applicable on Windows")
	}

	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "perm-test.json")

	if err := SavePlaintext(kp, path); err != nil {
		t.Fatalf("SavePlaintext() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permission = %o, want %o", perm, 0600)
	}
}

func TestSaveEncrypted_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission test not applicable on Windows")
	}

	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "perm-encrypted.json")

	if err := SaveEncrypted(kp, path, "password"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permission = %o, want %o", perm, 0600)
	}
}

func TestEnsureDir_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission test not applicable on Windows")
	}

	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "new-parent")
	path := filepath.Join(nestedDir, "wallet.json")

	if err := EnsureDir(path); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	info, err := os.Stat(nestedDir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0700 {
		t.Errorf("directory permission = %o, want %o", perm, 0700)
	}
}

// --- Solana CLI compatibility test ---

func TestSolanaCLIFormat_Compatibility(t *testing.T) {
	// Simulate a Solana CLI keygen file (64-byte JSON array).
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	// Build the Solana CLI format manually.
	intArray := make([]int, 64)
	for i, b := range kp.PrivateKey {
		intArray[i] = int(b)
	}
	data, err := json.Marshal(intArray)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "solana-cli.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load using our function.
	loaded, err := LoadPlaintext(path)
	if err != nil {
		t.Fatalf("LoadPlaintext() error = %v", err)
	}

	if !bytes.Equal(kp.PrivateKey, loaded.PrivateKey) {
		t.Error("Solana CLI format not compatible: private keys don't match")
	}
	if !bytes.Equal(kp.PublicKey, loaded.PublicKey) {
		t.Error("Solana CLI format not compatible: public keys don't match")
	}
}

// --- Edge case: save to nested directory that doesn't exist ---

func TestSavePlaintext_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "wallet.json")

	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	if err := SavePlaintext(kp, path); err != nil {
		t.Fatalf("SavePlaintext() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file was not created at nested path")
	}
}

func TestSaveEncrypted_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x", "y", "z", "wallet.json")

	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	if err := SaveEncrypted(kp, path, "password"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file was not created at nested path")
	}
}

// --- Encrypted wallet round-trip with various passwords ---

func TestEncryptedWallet_SpecialCharPasswords(t *testing.T) {
	passwords := []string{
		"simple",
		"with spaces in it",
		"special!@#$%^&*()",
		"unicode-\u00e9\u00e0\u00fc\u00f1",
		"very-long-password-" + strings.Repeat("a", 200),
	}

	for _, pw := range passwords {
		t.Run(fmt.Sprintf("password=%q", pw[:min(len(pw), 20)]), func(t *testing.T) {
			kp, err := GenerateKeypair()
			if err != nil {
				t.Fatalf("GenerateKeypair() error = %v", err)
			}

			dir := t.TempDir()
			path := filepath.Join(dir, "wallet.json")

			if err := SaveEncrypted(kp, path, pw); err != nil {
				t.Fatalf("SaveEncrypted() error = %v", err)
			}

			loaded, err := LoadEncrypted(path, pw)
			if err != nil {
				t.Fatalf("LoadEncrypted() error = %v", err)
			}

			if !bytes.Equal(kp.PrivateKey, loaded.PrivateKey) {
				t.Error("loaded private key does not match original")
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- Wallet file not found suggests wallet new ---

func TestLoadWallet_FileNotFound_SuggestsWalletNew(t *testing.T) {
	_, err := LoadWallet("/nonexistent/path/id.json", nil)
	if err == nil {
		t.Fatal("LoadWallet() expected error for nonexistent file")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "wallet new") {
		t.Errorf("error should suggest 'wallet new', got: %q", errMsg)
	}
	if !strings.Contains(errMsg, "--keypair") {
		t.Errorf("error should suggest '--keypair', got: %q", errMsg)
	}
}

// --- Wrong password gives clear error ---

func TestLoadWallet_Encrypted_WrongPassword(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted.json")

	if err := SaveEncrypted(kp, path, "correct-password"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	_, err = LoadWallet(path, StaticPassword("wrong-password"))
	if err == nil {
		t.Fatal("LoadWallet() expected error for wrong password")
	}
	if !strings.Contains(err.Error(), "密码错误") {
		t.Errorf("error should contain '密码错误', got: %q", err.Error())
	}
}

// --- Encrypted wallet with no provider gives clear error ---

func TestLoadWallet_Encrypted_NilProvider_SuggestsPasswordFlag(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted.json")

	if err := SaveEncrypted(kp, path, "password"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	_, err = LoadWallet(path, nil)
	if err == nil {
		t.Fatal("LoadWallet() expected error when no password provider")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "--password") {
		t.Errorf("error should mention '--password', got: %q", errMsg)
	}
	if !strings.Contains(errMsg, "HASH_DIARY_PASSWORD") {
		t.Errorf("error should mention 'HASH_DIARY_PASSWORD', got: %q", errMsg)
	}
}

// --- Invalid Base58 private key gives clear format hint ---

func TestImportFromBase58PrivateKey_InvalidFormat_SuggestsFormat(t *testing.T) {
	_, err := ImportFromBase58PrivateKey("notavalidbase58key!!!")
	if err == nil {
		t.Fatal("expected error for invalid Base58")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "Base58") {
		t.Errorf("error should mention 'Base58', got: %q", errMsg)
	}
	if !strings.Contains(errMsg, "Ed25519") {
		t.Errorf("error should mention 'Ed25519', got: %q", errMsg)
	}
}
