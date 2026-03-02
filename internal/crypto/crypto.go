// Package crypto provides NaCl box encryption, key conversion, and
// compression utilities for HashDiary diary entries.
package crypto

import (
	"bytes"
	"compress/zlib"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/nacl/box"
)

const (
	// NonceSize is the size of the NaCl box nonce in bytes.
	NonceSize = 24

	// KeySize is the size of X25519 keys in bytes.
	KeySize = 32

	// MACSize is the Poly1305 MAC overhead from NaCl box.
	MACSize = box.Overhead // 16 bytes

	// TotalOverhead is nonce + MAC = 40 bytes (before base64).
	TotalOverhead = NonceSize + MACSize

	// MemoPrefix is the protocol prefix for encoded memos.
	MemoPrefix = "HD:"

	// MaxMemoLen is the maximum total length of an encoded memo in bytes.
	MaxMemoLen = 512
)

// Ed25519PrivateToX25519 converts an Ed25519 private key to an X25519
// private key by hashing the seed with SHA-512 and applying clamping
// as specified in RFC 7748.
func Ed25519PrivateToX25519(privKey ed25519.PrivateKey) ([]byte, error) {
	if len(privKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("crypto: invalid Ed25519 private key length: got %d, want %d", len(privKey), ed25519.PrivateKeySize)
	}

	// The Ed25519 private key seed is the first 32 bytes.
	seed := privKey.Seed()

	// Hash the seed with SHA-512; the first 32 bytes become the X25519 scalar.
	h := sha512.Sum512(seed)

	// Apply X25519 clamping (RFC 7748, Section 5).
	h[0] &= 248
	h[31] &= 127
	h[31] |= 64

	x25519Key := make([]byte, KeySize)
	copy(x25519Key, h[:KeySize])
	return x25519Key, nil
}

// Ed25519PublicToX25519 converts an Ed25519 public key to an X25519
// public key using the birational map from Edwards to Montgomery form.
func Ed25519PublicToX25519(pubKey ed25519.PublicKey) ([]byte, error) {
	if len(pubKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("crypto: invalid Ed25519 public key length: got %d, want %d", len(pubKey), ed25519.PublicKeySize)
	}

	// Parse the Ed25519 public key as an edwards25519 point.
	p, err := new(edwards25519.Point).SetBytes(pubKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to parse Ed25519 public key: %w", err)
	}

	// Convert to Montgomery (X25519) form.
	return p.BytesMontgomery(), nil
}

// Compress applies zlib compression to the input data.
func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create zlib writer: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("crypto: failed to write compressed data: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("crypto: failed to close zlib writer: %w", err)
	}

	return buf.Bytes(), nil
}

// Decompress applies zlib decompression to the input data.
func Decompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create zlib reader: %w", err)
	}
	defer r.Close()

	result, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to decompress data: %w", err)
	}

	return result, nil
}

// Encrypt encrypts plaintext using NaCl box (Curve25519-XSalsa20-Poly1305).
// pubKey and privKey must be 32-byte X25519 keys.
// A random 24-byte nonce is generated and prepended to the ciphertext.
// Output format: nonce (24 bytes) || ciphertext (len(plaintext) + 16 bytes MAC).
func Encrypt(plaintext []byte, pubKey, privKey []byte) ([]byte, error) {
	if len(pubKey) != KeySize {
		return nil, fmt.Errorf("crypto: invalid public key length: got %d, want %d", len(pubKey), KeySize)
	}
	if len(privKey) != KeySize {
		return nil, fmt.Errorf("crypto: invalid private key length: got %d, want %d", len(privKey), KeySize)
	}

	// Generate random nonce.
	var nonce [NonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("crypto: failed to generate nonce: %w", err)
	}

	// Convert keys to fixed-size arrays.
	var pub, priv [KeySize]byte
	copy(pub[:], pubKey)
	copy(priv[:], privKey)

	// Seal prepends to `out`; we start with the nonce.
	out := make([]byte, NonceSize)
	copy(out, nonce[:])

	sealed := box.Seal(out, plaintext, &nonce, &pub, &priv)
	return sealed, nil
}

