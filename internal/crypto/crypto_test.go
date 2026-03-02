package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/crypto/curve25519"
)

// generateTestKeypair creates a fresh Ed25519 keypair for testing.
func generateTestKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate Ed25519 keypair: %v", err)
	}
	return pub, priv
}

// ---------------------------------------------------------------------------
// 2.1 Key Conversion Tests
// ---------------------------------------------------------------------------

func TestEd25519PrivateToX25519_Valid(t *testing.T) {
	_, priv := generateTestKeypair(t)

	x25519Priv, err := Ed25519PrivateToX25519(priv)
	if err != nil {
		t.Fatalf("Ed25519PrivateToX25519 failed: %v", err)
	}
	if len(x25519Priv) != KeySize {
		t.Errorf("expected X25519 private key length %d, got %d", KeySize, len(x25519Priv))
	}

	// Verify clamping was applied.
	if x25519Priv[0]&7 != 0 {
		t.Error("X25519 private key low 3 bits not cleared (clamping)")
	}
	if x25519Priv[31]&128 != 0 {
		t.Error("X25519 private key high bit not cleared (clamping)")
	}
	if x25519Priv[31]&64 == 0 {
		t.Error("X25519 private key bit 254 not set (clamping)")
	}
}

func TestEd25519PrivateToX25519_InvalidLength(t *testing.T) {
	_, err := Ed25519PrivateToX25519(ed25519.PrivateKey(make([]byte, 10)))
	if err == nil {
		t.Fatal("expected error for invalid key length, got nil")
	}
}

func TestEd25519PublicToX25519_Valid(t *testing.T) {
	pub, _ := generateTestKeypair(t)

	x25519Pub, err := Ed25519PublicToX25519(pub)
	if err != nil {
		t.Fatalf("Ed25519PublicToX25519 failed: %v", err)
	}
	if len(x25519Pub) != KeySize {
		t.Errorf("expected X25519 public key length %d, got %d", KeySize, len(x25519Pub))
	}
}

func TestEd25519PublicToX25519_InvalidLength(t *testing.T) {
	_, err := Ed25519PublicToX25519(ed25519.PublicKey(make([]byte, 10)))
	if err == nil {
		t.Fatal("expected error for invalid key length, got nil")
	}
}

func TestKeyConversion_RoundTrip(t *testing.T) {
	// The X25519 public key derived from the private key should match
	// the X25519 public key derived from the Ed25519 public key.
	pub, priv := generateTestKeypair(t)

	x25519Priv, err := Ed25519PrivateToX25519(priv)
	if err != nil {
		t.Fatalf("Ed25519PrivateToX25519 failed: %v", err)
	}

	x25519PubFromPriv, err := curve25519.X25519(x25519Priv, curve25519.Basepoint)
	if err != nil {
		t.Fatalf("curve25519.X25519 scalar base mult failed: %v", err)
	}

	x25519PubFromPub, err := Ed25519PublicToX25519(pub)
	if err != nil {
		t.Fatalf("Ed25519PublicToX25519 failed: %v", err)
	}

	if len(x25519PubFromPriv) != len(x25519PubFromPub) {
		t.Fatalf("length mismatch: %d vs %d", len(x25519PubFromPriv), len(x25519PubFromPub))
	}
	for i := range x25519PubFromPriv {
		if x25519PubFromPriv[i] != x25519PubFromPub[i] {
			t.Fatalf("X25519 public key mismatch at byte %d: priv-derived %02x vs pub-derived %02x",
				i, x25519PubFromPriv[i], x25519PubFromPub[i])
		}
	}
}

