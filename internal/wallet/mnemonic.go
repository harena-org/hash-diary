package wallet

import (
	"crypto/ed25519"
	"fmt"
	"strings"

	slip10 "github.com/anyproto/go-slip10"
	"github.com/tyler-smith/go-bip39"
)

// SolanaDerivationPath is the standard BIP-44 derivation path for Solana.
const SolanaDerivationPath = "m/44'/501'/0'/0'"

// ImportFromMnemonic derives a Solana keypair from a BIP-39 mnemonic phrase
// using the standard Solana derivation path (m/44'/501'/0'/0').
// The passphrase is the optional BIP-39 passphrase (not the wallet encryption password).
func ImportFromMnemonic(mnemonic, passphrase string) (*Keypair, error) {
	mnemonic = normalizeMnemonic(mnemonic)

	words := strings.Fields(mnemonic)
	if len(words) != 12 && len(words) != 24 {
		return nil, fmt.Errorf("wallet: 助记词必须是 12 或 24 个单词，当前为 %d 个", len(words))
	}

	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("wallet: 无效的助记词，请检查拼写和词序")
	}

	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, passphrase)
	if err != nil {
		return nil, fmt.Errorf("wallet: 生成种子失败: %w", err)
	}

	node, err := slip10.DeriveForPath(SolanaDerivationPath, seed)
	if err != nil {
		return nil, fmt.Errorf("wallet: 密钥派生失败: %w", err)
	}

	pub, priv := node.Keypair()

	return &Keypair{
		PrivateKey: ed25519.PrivateKey(priv),
		PublicKey:  ed25519.PublicKey(pub),
	}, nil
}

// normalizeMnemonic trims and collapses whitespace in a mnemonic phrase.
func normalizeMnemonic(mnemonic string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(mnemonic)), " ")
}