// Decrypt decrypts ciphertext produced by Encrypt using NaCl box.
// pubKey and privKey must be 32-byte X25519 keys.
// The first 24 bytes of ciphertext are interpreted as the nonce.
func Decrypt(ciphertext []byte, pubKey, privKey []byte) ([]byte, error) {
	if len(pubKey) != KeySize {
		return nil, fmt.Errorf("crypto: invalid public key length: got %d, want %d", len(pubKey), KeySize)
	}
	if len(privKey) != KeySize {
		return nil, fmt.Errorf("crypto: invalid private key length: got %d, want %d", len(privKey), KeySize)
	}
	if len(ciphertext) < TotalOverhead {
		return nil, fmt.Errorf("crypto: ciphertext too short: got %d bytes, minimum is %d", len(ciphertext), TotalOverhead)
	}

	// Extract nonce from the first 24 bytes.
	var nonce [NonceSize]byte
	copy(nonce[:], ciphertext[:NonceSize])

	// Convert keys to fixed-size arrays.
	var pub, priv [KeySize]byte
	copy(pub[:], pubKey)
	copy(priv[:], privKey)

	// Open the box.
	plaintext, ok := box.Open(nil, ciphertext[NonceSize:], &nonce, &pub, &priv)
	if !ok {
		return nil, errors.New("crypto: decryption failed (authentication error)")
	}

	return plaintext, nil
}

// EncodeMemo implements the full memo encoding pipeline:
//  1. UTF-8 input (Go strings are UTF-8 by default)
//  2. Zlib compress
//  3. NaCl box encrypt (converts Ed25519 keys to X25519 internally)
//  4. Standard Base64 encode (RFC 4648)
//  5. Prepend "HD:" prefix
//  6. Check total length <= 512 bytes
//
// senderPrivKey is the sender's Ed25519 private key.
// recipientPubKey is the recipient's Ed25519 public key.
func EncodeMemo(plaintext string, recipientPubKey ed25519.PublicKey, senderPrivKey ed25519.PrivateKey) (string, error) {
	// Convert Ed25519 keys to X25519.
	x25519Priv, err := Ed25519PrivateToX25519(senderPrivKey)
	if err != nil {
		return "", fmt.Errorf("crypto: EncodeMemo: failed to convert sender private key: %w", err)
	}

	x25519Pub, err := Ed25519PublicToX25519(recipientPubKey)
	if err != nil {
		return "", fmt.Errorf("crypto: EncodeMemo: failed to convert recipient public key: %w", err)
	}

	// Step 2: Zlib compress.
	compressed, err := Compress([]byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("crypto: EncodeMemo: compression failed: %w", err)
	}

	// Step 3: NaCl box encrypt.
	encrypted, err := Encrypt(compressed, x25519Pub, x25519Priv)
	if err != nil {
		return "", fmt.Errorf("crypto: EncodeMemo: encryption failed: %w", err)
	}

	// Step 4: Base64 encode.
	encoded := base64.StdEncoding.EncodeToString(encrypted)

	// Step 5: Prepend prefix.
	memo := MemoPrefix + encoded

	// Step 6: Check length.
	if len(memo) > MaxMemoLen {
		return "", fmt.Errorf("crypto: EncodeMemo: memo exceeds max length: %d bytes (max %d)", len(memo), MaxMemoLen)
	}

	return memo, nil
}

// DecodeMemo implements the reverse memo decoding pipeline:
//  1. Check and strip "HD:" prefix
//  2. Base64 decode
//  3. NaCl box decrypt (converts Ed25519 keys to X25519 internally)
//  4. Zlib decompress
//  5. Return UTF-8 string
//
// senderPubKey is the sender's Ed25519 public key.
// recipientPrivKey is the recipient's Ed25519 private key.
func DecodeMemo(memo string, senderPubKey ed25519.PublicKey, recipientPrivKey ed25519.PrivateKey) (string, error) {
	// Step 1: Check and strip prefix.
	if !strings.HasPrefix(memo, MemoPrefix) {
		return "", fmt.Errorf("crypto: DecodeMemo: memo missing %q prefix", MemoPrefix)
	}
	encoded := memo[len(MemoPrefix):]

	// Step 2: Base64 decode.
	encrypted, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto: DecodeMemo: base64 decode failed: %w", err)
	}

	// Convert Ed25519 keys to X25519.
	x25519Priv, err := Ed25519PrivateToX25519(recipientPrivKey)
	if err != nil {
		return "", fmt.Errorf("crypto: DecodeMemo: failed to convert recipient private key: %w", err)
	}

	x25519Pub, err := Ed25519PublicToX25519(senderPubKey)
	if err != nil {
		return "", fmt.Errorf("crypto: DecodeMemo: failed to convert sender public key: %w", err)
	}

	// Step 3: NaCl box decrypt.
	compressed, err := Decrypt(encrypted, x25519Pub, x25519Priv)
	if err != nil {
		return "", fmt.Errorf("crypto: DecodeMemo: decryption failed: %w", err)
	}

	// Step 4: Zlib decompress.
	plaintext, err := Decompress(compressed)
	if err != nil {
		return "", fmt.Errorf("crypto: DecodeMemo: decompression failed: %w", err)
	}

	// Step 5: Return UTF-8 string.
	return string(plaintext), nil
}
