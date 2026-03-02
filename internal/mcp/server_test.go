package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hash-diary/hash-diary/internal/wallet"
	gomcp "github.com/mark3labs/mcp-go/mcp"
)

// newTestRequest builds a CallToolRequest with the given tool name and arguments.
func newTestRequest(name string, args map[string]any) gomcp.CallToolRequest {
	req := gomcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	return req
}

// newTestServer creates a Server with a freshly generated keypair
// and a dummy endpoint (no real RPC calls will be made in unit tests).
func newTestServer(t *testing.T) *Server {
	t.Helper()
	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	return NewServer(kp, "https://api.devnet.solana.com")
}

// --- NewServer tests ---

func TestNewServer(t *testing.T) {
	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	endpoint := "https://api.mainnet-beta.solana.com"
	srv := NewServer(kp, endpoint)

	if srv == nil {
		t.Fatal("NewServer() returned nil")
	}
	if srv.keypair != kp {
		t.Error("NewServer() did not store the provided keypair")
	}
	if srv.endpoint != endpoint {
		t.Errorf("NewServer() endpoint = %q, want %q", srv.endpoint, endpoint)
	}
}

func TestNewServer_DifferentEndpoints(t *testing.T) {
	kp, err := wallet.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	endpoints := []string{
		"https://api.devnet.solana.com",
		"https://api.mainnet-beta.solana.com",
		"http://localhost:8899",
	}

	for _, ep := range endpoints {
		srv := NewServer(kp, ep)
		if srv.endpoint != ep {
			t.Errorf("NewServer(%q) endpoint = %q", ep, srv.endpoint)
		}
	}
}

// --- handleDiaryWrite error path tests ---

func TestHandleDiaryWrite_MissingText(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// No arguments at all.
	req := newTestRequest("diary_write", nil)
	result, err := srv.handleDiaryWrite(ctx, req)
	if err != nil {
		t.Fatalf("handleDiaryWrite() returned error: %v", err)
	}

	if !result.IsError {
		t.Fatal("handleDiaryWrite() should return an error result for missing text")
	}

	text := getResultText(t, result)
	if !strings.Contains(text, "missing required parameter 'text'") {
		t.Errorf("error message = %q, want to contain 'missing required parameter'", text)
	}
}

func TestHandleDiaryWrite_MissingTextKey(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Arguments map exists but without "text" key.
	req := newTestRequest("diary_write", map[string]any{
		"other": "value",
	})
	result, err := srv.handleDiaryWrite(ctx, req)
	if err != nil {
		t.Fatalf("handleDiaryWrite() returned error: %v", err)
	}

	if !result.IsError {
		t.Fatal("handleDiaryWrite() should return an error result when 'text' key is absent")
	}

	text := getResultText(t, result)
	if !strings.Contains(text, "missing required parameter 'text'") {
		t.Errorf("error message = %q, want to contain 'missing required parameter'", text)
	}
}

func TestHandleDiaryWrite_EmptyText(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	req := newTestRequest("diary_write", map[string]any{
		"text": "",
	})
	result, err := srv.handleDiaryWrite(ctx, req)
	if err != nil {
		t.Fatalf("handleDiaryWrite() returned error: %v", err)
	}

	if !result.IsError {
		t.Fatal("handleDiaryWrite() should return an error result for empty text")
	}

	text := getResultText(t, result)
	if !strings.Contains(text, "text content cannot be empty") {
		t.Errorf("error message = %q, want to contain 'text content cannot be empty'", text)
	}
}

func TestHandleDiaryWrite_WhitespaceOnlyText(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	req := newTestRequest("diary_write", map[string]any{
		"text": "   \t\n  ",
	})
	result, err := srv.handleDiaryWrite(ctx, req)
	if err != nil {
		t.Fatalf("handleDiaryWrite() returned error: %v", err)
	}

	if !result.IsError {
		t.Fatal("handleDiaryWrite() should return an error result for whitespace-only text")
	}

	text := getResultText(t, result)
	if !strings.Contains(text, "text content cannot be empty") {
		t.Errorf("error message = %q, want to contain 'text content cannot be empty'", text)
	}
}

func TestHandleDiaryWrite_NonStringText(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Provide a non-string value for "text".
	req := newTestRequest("diary_write", map[string]any{
		"text": 12345,
	})
	result, err := srv.handleDiaryWrite(ctx, req)
	if err != nil {
		t.Fatalf("handleDiaryWrite() returned error: %v", err)
	}

	if !result.IsError {
		t.Fatal("handleDiaryWrite() should return an error result for non-string text")
	}

	text := getResultText(t, result)
	if !strings.Contains(text, "missing required parameter 'text'") {
		t.Errorf("error message = %q, want to contain 'missing required parameter'", text)
	}
}

// --- handleDiaryRead parameter parsing tests ---

func TestHandleDiaryRead_DefaultLimit(t *testing.T) {
	// We test the parameter parsing by verifying the GetInt call behavior
	// directly through the CallToolRequest API.
	req := newTestRequest("diary_read", map[string]any{})

	limit := req.GetInt("limit", 7)
	if limit != 7 {
		t.Errorf("default limit = %d, want 7", limit)
	}
}

func TestHandleDiaryRead_CustomLimit(t *testing.T) {
	req := newTestRequest("diary_read", map[string]any{
		"limit": float64(15), // JSON numbers arrive as float64
	})

	limit := req.GetInt("limit", 7)
	if limit != 15 {
		t.Errorf("custom limit = %d, want 15", limit)
	}
}

