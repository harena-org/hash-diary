package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func newTestFormatter(format string, verbose bool) (*Formatter, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	f := &Formatter{
		Format:    format,
		Verbose:   verbose,
		Writer:    stdout,
		ErrWriter: stderr,
	}
	return f, stdout, stderr
}

func TestSuccess_Text(t *testing.T) {
	f, stdout, _ := newTestFormatter("text", false)
	f.Success("hello world")

	got := stdout.String()
	if got != "hello world\n" {
		t.Errorf("Success text: got %q, want %q", got, "hello world\n")
	}
}

func TestSuccess_JSON(t *testing.T) {
	f, stdout, _ := newTestFormatter("json", false)
	f.Success(map[string]string{"key": "value"})

	var result map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Success JSON: failed to unmarshal: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("Success JSON: got key=%q, want %q", result["key"], "value")
	}
}

func TestError_Text(t *testing.T) {
	f, stdout, stderr := newTestFormatter("text", false)
	f.Error(errors.New("something failed"))

	if stdout.Len() != 0 {
		t.Errorf("Error text: expected no stdout, got %q", stdout.String())
	}
	got := stderr.String()
	if got != "错误: something failed\n" {
		t.Errorf("Error text: got %q, want %q", got, "错误: something failed\n")
	}
}

func TestError_JSON(t *testing.T) {
	f, stdout, stderr := newTestFormatter("json", false)
	f.Error(errors.New("something failed"))

	if stderr.Len() != 0 {
		t.Errorf("Error JSON: expected no stderr, got %q", stderr.String())
	}
	var result map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Error JSON: failed to unmarshal: %v", err)
	}
	if result["error"] != "something failed" {
		t.Errorf("Error JSON: got error=%q, want %q", result["error"], "something failed")
	}
}

func TestPrompt(t *testing.T) {
	f, stdout, stderr := newTestFormatter("text", false)
	f.Prompt("Enter password: ")

	if stdout.Len() != 0 {
		t.Errorf("Prompt: expected no stdout, got %q", stdout.String())
	}
	got := stderr.String()
	if got != "Enter password: " {
		t.Errorf("Prompt: got %q, want %q", got, "Enter password: ")
	}
}

