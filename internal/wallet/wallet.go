// Package wallet provides Solana wallet management for HashDiary.
//
// It supports generating, saving, loading, and importing Ed25519 keypairs
// in both plaintext (Solana CLI compatible) and encrypted (scrypt + AES-256-GCM)
// formats. A unified loader auto-detects the file format.
package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mr-tron/base58"
	"golang.org/x/crypto/scrypt"
)

// Keypair holds an Ed25519 keypair used for Solana operations.
type Keypair struct {
	PrivateKey ed25519.PrivateKey // 64 bytes (seed + public key)
	PublicKey  ed25519.PublicKey  // 32 bytes
}

// Address returns the Base58-encoded public key (Solana address).
func (k *Keypair) Address() string {
	return base58.Encode(k.PublicKey)
}

// PasswordProvider is an interface for obtaining a password from various sources.
type PasswordProvider interface {
	GetPassword() (string, error)
}

// scryptParams holds the scrypt key derivation parameters.
type scryptParams struct {
	N int `json:"N"`
	R int `json:"r"`
	P int `json:"p"`
}

// encryptedWallet is the JSON structure for an encrypted wallet file.
type encryptedWallet struct {
	Address   string       `json:"address"`
	Encrypted string       `json:"encrypted"`
	Nonce     string       `json:"nonce"`
	Salt      string       `json:"salt"`
	Scrypt    scryptParams `json:"scrypt"`
}

// Default scrypt parameters.
const (
	scryptN      = 32768
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32

	// aesGCMNonceSize is the standard AES-GCM nonce size.
	aesGCMNonceSize = 12

	// scryptSaltSize is the size of the random salt for scrypt.
	scryptSaltSize = 32

	// filePerm is the permission mode for wallet files.
	filePerm = 0600

	// dirPerm is the permission mode for wallet directories.
	dirPerm = 0700
)

// GenerateKeypair generates a new random Ed25519 keypair.
func GenerateKeypair() (*Keypair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("wallet: failed to generate keypair: %w", err)
	}
	return &Keypair{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// SavePlaintext saves a keypair as a JSON byte array compatible with the Solana
// CLI's keygen format (64 integers representing the 64-byte private key).
func SavePlaintext(keypair *Keypair, path string) error {
	if err := EnsureDir(path); err != nil {
		return err
	}

	// The Solana CLI format is a JSON array of 64 bytes (the full ed25519.PrivateKey).
	bytes := make([]int, len(keypair.PrivateKey))
	for i, b := range keypair.PrivateKey {
		bytes[i] = int(b)
	}

	data, err := json.Marshal(bytes)
	if err != nil {
		return fmt.Errorf("wallet: failed to marshal keypair: %w", err)
	}

	if err := os.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("wallet: failed to write plaintext keypair to %s: %w", path, err)
	}

	return nil
}

// LoadPlaintext loads a keypair from a Solana CLI compatible JSON byte array file.
func LoadPlaintext(path string) (*Keypair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("wallet: failed to read plaintext keypair from %s: %w", path, err)
	}

	return parsePlaintext(data)
}

// parsePlaintext parses a plaintext keypair from JSON byte array data.
func parsePlaintext(data []byte) (*Keypair, error) {
	var values []byte
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("wallet: failed to parse plaintext keypair JSON: %w", err)
	}

	if len(values) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("wallet: invalid plaintext keypair length: got %d, want %d", len(values), ed25519.PrivateKeySize)
	}

	priv := ed25519.PrivateKey(values)
	pub := priv.Public().(ed25519.PublicKey)

	return &Keypair{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// SaveEncrypted encrypts a keypair with the given password using scrypt + AES-256-GCM
// and saves it as a JSON file.
func SaveEncrypted(keypair *Keypair, path string, password string) error {
	if err := EnsureDir(path); err != nil {
		return err
	}

	// Generate random salt for scrypt.
	salt := make([]byte, scryptSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("wallet: failed to generate salt: %w", err)
	}

	// Derive encryption key using scrypt.
	key, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return fmt.Errorf("wallet: failed to derive key: %w", err)
	}

	// Create AES-256-GCM cipher.
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("wallet: failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("wallet: failed to create GCM: %w", err)
	}

	// Generate random nonce.
	nonce := make([]byte, aesGCMNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("wallet: failed to generate nonce: %w", err)
	}

	// Encrypt the 64-byte private key (which contains both seed and public key).
	ciphertext := aesGCM.Seal(nil, nonce, []byte(keypair.PrivateKey), nil)

	// Build the encrypted wallet structure.
	wallet := encryptedWallet{
		Address:   keypair.Address(),
		Encrypted: base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:     base64.StdEncoding.EncodeToString(nonce),
		Salt:      base64.StdEncoding.EncodeToString(salt),
		Scrypt: scryptParams{
			N: scryptN,
			R: scryptR,
			P: scryptP,
		},
	}

	data, err := json.MarshalIndent(wallet, "", "  ")
	if err != nil {
		return fmt.Errorf("wallet: failed to marshal encrypted wallet: %w", err)
	}

	if err := os.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("wallet: failed to write encrypted wallet to %s: %w", path, err)
	}

	return nil
}

