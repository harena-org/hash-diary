// Package output provides output formatting for HashDiary CLI.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// DiaryEntry represents a single diary entry for display.
type DiaryEntry struct {
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
	Signature string `json:"signature"`
}

// Formatter handles output formatting for the CLI.
type Formatter struct {
	Format    string    // "text" or "json"
	Verbose   bool      // whether verbose logging is enabled
	Writer    io.Writer // stdout
	ErrWriter io.Writer // stderr
}

// New creates a new Formatter with the given settings.
func New(format string, verbose bool) *Formatter {
	return &Formatter{
		Format:    format,
		Verbose:   verbose,
		Writer:    os.Stdout,
		ErrWriter: os.Stderr,
	}
}

// Success outputs a success result.
// In text mode, it formats as a human-readable string.
// In json mode, it marshals to JSON and prints.
func (f *Formatter) Success(data interface{}) {
	if f.Format == "json" {
		f.printJSON(data)
		return
	}
	fmt.Fprintln(f.Writer, data)
}

// Error outputs an error.
// In text mode, it writes to stderr in Chinese.
// In json mode, it writes a JSON error object to stdout.
func (f *Formatter) Error(err error) {
	if f.Format == "json" {
		f.printJSON(map[string]string{"error": err.Error()})
		return
	}
	fmt.Fprintf(f.ErrWriter, "错误: %s\n", err)
}

// Prompt writes an interactive prompt message to stderr.
// Prompts always go to stderr so they don't interfere with piped output.
func (f *Formatter) Prompt(msg string) {
	fmt.Fprint(f.ErrWriter, msg)
}

// VerboseLog writes a verbose log message to stderr if verbose mode is enabled.
// It must NOT leak private key paths, plaintext passwords, or diary content.
func (f *Formatter) VerboseLog(msg string, args ...interface{}) {
	if !f.Verbose {
		return
	}
	fmt.Fprintf(f.ErrWriter, "[verbose] "+msg+"\n", args...)
}

// PrintDiaryEntries formats and outputs diary entries.
// In text mode: [2026-03-01 18:30:00 CST] content
// In json mode: JSON array of entry objects.
func (f *Formatter) PrintDiaryEntries(entries []DiaryEntry) {
	if f.Format == "json" {
		f.printJSON(entries)
		return
	}
	for _, entry := range entries {
		fmt.Fprintf(f.Writer, "[%s] %s\n", entry.Timestamp, entry.Content)
	}
}

// PrintWalletInfo formats and outputs wallet information.
func (f *Formatter) PrintWalletInfo(address, balance, network string) {
	if f.Format == "json" {
		f.printJSON(map[string]string{
			"address": address,
			"balance": balance,
			"network": network,
		})
		return
	}
	fmt.Fprintf(f.Writer, "地址:    %s\n", address)
	fmt.Fprintf(f.Writer, "余额:    %s SOL\n", balance)
	fmt.Fprintf(f.Writer, "网络:    %s\n", network)
}

// PrintTransaction formats and outputs a transaction result.
func (f *Formatter) PrintTransaction(signature, status string) {
	if f.Format == "json" {
		f.printJSON(map[string]string{
			"signature": signature,
			"status":    status,
		})
		return
	}
	fmt.Fprintf(f.Writer, "交易签名: %s\n", signature)
	fmt.Fprintf(f.Writer, "状态:     %s\n", status)
}

// printJSON marshals data to JSON and writes it to the Writer.
func (f *Formatter) printJSON(data interface{}) {
	enc := json.NewEncoder(f.Writer)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		fmt.Fprintf(f.ErrWriter, "错误: JSON encoding failed: %s\n", err)
	}
}