func TestKeyConversion_Deterministic(t *testing.T) {
	_, priv := generateTestKeypair(t)
	pub := priv.Public().(ed25519.PublicKey)

	x25519Priv1, _ := Ed25519PrivateToX25519(priv)
	x25519Priv2, _ := Ed25519PrivateToX25519(priv)
	x25519Pub1, _ := Ed25519PublicToX25519(pub)
	x25519Pub2, _ := Ed25519PublicToX25519(pub)

	for i := range x25519Priv1 {
		if x25519Priv1[i] != x25519Priv2[i] {
			t.Fatal("Ed25519PrivateToX25519 is not deterministic")
		}
	}
	for i := range x25519Pub1 {
		if x25519Pub1[i] != x25519Pub2[i] {
			t.Fatal("Ed25519PublicToX25519 is not deterministic")
		}
	}
}

// ---------------------------------------------------------------------------
// 2.2 Compression Tests
// ---------------------------------------------------------------------------

func TestCompressDecompress_RoundTrip(t *testing.T) {
	data := []byte("Hello, World! This is a test of zlib compression.")
	compressed, err := Compress(data)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if string(decompressed) != string(data) {
		t.Errorf("round-trip mismatch:\n  got:  %q\n  want: %q", decompressed, data)
	}
}

func TestCompressDecompress_EmptyData(t *testing.T) {
	compressed, err := Compress([]byte{})
	if err != nil {
		t.Fatalf("Compress empty data failed: %v", err)
	}

	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress empty data failed: %v", err)
	}

	if len(decompressed) != 0 {
		t.Errorf("expected empty result, got %d bytes", len(decompressed))
	}
}

func TestCompressDecompress_LargeData(t *testing.T) {
	// Create a large, compressible input.
	data := make([]byte, 100_000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	compressed, err := Compress(data)
	if err != nil {
		t.Fatalf("Compress large data failed: %v", err)
	}

	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress large data failed: %v", err)
	}

	if len(decompressed) != len(data) {
		t.Fatalf("round-trip length mismatch: got %d, want %d", len(decompressed), len(data))
	}
	for i := range data {
		if decompressed[i] != data[i] {
			t.Fatalf("round-trip mismatch at byte %d", i)
		}
	}
}

func TestCompressDecompress_ChineseCharacters(t *testing.T) {
	data := []byte("你好世界！这是一个中文压缩测试。日记内容：今天天气很好。")

	compressed, err := Compress(data)
	if err != nil {
		t.Fatalf("Compress Chinese text failed: %v", err)
	}

	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress Chinese text failed: %v", err)
	}

	if string(decompressed) != string(data) {
		t.Errorf("round-trip mismatch for Chinese text:\n  got:  %q\n  want: %q", decompressed, data)
	}
}

func TestDecompress_InvalidData(t *testing.T) {
	_, err := Decompress([]byte("this is not valid zlib data"))
	if err == nil {
		t.Fatal("expected error decompressing invalid data, got nil")
	}
}