// LoadEncrypted loads and decrypts a keypair from an encrypted wallet file.
func LoadEncrypted(path string, password string) (*Keypair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("wallet: failed to read encrypted wallet from %s: %w", path, err)
	}

	return parseEncrypted(data, password)
}

// parseEncrypted parses and decrypts an encrypted wallet from JSON data.
func parseEncrypted(data []byte, password string) (*Keypair, error) {
	var wallet encryptedWallet
	if err := json.Unmarshal(data, &wallet); err != nil {
		return nil, fmt.Errorf("wallet: failed to parse encrypted wallet JSON: %w", err)
	}

	// Decode base64 fields.
	salt, err := base64.StdEncoding.DecodeString(wallet.Salt)
	if err != nil {
		return nil, fmt.Errorf("wallet: failed to decode salt: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(wallet.Nonce)
	if err != nil {
		return nil, fmt.Errorf("wallet: failed to decode nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(wallet.Encrypted)
	if err != nil {
		return nil, fmt.Errorf("wallet: failed to decode ciphertext: %w", err)
	}

	// Derive encryption key using scrypt with stored parameters.
	key, err := scrypt.Key([]byte(password), salt, wallet.Scrypt.N, wallet.Scrypt.R, wallet.Scrypt.P, scryptKeyLen)
	if err != nil {
		return nil, fmt.Errorf("wallet: failed to derive key: %w", err)
	}

	// Create AES-256-GCM cipher.
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("wallet: failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("wallet: failed to create GCM: %w", err)
	}

	// Decrypt.
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("wallet: 密码错误，请确认输入的密码是否正确")
	}

	if len(plaintext) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("wallet: decrypted data has invalid length: got %d, want %d", len(plaintext), ed25519.PrivateKeySize)
	}

	priv := ed25519.PrivateKey(plaintext)
	pub := priv.Public().(ed25519.PublicKey)

	return &Keypair{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// LoadWallet auto-detects the wallet format and loads the keypair.
// It tries plaintext (JSON array) first, then encrypted (JSON object).
// For encrypted wallets, it uses the provided PasswordProvider to obtain the password.
func LoadWallet(path string, passwordProvider PasswordProvider) (*Keypair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("wallet: 钱包文件不存在: %s，请先运行 `hash-diary wallet new` 或通过 --keypair 指定路径", path)
		}
		return nil, fmt.Errorf("wallet: failed to read wallet file %s: %w", path, err)
	}

	// Try plaintext first (JSON array of bytes).
	kp, err := parsePlaintext(data)
	if err == nil {
		return kp, nil
	}

	// Try encrypted (JSON object).
	var wallet encryptedWallet
	if err := json.Unmarshal(data, &wallet); err != nil {
		return nil, fmt.Errorf("wallet: unrecognized wallet format in %s", path)
	}

	// Validate it looks like an encrypted wallet.
	if wallet.Encrypted == "" || wallet.Nonce == "" || wallet.Salt == "" {
		return nil, fmt.Errorf("wallet: unrecognized wallet format in %s", path)
	}

	if passwordProvider == nil {
		return nil, errors.New("wallet: 加密钱包需要密码，请通过 --password 或环境变量 HASH_DIARY_PASSWORD 提供")
	}

	password, err := passwordProvider.GetPassword()
	if err != nil {
		return nil, fmt.Errorf("wallet: failed to get password: %w", err)
	}

	return parseEncrypted(data, password)
}

// IsEncrypted checks if the wallet file at the given path is in encrypted format.
func IsEncrypted(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("wallet: failed to read wallet file %s: %w", path, err)
	}

	// If it parses as a plaintext array, it's not encrypted.
	var arr []byte
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) == ed25519.PrivateKeySize {
		return false, nil
	}

	// Check if it looks like an encrypted wallet object.
	var wallet encryptedWallet
	if err := json.Unmarshal(data, &wallet); err == nil {
		if wallet.Encrypted != "" && wallet.Nonce != "" && wallet.Salt != "" {
			return true, nil
		}
	}

	return false, fmt.Errorf("wallet: unrecognized wallet format in %s", path)
}