func TestVerboseLog_Enabled(t *testing.T) {
	f, stdout, stderr := newTestFormatter("text", true)
	f.VerboseLog("connecting to %s", "devnet")

	if stdout.Len() != 0 {
		t.Errorf("VerboseLog: expected no stdout, got %q", stdout.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "[verbose]") {
		t.Errorf("VerboseLog: expected [verbose] prefix, got %q", got)
	}
	if !strings.Contains(got, "connecting to devnet") {
		t.Errorf("VerboseLog: expected message content, got %q", got)
	}
}

func TestVerboseLog_Disabled(t *testing.T) {
	f, stdout, stderr := newTestFormatter("text", false)
	f.VerboseLog("should not appear")

	if stdout.Len() != 0 {
		t.Errorf("VerboseLog disabled: expected no stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("VerboseLog disabled: expected no stderr, got %q", stderr.String())
	}
}

func TestPrintDiaryEntries_Text(t *testing.T) {
	f, stdout, _ := newTestFormatter("text", false)
	entries := []DiaryEntry{
		{Timestamp: "2026-03-01 18:30:00 CST", Content: "今天天气不错", Signature: "abc123"},
		{Timestamp: "2026-03-02 09:00:00 CST", Content: "早安世界", Signature: "def456"},
	}
	f.PrintDiaryEntries(entries)

	got := stdout.String()
	if !strings.Contains(got, "[2026-03-01 18:30:00 CST] 今天天气不错") {
		t.Errorf("PrintDiaryEntries text: missing first entry, got %q", got)
	}
	if !strings.Contains(got, "[2026-03-02 09:00:00 CST] 早安世界") {
		t.Errorf("PrintDiaryEntries text: missing second entry, got %q", got)
	}
}

func TestPrintDiaryEntries_JSON(t *testing.T) {
	f, stdout, _ := newTestFormatter("json", false)
	entries := []DiaryEntry{
		{Timestamp: "2026-03-01 18:30:00 CST", Content: "今天天气不错", Signature: "abc123"},
	}
	f.PrintDiaryEntries(entries)

	var result []DiaryEntry
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("PrintDiaryEntries JSON: failed to unmarshal: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("PrintDiaryEntries JSON: expected 1 entry, got %d", len(result))
	}
	if result[0].Timestamp != "2026-03-01 18:30:00 CST" {
		t.Errorf("PrintDiaryEntries JSON: got timestamp=%q", result[0].Timestamp)
	}
	if result[0].Content != "今天天气不错" {
		t.Errorf("PrintDiaryEntries JSON: got content=%q", result[0].Content)
	}
	if result[0].Signature != "abc123" {
		t.Errorf("PrintDiaryEntries JSON: got signature=%q", result[0].Signature)
	}
}

func TestPrintWalletInfo_Text(t *testing.T) {
	f, stdout, _ := newTestFormatter("text", false)
	f.PrintWalletInfo("ABC123pubkey", "1.5", "devnet")

	got := stdout.String()
	if !strings.Contains(got, "ABC123pubkey") {
		t.Errorf("PrintWalletInfo text: missing address, got %q", got)
	}
	if !strings.Contains(got, "1.5 SOL") {
		t.Errorf("PrintWalletInfo text: missing balance, got %q", got)
	}
	if !strings.Contains(got, "devnet") {
		t.Errorf("PrintWalletInfo text: missing network, got %q", got)
	}
}

func TestPrintWalletInfo_JSON(t *testing.T) {
	f, stdout, _ := newTestFormatter("json", false)
	f.PrintWalletInfo("ABC123pubkey", "1.5", "devnet")

	var result map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("PrintWalletInfo JSON: failed to unmarshal: %v", err)
	}
	if result["address"] != "ABC123pubkey" {
		t.Errorf("PrintWalletInfo JSON: got address=%q", result["address"])
	}
	if result["balance"] != "1.5" {
		t.Errorf("PrintWalletInfo JSON: got balance=%q", result["balance"])
	}
	if result["network"] != "devnet" {
		t.Errorf("PrintWalletInfo JSON: got network=%q", result["network"])
	}
}

func TestPrintTransaction_Text(t *testing.T) {
	f, stdout, _ := newTestFormatter("text", false)
	f.PrintTransaction("sig123abc", "confirmed")

	got := stdout.String()
	if !strings.Contains(got, "sig123abc") {
		t.Errorf("PrintTransaction text: missing signature, got %q", got)
	}
	if !strings.Contains(got, "confirmed") {
		t.Errorf("PrintTransaction text: missing status, got %q", got)
	}
}

func TestPrintTransaction_JSON(t *testing.T) {
	f, stdout, _ := newTestFormatter("json", false)
	f.PrintTransaction("sig123abc", "confirmed")

	var result map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("PrintTransaction JSON: failed to unmarshal: %v", err)
	}
	if result["signature"] != "sig123abc" {
		t.Errorf("PrintTransaction JSON: got signature=%q", result["signature"])
	}
	if result["status"] != "confirmed" {
		t.Errorf("PrintTransaction JSON: got status=%q", result["status"])
	}
}

func TestNew(t *testing.T) {
	f := New("json", true)
	if f.Format != "json" {
		t.Errorf("New: got format=%q, want %q", f.Format, "json")
	}
	if !f.Verbose {
		t.Error("New: expected verbose=true")
	}
	if f.Writer == nil {
		t.Error("New: Writer should not be nil")
	}
	if f.ErrWriter == nil {
		t.Error("New: ErrWriter should not be nil")
	}
}

func TestPrintDiaryEntries_Empty(t *testing.T) {
	f, stdout, _ := newTestFormatter("text", false)
	f.PrintDiaryEntries([]DiaryEntry{})

	if stdout.Len() != 0 {
		t.Errorf("PrintDiaryEntries empty text: expected no output, got %q", stdout.String())
	}
}

func TestPrintDiaryEntries_Empty_JSON(t *testing.T) {
	f, stdout, _ := newTestFormatter("json", false)
	f.PrintDiaryEntries([]DiaryEntry{})

	var result []DiaryEntry
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("PrintDiaryEntries empty JSON: failed to unmarshal: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("PrintDiaryEntries empty JSON: expected 0 entries, got %d", len(result))
	}
}