// ---------------------------------------------------------------------------
// 2.3 Encryption Tests
// ---------------------------------------------------------------------------

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	// Generate two keypairs (Alice and Bob).
	alicePub, alicePriv := generateTestKeypair(t)
	bobPub, bobPriv := generateTestKeypair(t)

	aliceX25519Priv, _ := Ed25519PrivateToX25519(alicePriv)
	aliceX25519Pub, _ := Ed25519PublicToX25519(alicePub)
	bobX25519Priv, _ := Ed25519PrivateToX25519(bobPriv)
	bobX25519Pub, _ := Ed25519PublicToX25519(bobPub)

	plaintext := []byte("secret message from Alice to Bob")

	// Alice encrypts to Bob.
	ciphertext, err := Encrypt(plaintext, bobX25519Pub, aliceX25519Priv)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Verify overhead: nonce (24) + MAC (16) = 40 bytes.
	expectedLen := len(plaintext) + TotalOverhead
	if len(ciphertext) != expectedLen {
		t.Errorf("ciphertext length: got %d, want %d (plaintext %d + overhead %d)",
			len(ciphertext), expectedLen, len(plaintext), TotalOverhead)
	}

	// Bob decrypts from Alice.
	decrypted, err := Decrypt(ciphertext, aliceX25519Pub, bobX25519Priv)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("round-trip mismatch:\n  got:  %q\n  want: %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_SelfEncrypt(t *testing.T) {
	// Encrypt to self (same key for sender and recipient).
	pub, priv := generateTestKeypair(t)
	x25519Priv, _ := Ed25519PrivateToX25519(priv)
	x25519Pub, _ := Ed25519PublicToX25519(pub)

	plaintext := []byte("self-encrypted message")

	ciphertext, err := Encrypt(plaintext, x25519Pub, x25519Priv)
	if err != nil {
		t.Fatalf("Encrypt (self) failed: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, x25519Pub, x25519Priv)
	if err != nil {
		t.Fatalf("Decrypt (self) failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("self-encrypt round-trip mismatch:\n  got:  %q\n  want: %q", decrypted, plaintext)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	alicePub, alicePriv := generateTestKeypair(t)
	_, bobPriv := generateTestKeypair(t)
	_, evePriv := generateTestKeypair(t)

	aliceX25519Priv, _ := Ed25519PrivateToX25519(alicePriv)
	aliceX25519Pub, _ := Ed25519PublicToX25519(alicePub)
	bobX25519Priv, _ := Ed25519PrivateToX25519(bobPriv)
	eveX25519Priv, _ := Ed25519PrivateToX25519(evePriv)

	// Generate Bob's X25519 public key from his private key for Encrypt.
	bobX25519Pub, _ := curve25519.X25519(bobX25519Priv, curve25519.Basepoint)

	plaintext := []byte("secret message")

	// Alice encrypts to Bob.
	ciphertext, err := Encrypt(plaintext, bobX25519Pub, aliceX25519Priv)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Eve tries to decrypt (wrong private key).
	_, err = Decrypt(ciphertext, aliceX25519Pub, eveX25519Priv)
	if err == nil {
		t.Fatal("expected decryption to fail with wrong key, but it succeeded")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	pub, priv := generateTestKeypair(t)
	x25519Priv, _ := Ed25519PrivateToX25519(priv)
	x25519Pub, _ := Ed25519PublicToX25519(pub)

	plaintext := []byte("tamper test message")
	ciphertext, err := Encrypt(plaintext, x25519Pub, x25519Priv)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with the encrypted payload (after the nonce).
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[NonceSize+5] ^= 0xFF

	_, err = Decrypt(tampered, x25519Pub, x25519Priv)
	if err == nil {
		t.Fatal("expected decryption to fail with tampered ciphertext, but it succeeded")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	_, err := Decrypt(make([]byte, TotalOverhead-1), make([]byte, KeySize), make([]byte, KeySize))
	if err == nil {
		t.Fatal("expected error for ciphertext shorter than overhead, got nil")
	}
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	_, err := Encrypt([]byte("test"), make([]byte, 16), make([]byte, KeySize))
	if err == nil {
		t.Fatal("expected error for short public key")
	}

	_, err = Encrypt([]byte("test"), make([]byte, KeySize), make([]byte, 16))
	if err == nil {
		t.Fatal("expected error for short private key")
	}
}

func TestEncrypt_RandomNonce(t *testing.T) {
	pub, priv := generateTestKeypair(t)
	x25519Priv, _ := Ed25519PrivateToX25519(priv)
	x25519Pub, _ := Ed25519PublicToX25519(pub)

	plaintext := []byte("same plaintext")

	ct1, _ := Encrypt(plaintext, x25519Pub, x25519Priv)
	ct2, _ := Encrypt(plaintext, x25519Pub, x25519Priv)

	// The nonces (first 24 bytes) should differ.
	nonce1 := ct1[:NonceSize]
	nonce2 := ct2[:NonceSize]
	same := true
	for i := range nonce1 {
		if nonce1[i] != nonce2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("two encryptions of the same plaintext produced identical nonces")
	}
}

// ---------------------------------------------------------------------------
// 2.4 Memo Pipeline Tests
// ---------------------------------------------------------------------------

func TestEncodeMemoDecodeMemo_English(t *testing.T) {
	pub, priv := generateTestKeypair(t)

	plaintext := "Today I went for a walk in the park."

	memo, err := EncodeMemo(plaintext, pub, priv)
	if err != nil {
		t.Fatalf("EncodeMemo failed: %v", err)
	}

	if !strings.HasPrefix(memo, MemoPrefix) {
		t.Errorf("memo should start with %q, got: %q", MemoPrefix, memo[:10])
	}

	decoded, err := DecodeMemo(memo, pub, priv)
	if err != nil {
		t.Fatalf("DecodeMemo failed: %v", err)
	}

	if decoded != plaintext {
		t.Errorf("round-trip mismatch:\n  got:  %q\n  want: %q", decoded, plaintext)
	}
}

func TestEncodeMemoDecodeMemo_Chinese(t *testing.T) {
	pub, priv := generateTestKeypair(t)

	plaintext := "今天天气很好，我去公园散步了。看到了美丽的花朵。"

	memo, err := EncodeMemo(plaintext, pub, priv)
	if err != nil {
		t.Fatalf("EncodeMemo (Chinese) failed: %v", err)
	}

	decoded, err := DecodeMemo(memo, pub, priv)
	if err != nil {
		t.Fatalf("DecodeMemo (Chinese) failed: %v", err)
	}

	if decoded != plaintext {
		t.Errorf("Chinese round-trip mismatch:\n  got:  %q\n  want: %q", decoded, plaintext)
	}
}

func TestEncodeMemoDecodeMemo_Mixed(t *testing.T) {
	pub, priv := generateTestKeypair(t)

	plaintext := "Hello世界! 2024年1月1日: New Year's Day 新年快乐！🎉"

	memo, err := EncodeMemo(plaintext, pub, priv)
	if err != nil {
		t.Fatalf("EncodeMemo (mixed) failed: %v", err)
	}

	decoded, err := DecodeMemo(memo, pub, priv)
	if err != nil {
		t.Fatalf("DecodeMemo (mixed) failed: %v", err)
	}

	if decoded != plaintext {
		t.Errorf("mixed round-trip mismatch:\n  got:  %q\n  want: %q", decoded, plaintext)
	}
}

func TestEncodeMemoDecodeMemo_TwoParties(t *testing.T) {
	// Alice sends to Bob.
	alicePub, alicePriv := generateTestKeypair(t)
	bobPub, bobPriv := generateTestKeypair(t)

	plaintext := "Secret diary entry from Alice"

	memo, err := EncodeMemo(plaintext, bobPub, alicePriv)
	if err != nil {
		t.Fatalf("EncodeMemo failed: %v", err)
	}

	decoded, err := DecodeMemo(memo, alicePub, bobPriv)
	if err != nil {
		t.Fatalf("DecodeMemo failed: %v", err)
	}

	if decoded != plaintext {
		t.Errorf("two-party round-trip mismatch:\n  got:  %q\n  want: %q", decoded, plaintext)
	}
}

func TestDecodeMemo_MissingPrefix(t *testing.T) {
	_, err := DecodeMemo("no-prefix-here", ed25519.PublicKey(make([]byte, 32)), ed25519.PrivateKey(make([]byte, 64)))
	if err == nil {
		t.Fatal("expected error for missing HD: prefix, got nil")
	}
	if !strings.Contains(err.Error(), "HD:") {
		t.Errorf("error should mention HD: prefix, got: %v", err)
	}
}

func TestDecodeMemo_InvalidBase64(t *testing.T) {
	pub, priv := generateTestKeypair(t)
	_, err := DecodeMemo("HD:!!!invalid-base64!!!", pub, priv)
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestEncodeMemo_ExceedsMaxLength(t *testing.T) {
	pub, priv := generateTestKeypair(t)

	// Create a long plaintext that will exceed 512 bytes after compression +
	// encryption + base64. Use random bytes to defeat compression.
	// Work backwards: 512 - 3 (prefix) = 509 bytes of base64.
	// 509 / 4 * 3 ~= 381 bytes of binary. 381 - 40 (overhead) = 341 bytes
	// of compressed data. So we need plaintext that compresses to > 341 bytes.
	// Random data won't compress at all, so 400 random bytes will do it.
	randomBytes := make([]byte, 400)
	_, _ = rand.Read(randomBytes)
	// Convert to a hex-ish string to ensure it stays as valid UTF-8.
	longText := base64.StdEncoding.EncodeToString(randomBytes)

	_, err := EncodeMemo(longText, pub, priv)
	if err == nil {
		t.Fatal("expected error for memo exceeding 512 bytes, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds max length") {
		t.Errorf("error should mention exceeds max length, got: %v", err)
	}
}

func TestEncodeMemo_OverheadCalculation(t *testing.T) {
	pub, priv := generateTestKeypair(t)

	plaintext := "test"

	memo, err := EncodeMemo(plaintext, pub, priv)
	if err != nil {
		t.Fatalf("EncodeMemo failed: %v", err)
	}

	// Strip prefix and decode base64 to get the raw encrypted payload.
	b64 := memo[len(MemoPrefix):]
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	// Compress the plaintext to determine the compressed size.
	compressed, err := Compress([]byte(plaintext))
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// The raw encrypted payload should be: compressed_size + 40 (nonce + MAC).
	expectedRawLen := len(compressed) + TotalOverhead
	if len(raw) != expectedRawLen {
		t.Errorf("raw encrypted length: got %d, want %d (compressed %d + overhead %d)",
			len(raw), expectedRawLen, len(compressed), TotalOverhead)
	}
}

func TestEncodeMemo_EmptyString(t *testing.T) {
	pub, priv := generateTestKeypair(t)

	memo, err := EncodeMemo("", pub, priv)
	if err != nil {
		t.Fatalf("EncodeMemo (empty) failed: %v", err)
	}

	decoded, err := DecodeMemo(memo, pub, priv)
	if err != nil {
		t.Fatalf("DecodeMemo (empty) failed: %v", err)
	}

	if decoded != "" {
		t.Errorf("expected empty string, got %q", decoded)
	}
}

func TestDecodeMemo_WrongKey(t *testing.T) {
	alicePub, alicePriv := generateTestKeypair(t)
	_, evePriv := generateTestKeypair(t)

	memo, err := EncodeMemo("secret", alicePub, alicePriv)
	if err != nil {
		t.Fatalf("EncodeMemo failed: %v", err)
	}

	// Eve tries to decode with her own key.
	_, err = DecodeMemo(memo, alicePub, evePriv)
	if err == nil {
		t.Fatal("expected decryption failure with wrong key, got nil")
	}
}

func TestMemoPrefix(t *testing.T) {
	if MemoPrefix != "HD:" {
		t.Errorf("MemoPrefix should be 'HD:', got %q", MemoPrefix)
	}
	if len(MemoPrefix) != 3 {
		t.Errorf("MemoPrefix length should be 3, got %d", len(MemoPrefix))
	}
}

func TestConstants(t *testing.T) {
	if NonceSize != 24 {
		t.Errorf("NonceSize should be 24, got %d", NonceSize)
	}
	if MACSize != 16 {
		t.Errorf("MACSize should be 16, got %d", MACSize)
	}
	if TotalOverhead != 40 {
		t.Errorf("TotalOverhead should be 40, got %d", TotalOverhead)
	}
	if MaxMemoLen != 512 {
		t.Errorf("MaxMemoLen should be 512, got %d", MaxMemoLen)
	}
}
