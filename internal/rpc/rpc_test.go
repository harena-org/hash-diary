package rpc

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	solanarpc "github.com/gagliardetto/solana-go/rpc"
)

// --- Mock RPC Client ---

type mockRPCClient struct {
	getBalanceFn                    func(ctx context.Context, pubKey solana.PublicKey, commitment solanarpc.CommitmentType) (*solanarpc.GetBalanceResult, error)
	requestAirdropFn                func(ctx context.Context, account solana.PublicKey, lamports uint64, commitment solanarpc.CommitmentType) (solana.Signature, error)
	sendTransactionWithOptsFn       func(ctx context.Context, tx *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error)
	getSignatureStatusesFn          func(ctx context.Context, searchHistory bool, sigs ...solana.Signature) (*solanarpc.GetSignatureStatusesResult, error)
	getSignaturesForAddressOptsFn   func(ctx context.Context, account solana.PublicKey, opts *solanarpc.GetSignaturesForAddressOpts) ([]*solanarpc.TransactionSignature, error)
	getTransactionFn                func(ctx context.Context, txSig solana.Signature, opts *solanarpc.GetTransactionOpts) (*solanarpc.GetTransactionResult, error)
}

func (m *mockRPCClient) GetBalance(ctx context.Context, pubKey solana.PublicKey, commitment solanarpc.CommitmentType) (*solanarpc.GetBalanceResult, error) {
	if m.getBalanceFn != nil {
		return m.getBalanceFn(ctx, pubKey, commitment)
	}
	return &solanarpc.GetBalanceResult{Value: 0}, nil
}

func (m *mockRPCClient) RequestAirdrop(ctx context.Context, account solana.PublicKey, lamports uint64, commitment solanarpc.CommitmentType) (solana.Signature, error) {
	if m.requestAirdropFn != nil {
		return m.requestAirdropFn(ctx, account, lamports, commitment)
	}
	return solana.Signature{}, nil
}

func (m *mockRPCClient) SendTransactionWithOpts(ctx context.Context, tx *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error) {
	if m.sendTransactionWithOptsFn != nil {
		return m.sendTransactionWithOptsFn(ctx, tx, opts)
	}
	return solana.Signature{}, nil
}

func (m *mockRPCClient) GetSignatureStatuses(ctx context.Context, searchHistory bool, sigs ...solana.Signature) (*solanarpc.GetSignatureStatusesResult, error) {
	if m.getSignatureStatusesFn != nil {
		return m.getSignatureStatusesFn(ctx, searchHistory, sigs...)
	}
	return nil, nil
}

func (m *mockRPCClient) GetSignaturesForAddressWithOpts(ctx context.Context, account solana.PublicKey, opts *solanarpc.GetSignaturesForAddressOpts) ([]*solanarpc.TransactionSignature, error) {
	if m.getSignaturesForAddressOptsFn != nil {
		return m.getSignaturesForAddressOptsFn(ctx, account, opts)
	}
	return nil, nil
}

func (m *mockRPCClient) GetTransaction(ctx context.Context, txSig solana.Signature, opts *solanarpc.GetTransactionOpts) (*solanarpc.GetTransactionResult, error) {
	if m.getTransactionFn != nil {
		return m.getTransactionFn(ctx, txSig, opts)
	}
	return nil, nil
}

// --- Tests ---