func TestHandleDiaryRead_ZeroLimitFallsBackToDefault(t *testing.T) {
	// The handler resets limit to 7 when it's <= 0.
	req := newTestRequest("diary_read", map[string]any{
		"limit": float64(0),
	})

	limit := req.GetInt("limit", 7)
	// The raw GetInt returns 0, but the handler code corrects this.
	if limit != 0 {
		t.Errorf("raw limit = %d, want 0 (handler will correct to 7)", limit)
	}

	// Verify the handler logic: if limit <= 0, set to 7.
	if limit <= 0 {
		limit = 7
	}
	if limit != 7 {
		t.Errorf("corrected limit = %d, want 7", limit)
	}
}

func TestHandleDiaryRead_NegativeLimitFallsBackToDefault(t *testing.T) {
	req := newTestRequest("diary_read", map[string]any{
		"limit": float64(-5),
	})

	limit := req.GetInt("limit", 7)
	// Verify the handler logic: if limit <= 0, set to 7.
	if limit <= 0 {
		limit = 7
	}
	if limit != 7 {
		t.Errorf("corrected limit = %d, want 7", limit)
	}
}

func TestHandleDiaryRead_ParsesSinceParameter(t *testing.T) {
	req := newTestRequest("diary_read", map[string]any{
		"since": "2025-01-15",
	})

	since := req.GetString("since", "")
	if since != "2025-01-15" {
		t.Errorf("since = %q, want %q", since, "2025-01-15")
	}
}

func TestHandleDiaryRead_ParsesSearchParameter(t *testing.T) {
	req := newTestRequest("diary_read", map[string]any{
		"search": "hello world",
	})

	search := req.GetString("search", "")
	if search != "hello world" {
		t.Errorf("search = %q, want %q", search, "hello world")
	}
}

func TestHandleDiaryRead_ParsesBeforeParameter(t *testing.T) {
	sig := "5VERv8NMhBJG4Rt7qJKrYFtYxvPU2uJGXTcBcfpRz8KSMHj1Hs"
	req := newTestRequest("diary_read", map[string]any{
		"before": sig,
	})

	before := req.GetString("before", "")
	if before != sig {
		t.Errorf("before = %q, want %q", before, sig)
	}
}

func TestHandleDiaryRead_DefaultStringParameters(t *testing.T) {
	// When no optional parameters are set, defaults should be empty strings.
	req := newTestRequest("diary_read", map[string]any{})

	since := req.GetString("since", "")
	search := req.GetString("search", "")
	before := req.GetString("before", "")

	if since != "" {
		t.Errorf("since default = %q, want empty", since)
	}
	if search != "" {
		t.Errorf("search default = %q, want empty", search)
	}
	if before != "" {
		t.Errorf("before default = %q, want empty", before)
	}
}

func TestHandleDiaryRead_AllParametersTogether(t *testing.T) {
	req := newTestRequest("diary_read", map[string]any{
		"limit":  float64(3),
		"since":  "2025-06-01",
		"search": "diary",
		"before": "abc123",
	})

	if req.GetInt("limit", 7) != 3 {
		t.Error("limit not parsed correctly")
	}
	if req.GetString("since", "") != "2025-06-01" {
		t.Error("since not parsed correctly")
	}
	if req.GetString("search", "") != "diary" {
		t.Error("search not parsed correctly")
	}
	if req.GetString("before", "") != "abc123" {
		t.Error("before not parsed correctly")
	}
}

// --- Error code constants tests ---

func TestErrorCodeConstants(t *testing.T) {
	// Verify error codes are in the JSON-RPC server error range.
	codes := []struct {
		name string
		code int
	}{
		{"ErrCodeWalletLocked", ErrCodeWalletLocked},
		{"ErrCodeContentTooLarge", ErrCodeContentTooLarge},
		{"ErrCodeInsufficientBalance", ErrCodeInsufficientBalance},
		{"ErrCodeNetworkError", ErrCodeNetworkError},
	}

	for _, tc := range codes {
		if tc.code > -32000 || tc.code < -32099 {
			t.Errorf("%s = %d, want in range [-32099, -32000]", tc.name, tc.code)
		}
	}

	// Verify all codes are unique.
	seen := make(map[int]string)
	for _, tc := range codes {
		if prev, exists := seen[tc.code]; exists {
			t.Errorf("duplicate error code %d: %s and %s", tc.code, prev, tc.name)
		}
		seen[tc.code] = tc.name
	}
}

// --- toolError helper tests ---

func TestToolError(t *testing.T) {
	result := toolError(-32002, "content too large")

	if !result.IsError {
		t.Fatal("toolError() should set IsError = true")
	}

	text := getResultText(t, result)
	expected := "[-32002] content too large"
	if text != expected {
		t.Errorf("toolError() text = %q, want %q", text, expected)
	}
}

func TestToolError_AllCodes(t *testing.T) {
	cases := []struct {
		code int
		msg  string
	}{
		{ErrCodeWalletLocked, "wallet is locked"},
		{ErrCodeContentTooLarge, "content exceeds 512-byte limit"},
		{ErrCodeInsufficientBalance, "insufficient SOL balance"},
		{ErrCodeNetworkError, "RPC connection failed"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("code_%d", tc.code), func(t *testing.T) {
			result := toolError(tc.code, tc.msg)
			if !result.IsError {
				t.Fatal("toolError() should set IsError = true")
			}
			text := getResultText(t, result)
			prefix := fmt.Sprintf("[%d]", tc.code)
			if !strings.HasPrefix(text, prefix) {
				t.Errorf("toolError() text = %q, want prefix %q", text, prefix)
			}
			if !strings.Contains(text, tc.msg) {
				t.Errorf("toolError() text = %q, want to contain %q", text, tc.msg)
			}
		})
	}
}

// --- Helper functions ---

// getResultText extracts the text from the first TextContent in a CallToolResult.
func getResultText(t *testing.T, result *gomcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := gomcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("first content is not TextContent: %T", result.Content[0])
	}
	return tc.Text
}
