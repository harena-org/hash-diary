package wallet

import (
	"path/filepath"
	"strings"
	"testing"
)

// Well-known 12-word test mnemonic (BIP-39 "abandon" mnemonic).
const testMnemonic12 = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

// Well-known 24-word test mnemonic.
const testMnemonic24 = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"

func TestImportFromMnemonic_KnownDerivation(t *testing.T) {
	kp, err := ImportFromMnemonic(testMnemonic12, "")
	if err != nil {
		t.Fatalf("ImportFromMnemonic() error: %v", err)
	}

	// Solana derivation path m/44'/501'/0'/0' with the "abandon...about" mnemonic
	// produces a deterministic address. Hardcode it for regression detection.
	const wantAddr = "HAgk14JpMQLgt6rVgv7cBQFJWFto5Dqxi472uT3DKpqk"
	addr := kp.Address()
	if addr != wantAddr {
		t.Errorf("address = %q, want %q", addr, wantAddr)
	}

	// Verify determinism: calling again should produce the same address.
	kp2, err := ImportFromMnemonic(testMnemonic12, "")
	if err != nil {
		t.Fatalf("second ImportFromMnemonic() error: %v", err)
	}
	if kp2.Address() != addr {
		t.Errorf("determinism failed: first=%q, second=%q", addr, kp2.Address())
	}

	t.Logf("12-word mnemonic derived address: %s", addr)
}

func TestImportFromMnemonic_24Words(t *testing.T) {
	kp, err := ImportFromMnemonic(testMnemonic24, "")
	if err != nil {
		t.Fatalf("ImportFromMnemonic() error: %v", err)
	}

	addr := kp.Address()
	if addr == "" {
		t.Fatal("expected non-empty address")
	}

	// 24-word mnemonic should produce a different address than 12-word.
	kp12, err := ImportFromMnemonic(testMnemonic12, "")
	if err != nil {
		t.Fatalf("ImportFromMnemonic(12-word) error: %v", err)
	}
	if addr == kp12.Address() {
		t.Error("24-word and 12-word mnemonics should produce different addresses")
	}

	t.Logf("24-word mnemonic derived address: %s", addr)
}

func TestImportFromMnemonic_Determinism(t *testing.T) {
	// Same mnemonic should always produce the same keypair.
	addresses := make([]string, 5)
	for i := range addresses {
		kp, err := ImportFromMnemonic(testMnemonic12, "")
		if err != nil {
			t.Fatalf("ImportFromMnemonic() iteration %d error: %v", i, err)
		}
		addresses[i] = kp.Address()
	}

	for i := 1; i < len(addresses); i++ {
		if addresses[i] != addresses[0] {
			t.Errorf("determinism check failed at iteration %d: %q != %q", i, addresses[i], addresses[0])
		}
	}
}

func TestImportFromMnemonic_PassphraseChangesAddress(t *testing.T) {
	kpNoPass, err := ImportFromMnemonic(testMnemonic12, "")
	if err != nil {
		t.Fatalf("ImportFromMnemonic(no passphrase) error: %v", err)
	}

	kpWithPass, err := ImportFromMnemonic(testMnemonic12, "my-secret-passphrase")
	if err != nil {
		t.Fatalf("ImportFromMnemonic(with passphrase) error: %v", err)
	}

	if kpNoPass.Address() == kpWithPass.Address() {
		t.Error("different passphrases should produce different addresses")
	}
}

func TestImportFromMnemonic_InvalidMnemonic(t *testing.T) {
	tests := []struct {
		name     string
		mnemonic string
		wantErr  string
	}{
		{
			name:     "empty",
			mnemonic: "",
			wantErr:  "助记词必须是 12 或 24 个单词",
		},
		{
			name:     "random words",
			mnemonic: "hello world foo bar baz qux quux corge grault garply waldo fred",
			wantErr:  "无效的助记词",
		},
		{
			name:     "wrong word count (11 words)",
			mnemonic: "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon",
			wantErr:  "助记词必须是 12 或 24 个单词",
		},
		{
			name:     "wrong word count (13 words)",
			mnemonic: "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about zoo",
			wantErr:  "助记词必须是 12 或 24 个单词",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ImportFromMnemonic(tt.mnemonic, "")
			if err == nil {
				t.Fatal("expected error for invalid mnemonic")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNormalizeMnemonic(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already normalized",
			input: "abandon abandon about",
			want:  "abandon abandon about",
		},
		{
			name:  "leading/trailing whitespace",
			input: "  abandon abandon about  ",
			want:  "abandon abandon about",
		},
		{
			name:  "multiple spaces between words",
			input: "abandon   abandon   about",
			want:  "abandon abandon about",
		},
		{
			name:  "tabs and newlines",
			input: "abandon\tabandon\nabout",
			want:  "abandon abandon about",
		},
		{
			name:  "mixed whitespace",
			input: "\t  abandon  \n  abandon   about \t ",
			want:  "abandon abandon about",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeMnemonic(tt.input)
			if got != tt.want {
				t.Errorf("normalizeMnemonic(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestImportFromMnemonic_SaveLoadRoundTrip(t *testing.T) {
	kp, err := ImportFromMnemonic(testMnemonic12, "")
	if err != nil {
		t.Fatalf("ImportFromMnemonic() error: %v", err)
	}

	dir := t.TempDir()

	// Plaintext round-trip.
	plainPath := filepath.Join(dir, "plain.json")
	if err := SavePlaintext(kp, plainPath); err != nil {
		t.Fatalf("SavePlaintext() error: %v", err)
	}
	loaded, err := LoadPlaintext(plainPath)
	if err != nil {
		t.Fatalf("LoadPlaintext() error: %v", err)
	}
	if loaded.Address() != kp.Address() {
		t.Errorf("plaintext round-trip: got %q, want %q", loaded.Address(), kp.Address())
	}

	// Encrypted round-trip.
	encPath := filepath.Join(dir, "encrypted.json")
	password := "test-password"
	if err := SaveEncrypted(kp, encPath, password); err != nil {
		t.Fatalf("SaveEncrypted() error: %v", err)
	}
	loadedEnc, err := LoadEncrypted(encPath, password)
	if err != nil {
		t.Fatalf("LoadEncrypted() error: %v", err)
	}
	if loadedEnc.Address() != kp.Address() {
		t.Errorf("encrypted round-trip: got %q, want %q", loadedEnc.Address(), kp.Address())
	}
}
