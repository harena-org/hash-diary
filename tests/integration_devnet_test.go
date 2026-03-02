//go:build integration

package tests

import (
	"testing"
)

// TestWriteReadRoundTrip_Devnet is a full end-to-end test that writes a diary
// entry to the Solana devnet and reads it back. It requires a live devnet
// connection and a funded wallet, making it slow and unreliable for CI.
//
// Run with: go test -tags integration -v ./tests/
func TestWriteReadRoundTrip_Devnet(t *testing.T) {
	t.Skip("requires live devnet connection and funded wallet - run manually with: go test -tags integration -v ./tests/")

	// Skeleton for future implementation:
	//
	// 1. Generate a new keypair
	// 2. Request airdrop on devnet (1 SOL)
	// 3. Wait for airdrop confirmation
	// 4. Write a diary entry via the write pipeline:
	//    a. EncodeMemo the text
	//    b. Build a self-transfer transaction with Memo instruction
	//    c. Sign and send the transaction
	//    d. Wait for confirmation
	// 5. Read the diary entry back:
	//    a. GetSignaturesForAddress
	//    b. GetTransaction for each signature
	//    c. Extract and decode memo data
	// 6. Verify the decoded content matches the original
	//
	// This test is inherently flaky due to network dependencies (devnet
	// availability, airdrop rate limits, confirmation times) and should
	// only be run manually or in a dedicated integration test pipeline.
}