func TestDetectNetwork(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "devnet standard",
			endpoint: "https://api.devnet.solana.com",
			want:     NetworkDevnet,
		},
		{
			name:     "testnet standard",
			endpoint: "https://api.testnet.solana.com",
			want:     NetworkTestnet,
		},
		{
			name:     "mainnet-beta standard",
			endpoint: "https://api.mainnet-beta.solana.com",
			want:     NetworkMainnetBeta,
		},
		{
			name:     "mainnet in URL",
			endpoint: "https://mainnet.helius-rpc.com/?api-key=xxx",
			want:     NetworkMainnetBeta,
		},
		{
			name:     "devnet case insensitive",
			endpoint: "https://api.DEVNET.solana.com",
			want:     NetworkDevnet,
		},
		{
			name:     "custom endpoint",
			endpoint: "https://my-rpc.example.com",
			want:     NetworkCustom,
		},
		{
			name:     "localhost",
			endpoint: "http://127.0.0.1:8899",
			want:     NetworkCustom,
		},
		{
			name:     "empty string",
			endpoint: "",
			want:     NetworkCustom,
		},
		{
			name:     "testnet in path",
			endpoint: "https://rpc.example.com/testnet",
			want:     NetworkTestnet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectNetwork(tt.endpoint)
			if got != tt.want {
				t.Errorf("DetectNetwork(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestNewClient_DefaultEndpoint(t *testing.T) {
	c := NewClient("")
	if c.Endpoint() != DefaultEndpoint {
		t.Errorf("expected default endpoint %q, got %q", DefaultEndpoint, c.Endpoint())
	}
	if c.Network() != NetworkDevnet {
		t.Errorf("expected network %q, got %q", NetworkDevnet, c.Network())
	}
}

func TestNewClient_CustomEndpoint(t *testing.T) {
	ep := "https://api.mainnet-beta.solana.com"
	c := NewClient(ep)
	if c.Endpoint() != ep {
		t.Errorf("expected endpoint %q, got %q", ep, c.Endpoint())
	}
	if c.Network() != NetworkMainnetBeta {
		t.Errorf("expected network %q, got %q", NetworkMainnetBeta, c.Network())
	}
}

func TestGetBalance_Success(t *testing.T) {
	mock := &mockRPCClient{
		getBalanceFn: func(ctx context.Context, pubKey solana.PublicKey, commitment solanarpc.CommitmentType) (*solanarpc.GetBalanceResult, error) {
			return &solanarpc.GetBalanceResult{Value: 1_500_000_000}, nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	balance, err := c.GetBalance(context.Background(), "11111111111111111111111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance != 1.5 {
		t.Errorf("expected balance 1.5, got %f", balance)
	}
}

func TestGetBalance_InvalidAddress(t *testing.T) {
	mock := &mockRPCClient{}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, err := c.GetBalance(context.Background(), "invalid-address")
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

func TestGetBalance_RPCError(t *testing.T) {
	mock := &mockRPCClient{
		getBalanceFn: func(ctx context.Context, pubKey solana.PublicKey, commitment solanarpc.CommitmentType) (*solanarpc.GetBalanceResult, error) {
			return nil, errors.New("connection refused")
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, err := c.GetBalance(context.Background(), "11111111111111111111111111111111")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); !contains(got, "网络错误") {
		t.Errorf("expected network error message, got: %s", got)
	}
}

func TestRequestAirdrop_Success(t *testing.T) {
	expectedSig := solana.MustSignatureFromBase58("5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW")
	mock := &mockRPCClient{
		requestAirdropFn: func(ctx context.Context, account solana.PublicKey, lamports uint64, commitment solanarpc.CommitmentType) (solana.Signature, error) {
			if lamports != 2_000_000_000 {
				t.Errorf("expected 2000000000 lamports, got %d", lamports)
			}
			return expectedSig, nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	sig, err := c.RequestAirdrop(context.Background(), "11111111111111111111111111111111", 2.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig != expectedSig.String() {
		t.Errorf("expected sig %q, got %q", expectedSig.String(), sig)
	}
}

func TestRequestAirdrop_RejectMainnet(t *testing.T) {
	mock := &mockRPCClient{}
	c := newClientWithRPC("https://api.mainnet-beta.solana.com", mock)

	_, err := c.RequestAirdrop(context.Background(), "11111111111111111111111111111111", 1.0)
	if !errors.Is(err, ErrMainnetAirdrop) {
		t.Errorf("expected ErrMainnetAirdrop, got: %v", err)
	}
}

func TestRequestAirdrop_InvalidAddress(t *testing.T) {
	mock := &mockRPCClient{}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, err := c.RequestAirdrop(context.Background(), "bad-addr", 1.0)
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

func TestSendTransaction_Success(t *testing.T) {
	expectedSig := solana.MustSignatureFromBase58("5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW")
	mock := &mockRPCClient{
		sendTransactionWithOptsFn: func(ctx context.Context, tx *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error) {
			return expectedSig, nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	sig, err := c.SendTransaction(context.Background(), &solana.Transaction{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig != expectedSig.String() {
		t.Errorf("expected sig %q, got %q", expectedSig.String(), sig)
	}
}

func TestSendTransaction_NilTransaction(t *testing.T) {
	mock := &mockRPCClient{}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, err := c.SendTransaction(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil transaction")
	}
}

func TestSendTransaction_RetryLogic(t *testing.T) {
	// Override sleep to be instant for tests.
	origSleep := sleepFunc
	sleepFunc = func(d time.Duration) {} // no-op
	defer func() { sleepFunc = origSleep }()

	var attempts int32
	mock := &mockRPCClient{
		sendTransactionWithOptsFn: func(ctx context.Context, tx *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error) {
			count := atomic.AddInt32(&attempts, 1)
			if count <= 2 {
				return solana.Signature{}, errors.New("temporary error")
			}
			return solana.MustSignatureFromBase58("5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW"), nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	sig, err := c.SendTransaction(context.Background(), &solana.Transaction{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig == "" {
		t.Error("expected non-empty signature")
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestSendTransaction_AllRetriesFail(t *testing.T) {
	origSleep := sleepFunc
	sleepFunc = func(d time.Duration) {}
	defer func() { sleepFunc = origSleep }()

	var attempts int32
	mock := &mockRPCClient{
		sendTransactionWithOptsFn: func(ctx context.Context, tx *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error) {
			atomic.AddInt32(&attempts, 1)
			return solana.Signature{}, errors.New("persistent error")
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, err := c.SendTransaction(context.Background(), &solana.Transaction{})
	if err == nil {
		t.Fatal("expected error after all retries fail")
	}
	// 1 initial + 3 retries = 4 total attempts
	if atomic.LoadInt32(&attempts) != 4 {
		t.Errorf("expected 4 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestSendTransaction_RetryBackoffDurations(t *testing.T) {
	origSleep := sleepFunc
	var sleepDurations []time.Duration
	sleepFunc = func(d time.Duration) {
		sleepDurations = append(sleepDurations, d)
	}
	defer func() { sleepFunc = origSleep }()

	mock := &mockRPCClient{
		sendTransactionWithOptsFn: func(ctx context.Context, tx *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error) {
			return solana.Signature{}, errors.New("error")
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, _ = c.SendTransaction(context.Background(), &solana.Transaction{})

	expected := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}
	if len(sleepDurations) != len(expected) {
		t.Fatalf("expected %d sleeps, got %d", len(expected), len(sleepDurations))
	}
	for i, d := range expected {
		if sleepDurations[i] != d {
			t.Errorf("sleep[%d]: expected %v, got %v", i, d, sleepDurations[i])
		}
	}
}

func TestSendTransaction_ContextCancelled(t *testing.T) {
	origSleep := sleepFunc
	sleepFunc = func(d time.Duration) {}
	defer func() { sleepFunc = origSleep }()

	ctx, cancel := context.WithCancel(context.Background())
	var attempts int32
	mock := &mockRPCClient{
		sendTransactionWithOptsFn: func(ctx context.Context, tx *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error) {
			count := atomic.AddInt32(&attempts, 1)
			if count == 1 {
				cancel()
			}
			return solana.Signature{}, errors.New("error")
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, err := c.SendTransaction(ctx, &solana.Transaction{})
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
	if got := err.Error(); !contains(got, "cancelled") {
		t.Errorf("expected cancellation error, got: %s", got)
	}
}

func TestConfirmTransaction_Success(t *testing.T) {
	sigStr := "5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW"
	var pollCount int32
	mock := &mockRPCClient{
		getSignatureStatusesFn: func(ctx context.Context, searchHistory bool, sigs ...solana.Signature) (*solanarpc.GetSignatureStatusesResult, error) {
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				// First poll: not yet confirmed
				return &solanarpc.GetSignatureStatusesResult{
					Value: []*solanarpc.SignatureStatusesResult{
						{ConfirmationStatus: solanarpc.ConfirmationStatusProcessed},
					},
				}, nil
			}
			// Second poll: confirmed
			return &solanarpc.GetSignatureStatusesResult{
				Value: []*solanarpc.SignatureStatusesResult{
					{ConfirmationStatus: solanarpc.ConfirmationStatusConfirmed},
				},
			}, nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	got, err := c.ConfirmTransaction(context.Background(), sigStr, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != sigStr {
		t.Errorf("expected signature %q, got %q", sigStr, got)
	}
}

func TestConfirmTransaction_Finalized(t *testing.T) {
	sigStr := "5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW"
	mock := &mockRPCClient{
		getSignatureStatusesFn: func(ctx context.Context, searchHistory bool, sigs ...solana.Signature) (*solanarpc.GetSignatureStatusesResult, error) {
			return &solanarpc.GetSignatureStatusesResult{
				Value: []*solanarpc.SignatureStatusesResult{
					{ConfirmationStatus: solanarpc.ConfirmationStatusFinalized},
				},
			}, nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	got, err := c.ConfirmTransaction(context.Background(), sigStr, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != sigStr {
		t.Errorf("expected signature %q, got %q", sigStr, got)
	}
}

func TestConfirmTransaction_Timeout(t *testing.T) {
	sigStr := "5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW"
	mock := &mockRPCClient{
		getSignatureStatusesFn: func(ctx context.Context, searchHistory bool, sigs ...solana.Signature) (*solanarpc.GetSignatureStatusesResult, error) {
			// Always return "processed" -- never confirmed.
			return &solanarpc.GetSignatureStatusesResult{
				Value: []*solanarpc.SignatureStatusesResult{
					{ConfirmationStatus: solanarpc.ConfirmationStatusProcessed},
				},
			}, nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	// Use a very short timeout so the test finishes quickly.
	got, err := c.ConfirmTransaction(context.Background(), sigStr, 100*time.Millisecond)
	if !errors.Is(err, ErrConfirmTimeout) {
		t.Errorf("expected ErrConfirmTimeout, got: %v", err)
	}
	// Should still return the signature even on timeout.
	if got != sigStr {
		t.Errorf("expected signature %q on timeout, got %q", sigStr, got)
	}
}

func TestConfirmTransaction_InvalidSignature(t *testing.T) {
	mock := &mockRPCClient{}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, err := c.ConfirmTransaction(context.Background(), "bad-sig", 10*time.Second)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestSendAndConfirmTransaction_Success(t *testing.T) {
	expectedSig := solana.MustSignatureFromBase58("5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW")
	mock := &mockRPCClient{
		sendTransactionWithOptsFn: func(ctx context.Context, tx *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error) {
			return expectedSig, nil
		},
		getSignatureStatusesFn: func(ctx context.Context, searchHistory bool, sigs ...solana.Signature) (*solanarpc.GetSignatureStatusesResult, error) {
			return &solanarpc.GetSignatureStatusesResult{
				Value: []*solanarpc.SignatureStatusesResult{
					{ConfirmationStatus: solanarpc.ConfirmationStatusConfirmed},
				},
			}, nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	sig, status, err := c.SendAndConfirmTransaction(context.Background(), &solana.Transaction{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig != expectedSig.String() {
		t.Errorf("expected sig %q, got %q", expectedSig.String(), sig)
	}
	if status != StatusConfirmed {
		t.Errorf("expected status %q, got %q", StatusConfirmed, status)
	}
}

func TestSendAndConfirmTransaction_Timeout(t *testing.T) {
	expectedSig := solana.MustSignatureFromBase58("5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW")
	mock := &mockRPCClient{
		sendTransactionWithOptsFn: func(ctx context.Context, tx *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error) {
			return expectedSig, nil
		},
		getSignatureStatusesFn: func(ctx context.Context, searchHistory bool, sigs ...solana.Signature) (*solanarpc.GetSignatureStatusesResult, error) {
			return &solanarpc.GetSignatureStatusesResult{
				Value: []*solanarpc.SignatureStatusesResult{
					{ConfirmationStatus: solanarpc.ConfirmationStatusProcessed},
				},
			}, nil
		},
	}

	// Override the default confirm timeout for this test only.
	// We do this by overriding the function behavior rather than the constant.
	// But since SendAndConfirmTransaction uses DefaultConfirmTimeout,
	// we need a short-lived context to make it timeout quickly.
	// Actually, we use ConfirmTransaction's timeout parameter.
	// SendAndConfirmTransaction uses DefaultConfirmTimeout (60s) which is too long.
	// Let's just test using context cancellation instead.
	// Actually the timeout is handled within ConfirmTransaction, we cannot shorten it
	// without modifying the code. Let's use a context with timeout instead.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	c := newClientWithRPC(DefaultEndpoint, mock)
	sig, status, err := c.SendAndConfirmTransaction(ctx, &solana.Transaction{})

	// When context times out before the confirm timeout, it returns a context error.
	// The test verifies we get a non-empty sig and either timeout or context error.
	if sig == "" {
		t.Error("expected non-empty signature even on timeout")
	}
	// Either the confirm timed out or the context did.
	if status == StatusConfirmed {
		t.Error("did not expect confirmed status")
	}
	_ = err // err may or may not be set depending on which timeout fires first
}

func TestSendAndConfirmTransaction_SendFails(t *testing.T) {
	origSleep := sleepFunc
	sleepFunc = func(d time.Duration) {}
	defer func() { sleepFunc = origSleep }()

	mock := &mockRPCClient{
		sendTransactionWithOptsFn: func(ctx context.Context, tx *solana.Transaction, opts solanarpc.TransactionOpts) (solana.Signature, error) {
			return solana.Signature{}, errors.New("send failed")
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	sig, status, err := c.SendAndConfirmTransaction(context.Background(), &solana.Transaction{})
	if err == nil {
		t.Fatal("expected error when send fails")
	}
	if sig != "" {
		t.Errorf("expected empty sig, got %q", sig)
	}
	if status != "" {
		t.Errorf("expected empty status, got %q", status)
	}
}

func TestGetSignaturesForAddress_Success(t *testing.T) {
	bt := solana.UnixTimeSeconds(1700000000)
	sig := solana.MustSignatureFromBase58("5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW")
	mock := &mockRPCClient{
		getSignaturesForAddressOptsFn: func(ctx context.Context, account solana.PublicKey, opts *solanarpc.GetSignaturesForAddressOpts) ([]*solanarpc.TransactionSignature, error) {
			return []*solanarpc.TransactionSignature{
				{
					Signature: sig,
					BlockTime: &bt,
					Err:       nil,
				},
			}, nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	infos, err := c.GetSignaturesForAddress(context.Background(), "11111111111111111111111111111111", SignatureOpts{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 result, got %d", len(infos))
	}
	if infos[0].Signature != sig.String() {
		t.Errorf("expected sig %q, got %q", sig.String(), infos[0].Signature)
	}
	if infos[0].BlockTime == nil {
		t.Fatal("expected non-nil BlockTime")
	}
	if *infos[0].BlockTime != 1700000000 {
		t.Errorf("expected block time 1700000000, got %d", *infos[0].BlockTime)
	}
}

func TestGetSignaturesForAddress_InvalidAddress(t *testing.T) {
	mock := &mockRPCClient{}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, err := c.GetSignaturesForAddress(context.Background(), "bad-addr", SignatureOpts{})
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

func TestGetSignaturesForAddress_WithOpts(t *testing.T) {
	beforeSig := "5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW"
	mock := &mockRPCClient{
		getSignaturesForAddressOptsFn: func(ctx context.Context, account solana.PublicKey, opts *solanarpc.GetSignaturesForAddressOpts) ([]*solanarpc.TransactionSignature, error) {
			if opts.Limit == nil || *opts.Limit != 5 {
				t.Error("expected limit 5")
			}
			if opts.Before.IsZero() {
				t.Error("expected non-zero Before signature")
			}
			return nil, nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, err := c.GetSignaturesForAddress(context.Background(), "11111111111111111111111111111111", SignatureOpts{
		Limit:  5,
		Before: beforeSig,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTransaction_Success(t *testing.T) {
	bt := solana.UnixTimeSeconds(1700000000)
	mock := &mockRPCClient{
		getTransactionFn: func(ctx context.Context, txSig solana.Signature, opts *solanarpc.GetTransactionOpts) (*solanarpc.GetTransactionResult, error) {
			return &solanarpc.GetTransactionResult{
				Slot:      12345,
				BlockTime: &bt,
				Meta: &solanarpc.TransactionMeta{
					LogMessages: []string{
						"Program MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr invoke [1]",
						`Program log: Memo (len 11): "Hello World"`,
						"Program MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr consumed 12345 of 200000 compute units",
						"Program MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr success",
					},
				},
			}, nil
		},
	}
	c := newClientWithRPC(DefaultEndpoint, mock)

	sigStr := "5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW"
	result, err := c.GetTransaction(context.Background(), sigStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Slot != 12345 {
		t.Errorf("expected slot 12345, got %d", result.Slot)
	}
	if result.BlockTime == nil || *result.BlockTime != 1700000000 {
		t.Error("unexpected block time")
	}
	if len(result.LogMessages) != 4 {
		t.Errorf("expected 4 log messages, got %d", len(result.LogMessages))
	}
}

func TestGetTransaction_InvalidSignature(t *testing.T) {
	mock := &mockRPCClient{}
	c := newClientWithRPC(DefaultEndpoint, mock)

	_, err := c.GetTransaction(context.Background(), "bad-sig")
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestTransactionResult_ExtractMemoData(t *testing.T) {
	tests := []struct {
		name        string
		logMessages []string
		want        []string
	}{
		{
			name: "standard memo log with quotes",
			logMessages: []string{
				"Program MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr invoke [1]",
				`Program log: Memo (len 11): "Hello World"`,
				"Program MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr success",
			},
			want: []string{"Hello World"},
		},
		{
			name: "multiple memos with quotes",
			logMessages: []string{
				`Program log: Memo (len 5): "First"`,
				`Program log: Memo (len 6): "Second"`,
			},
			want: []string{"First", "Second"},
		},
		{
			name: "memo without quotes (backwards compat)",
			logMessages: []string{
				"Program log: Memo (len 5): First",
			},
			want: []string{"First"},
		},
		{
			name: "no memo",
			logMessages: []string{
				"Program 11111111111111111111111111111111 invoke [1]",
				"Transfer: lamports 1000000000",
			},
			want: nil,
		},
		{
			name:        "empty logs",
			logMessages: nil,
			want:        nil,
		},
		{
			name: "memo with HD prefix and quotes",
			logMessages: []string{
				`Program log: Memo (len 30): "HD:SGVsbG8gV29ybGQ="`,
			},
			want: []string{"HD:SGVsbG8gV29ybGQ="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TransactionResult{LogMessages: tt.logMessages}
			got := tr.ExtractMemoData()
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d memos, got %d", len(tt.want), len(got))
			}
			for i, m := range got {
				if m != tt.want[i] {
					t.Errorf("memo[%d]: expected %q, got %q", i, tt.want[i], m)
				}
			}
		})
	}
}

func TestTransactionResult_ExtractMemoData_NilReceiver(t *testing.T) {
	var tr *TransactionResult
	got := tr.ExtractMemoData()
	if got != nil {
		t.Errorf("expected nil for nil receiver, got %v", got)
	}
}

func TestWrapNetworkError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{
			name:    "connection refused",
			err:     errors.New("dial tcp: connection refused"),
			wantMsg: "网络错误",
		},
		{
			name:    "rate limit",
			err:     errors.New("429 Too Many Requests"),
			wantMsg: "RPC 请求频率受限",
		},
		{
			name:    "insufficient funds",
			err:     errors.New("insufficient funds for rent"),
			wantMsg: "余额不足",
		},
		{
			name:    "generic error",
			err:     errors.New("something went wrong"),
			wantMsg: "something went wrong",
		},
		{
			name:    "nil error",
			err:     nil,
			wantMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapNetworkError(tt.err)
			if tt.err == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if !contains(got.Error(), tt.wantMsg) {
				t.Errorf("expected error containing %q, got %q", tt.wantMsg, got.Error())
			}
		})
	}
}

// helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