// --- Password Providers ---

// staticPassword provides a password from a static string (e.g., --password flag).
type staticPassword struct {
	password string
}

// StaticPassword creates a PasswordProvider that returns a fixed password.
func StaticPassword(pw string) PasswordProvider {
	return &staticPassword{password: pw}
}

func (s *staticPassword) GetPassword() (string, error) {
	if s.password == "" {
		return "", errors.New("wallet: static password is empty")
	}
	return s.password, nil
}

// envPassword provides a password from an environment variable.
type envPassword struct {
	envVar string
}

// EnvPassword creates a PasswordProvider that reads from an environment variable.
func EnvPassword(envVar string) PasswordProvider {
	return &envPassword{envVar: envVar}
}

func (e *envPassword) GetPassword() (string, error) {
	pw := os.Getenv(e.envVar)
	if pw == "" {
		return "", fmt.Errorf("wallet: environment variable %s is not set or empty", e.envVar)
	}
	return pw, nil
}

// interactivePassword prompts the user for a password on the given writer
// and reads it from the given reader.
type interactivePassword struct {
	reader io.Reader
	writer io.Writer
}

// InteractivePassword creates a PasswordProvider that prompts on writer and reads from reader.
func InteractivePassword(reader io.Reader, writer io.Writer) PasswordProvider {
	return &interactivePassword{reader: reader, writer: writer}
}

func (i *interactivePassword) GetPassword() (string, error) {
	if _, err := fmt.Fprint(i.writer, "Enter wallet password: "); err != nil {
		return "", fmt.Errorf("wallet: failed to write prompt: %w", err)
	}

	var password string
	buf := make([]byte, 1024)
	n, err := i.reader.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("wallet: failed to read password: %w", err)
	}

	password = string(buf[:n])
	// Strip trailing newline(s).
	for len(password) > 0 && (password[len(password)-1] == '\n' || password[len(password)-1] == '\r') {
		password = password[:len(password)-1]
	}

	if password == "" {
		return "", errors.New("wallet: password is empty")
	}

	return password, nil
}

// chainedProvider tries multiple password providers in order.
type chainedProvider struct {
	providers []PasswordProvider
}

// ChainedProvider creates a PasswordProvider that tries providers in order,
// returning the first successful result. Typical order: --password > env > interactive.
func ChainedProvider(providers ...PasswordProvider) PasswordProvider {
	return &chainedProvider{providers: providers}
}

func (c *chainedProvider) GetPassword() (string, error) {
	var lastErr error
	for _, p := range c.providers {
		pw, err := p.GetPassword()
		if err == nil {
			return pw, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return "", fmt.Errorf("wallet: 加密钱包需要密码，请通过 --password 或环境变量 HASH_DIARY_PASSWORD 提供: %w", lastErr)
	}
	return "", errors.New("wallet: 加密钱包需要密码，请通过 --password 或环境变量 HASH_DIARY_PASSWORD 提供")
}

// --- Key Import ---

// ImportFromFile reads an existing Solana CLI keypair file and returns a Keypair.
func ImportFromFile(srcPath string) (*Keypair, error) {
	return LoadPlaintext(srcPath)
}

// ImportFromBase58PrivateKey decodes a Base58-encoded private key (64 bytes)
// and returns a Keypair.
func ImportFromBase58PrivateKey(b58 string) (*Keypair, error) {
	decoded, err := base58.Decode(b58)
	if err != nil {
		return nil, fmt.Errorf("wallet: 无效的私钥格式，支持 Base58 编码的 64 字节 Ed25519 私钥: %w", err)
	}

	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("wallet: 无效的私钥长度 %d 字节（需要 %d 字节），支持格式：Base58 编码的 Ed25519 私钥或 Solana CLI JSON 文件", len(decoded), ed25519.PrivateKeySize)
	}

	priv := ed25519.PrivateKey(decoded)
	pub := priv.Public().(ed25519.PublicKey)

	return &Keypair{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// --- Path Helpers ---

// DefaultKeypairPath returns the default wallet path: ~/.hash-diary/id.json
func DefaultKeypairPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory.
		return filepath.Join(".", ".hash-diary", "id.json")
	}
	return filepath.Join(home, ".hash-diary", "id.json")
}

// EnsureDir creates the parent directory of the given path with mode 0700 if needed.
func EnsureDir(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("wallet: failed to create directory %s: %w", dir, err)
	}
	return nil
}
