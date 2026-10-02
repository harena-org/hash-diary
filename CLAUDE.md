# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build          # Compile binary → ./hash-diary
make test           # Run all tests with race detector
make test-verbose   # Verbose test output
make lint           # go vet ./...
go test ./internal/wallet/ -run TestImportFromMnemonic -v  # Run a single test
```

## Architecture

HashDiary is a CLI tool that writes encrypted diary entries to the Solana blockchain via the p-memo program. All user-facing text is in Chinese.

**Module:** `github.com/hash-diary/hash-diary` (Go 1.25.5)

### Package structure

- `cmd/` — Cobra CLI commands: `wallet` (new/show/airdrop/import), `write`, `read`, `mcp`
- `internal/crypto/` — NaCl box encryption, Ed25519→X25519 key conversion, zlib compression
- `internal/wallet/` — Keypair generation, save/load (plaintext & scrypt+AES-256-GCM encrypted), import (file/Base58/BIP-39 mnemonic)
- `internal/rpc/` — Solana RPC wrapper (balance, airdrop, send tx, get signatures/transactions, memo extraction)
- `internal/cache/` — JSON file cache with flock-based file locking, per-wallet entry storage
- `internal/output/` — Dual-mode formatter (text/json), verbose logging to stderr
- `internal/mcp/` — MCP (Model Context Protocol) server over stdio
- `tests/` — Integration tests (wallet+crypto+cache pipelines)

### Data flow

**Write:** text → zlib compress → NaCl box encrypt → base64 → prepend `HD:` prefix → Solana Memo instruction → sign & send tx

**Read:** GetSignaturesForAddress (paginated) → GetTransaction → extract memo from logs (strip quotes from Rust `{:?}` format) → check `HD:` prefix → base64 decode → NaCl decrypt → zlib decompress → cache locally → filter (since/search/before/limit) → display

### Key conventions

- **Error messages** are in Chinese with `fmt.Errorf("描述: %w", err)` wrapping
- **Output modes:** all commands support `--format text|json`. Prompts and verbose logs go to stderr; results go to stdout
- **Password chain:** `--password` flag → `HASH_DIARY_PASSWORD` env → interactive prompt (in that priority order)
- **File permissions:** 0600 for wallet files, 0700 for directories
- **Wallet storage:** plaintext format is Solana CLI compatible (JSON array of 64 bytes); encrypted uses scrypt+AES-256-GCM
- **Cache:** `~/.hash-diary/cache.json` with `lastSignature` checkpoint for incremental sync; file-locked with `.lock` file
- **Memo format:** Uses p-memo (`Memo4c2pN8afCj432Lb7RMVKi9PbQnnW7ewFFaV3oAH`). p-memo logs on two lines — `Program log: Memo (len N)` then `Program log: <data>` (no quotes). The legacy Memo program logs inline as `Program log: Memo (len N): "data"` (quotes must be stripped). `ExtractMemoData` supports both formats for backwards compatibility.
- **Global flags** (`flagURL`, `flagKeypair`, `flagPassword`, `flagFormat`, `flagVerbose`) are defined in `cmd/root.go` and used across all commands
- **Tests** use table-driven style, `t.TempDir()` for file operations, and always run with `-race`
