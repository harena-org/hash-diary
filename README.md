# HashDiary (哈希日记)

基于 Solana 区块链的命令行加密日记工具。将日记内容压缩、加密后写入链上 Memo 程序，实现不可篡改的私密存储。

## 功能特性

- **端到端加密** -- NaCl box (Curve25519-XSalsa20-Poly1305)，Ed25519 密钥自动转换为 X25519，仅持有私钥可解密
- **Zlib 压缩** -- 自动压缩日记内容，单条约 500-800 个中文字符
- **协议前缀** -- `HD:` 前缀标识 HashDiary 交易，跳过无关 Memo
- **本地缓存** -- 增量同步 + 文件锁保护，支持多终端并发和多钱包隔离
- **MCP Server** -- Model Context Protocol 模式，供 AI Agent (Claude Desktop / OpenClaw) 调用
- **密码保护** -- scrypt (N=32768, r=8, p=1) + AES-256-GCM 加密密钥文件
- **Solana CLI 兼容** -- 明文钱包与 `solana-keygen` 格式互操作
- **多网络** -- devnet / testnet / mainnet-beta 及自定义 RPC 端点
- **管道友好** -- 支持 stdin 输入、`--format json` 输出、所有提示走 stderr

## 安装

### 从源码编译

```bash
go install github.com/hash-diary/hash-diary@latest
```

或者克隆仓库：

```bash
git clone https://github.com/hash-diary/hash-diary.git
cd hash-diary
make build
```

### 系统要求

- Go 1.25 或更高版本

## 快速开始

```bash
# 1. 创建钱包（可选设置密码，回车跳过）
hash-diary wallet new

# 2. 领取 devnet 测试代币
hash-diary wallet airdrop

# 3. 查看钱包余额
hash-diary wallet show

# 4. 写入日记
hash-diary write "今天天气很好"

# 5. 读取日记
hash-diary read
```

## 命令参考

### 全局选项

| 选项 | 缩写 | 默认值 | 说明 |
|------|------|--------|------|
| `--url` | `-u` | `https://api.devnet.solana.com` | Solana RPC 端点 |
| `--keypair` | | `~/.hash-diary/id.json` | 钱包密钥文件路径 |
| `--password` | | | 解锁加密钱包的密码 |
| `--format` | | `text` | 输出格式：`text` \| `json` |
| `--verbose` | | `false` | 显示详细日志（输出到 stderr） |
| `--version` | `-v` | | 显示版本号 |

密码优先级：`--password` > 环境变量 `HASH_DIARY_PASSWORD` > 交互式输入

### wallet -- 钱包管理

#### `wallet new`

创建新的 Ed25519 密钥对，保存到 `--keypair` 指定路径。

```bash
hash-diary wallet new                    # 交互式设置可选密码
hash-diary wallet new --no-password      # 明文保存（Solana CLI 兼容格式）
hash-diary wallet new --force            # 覆盖已有文件
```

| 选项 | 说明 |
|------|------|
| `--force` | 覆盖已有密钥文件 |
| `--no-password` | 跳过密码，以明文 JSON 数组保存 |

输出示例：

```
钱包已创建，地址: 7xKX...3nPq
密钥文件已保存至: ~/.hash-diary/id.json
```

#### `wallet show`

显示当前钱包的公钥地址、SOL 余额和网络。

```bash
hash-diary wallet show
hash-diary wallet show --format json
```

输出示例：

```
地址:    7xKX...3nPq
余额:    1.5 SOL
网络:    devnet
```

#### `wallet airdrop`

在 devnet/testnet 上领取测试用 SOL（mainnet 不可用）。

```bash
hash-diary wallet airdrop              # 默认 1 SOL
hash-diary wallet airdrop --amount 2   # 指定数量
```

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `--amount` | `1` | 领取的 SOL 数量 |

#### `wallet import`

从已有的密钥文件、Base58 私钥或 BIP-39 助记词导入钱包。三种方式互斥。

```bash
# 从密钥文件导入
hash-diary wallet import /path/to/keypair.json

# 从 Base58 私钥导入
hash-diary wallet import --private-key <base58-private-key>

# 从 BIP-39 助记词恢复（使用 Solana 标准派生路径 m/44'/501'/0'/0'）
hash-diary wallet import --mnemonic "word1 word2 ... word12"
hash-diary wallet import --mnemonic "word1 word2 ... word12" --passphrase "optional-bip39-passphrase"
hash-diary wallet import --mnemonic "word1 word2 ... word12" --no-password
```

| 选项 | 说明 |
|------|------|
| `--private-key` | Base58 编码的 64 字节 Ed25519 私钥 |
| `--mnemonic` | BIP-39 助记词（12 或 24 个单词） |
| `--passphrase` | 可选的 BIP-39 passphrase（非钱包加密密码） |
| `--no-password` | 跳过密码，以明文保存 |

### write -- 写入日记

将文本日记加密后写入 Solana Memo 程序。

```bash
hash-diary write "今天天气很好"
hash-diary write "不等待确认" --no-wait
echo "通过管道写入" | hash-diary write
hash-diary write < file.txt
```

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `--no-wait` | `false` | 发送后立即返回，不等待 confirmed 确认 |

输入方式优先级：命令行参数 > stdin。

写入流程：UTF-8 文本 → Zlib 压缩 → NaCl box 加密 → Base64 编码 → `HD:` 前缀 → Memo 交易（自发自收）

交易发送失败最多重试 3 次（退避 500ms / 1s / 2s），确认超时 60 秒后返回签名 + timeout 状态。

### read -- 读取日记

从链上读取日记，优先使用本地缓存（`~/.hash-diary/cache.json`），仅拉取增量更新。

```bash
hash-diary read                              # 最近 7 条
hash-diary read --limit 20                   # 最近 20 条
hash-diary read --search "关键词"            # 关键词搜索（大小写不敏感）
hash-diary read --since 2026-01-01           # 指定日期之后（本地时区）
hash-diary read --before <signature>         # 游标分页
hash-diary read --force-refresh              # 忽略缓存，全量重建
hash-diary read --format json                # JSON 格式输出
```

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `--limit` | `7` | 最大返回记录数 |
| `--since` | | 日期过滤（`YYYY-MM-DD`，按本地时区） |
| `--search` | | 关键词过滤（大小写不敏感） |
| `--before` | | 游标分页：返回指定交易签名之前的记录 |
| `--force-refresh` | `false` | 忽略缓存，全量拉取重建 |

输出按时间倒序排列；时间戳相同时按签名字典序倒序保证稳定输出。当返回条数等于 limit 时显示分页提示。

时间戳使用 Solana 区块时间（Block Time），显示时转换为本地时区，格式：`YYYY-MM-DD HH:MM:SS TZ`。

### mcp -- MCP Server 模式

以 MCP Server 运行，通过 stdio (JSON-RPC) 与 AI Agent 通信。

```bash
hash-diary mcp
hash-diary mcp -u https://api.mainnet-beta.solana.com
hash-diary mcp --password <password>
HASH_DIARY_PASSWORD=<password> hash-diary mcp
```

**注意**：MCP 模式使用 stdin/stdout 进行 JSON-RPC 通信，不支持交互式密码输入。加密钱包必须通过 `--password` 或 `HASH_DIARY_PASSWORD` 环境变量提供密码。

暴露工具：

| 工具 | 参数 | 说明 |
|------|------|------|
| `diary_write` | `text` (string, 必填) | 写入一条加密日记，返回交易签名 |
| `diary_read` | `limit` (number, 默认7), `since` (string), `search` (string), `before` (string) | 读取日记记录，返回 JSON 数组 |

MCP 错误码（-32000 ~ -32099）：

| 错误码 | 含义 |
|--------|------|
| -32001 | 钱包未解锁 |
| -32002 | 内容超过 512 字节限制 |
| -32003 | SOL 余额不足 |
| -32004 | 网络/RPC 错误 |

## MCP 集成配置

在 AI 客户端（Claude Desktop、OpenClaw 等）中配置：

```json
{
  "mcpServers": {
    "hash-diary": {
      "command": "hash-diary",
      "args": ["mcp"],
      "env": {
        "HASH_DIARY_PASSWORD": "your-password-if-needed"
      }
    }
  }
}
```

若使用无密码钱包，可省略 `env` 字段。

## 项目结构

```
hash-diary/
├── main.go                 # 入口
├── cmd/                    # CLI 命令定义
│   ├── root.go             #   根命令 + 全局选项
│   ├── wallet.go           #   wallet new/show/airdrop/import
│   ├── write.go            #   write 子命令
│   ├── read.go             #   read 子命令
│   └── mcp.go              #   mcp 子命令
├── internal/
│   ├── crypto/             # 加密模块：密钥转换、Zlib、NaCl box、编码管道
│   ├── wallet/             # 钱包管理：生成、加密/明文存储、导入、密码提供者
│   ├── rpc/                # Solana RPC 封装：余额、空投、交易发送/确认、签名查询
│   ├── cache/              # 本地缓存：JSON 文件、文件锁、多钱包隔离、版本迁移
│   ├── output/             # 输出格式化：text/json 双格式、stderr 提示
│   └── mcp/                # MCP Server：stdio JSON-RPC、diary_write/diary_read 工具
└── tests/                  # 集成测试
```

## 技术细节

| 项目 | 说明 |
|------|------|
| 语言 | Go 1.25+ |
| 日记加密 | Ed25519 → X25519 密钥转换 + NaCl box (Curve25519-XSalsa20-Poly1305) |
| 数据压缩 | Zlib (BestCompression) |
| 编码格式 | `HD:` 前缀 + Standard Base64 (RFC 4648) |
| 加密开销 | 24 字节随机 nonce + 16 字节 Poly1305 MAC = 40 字节 |
| Memo 上限 | 512 字节（含前缀），明文上限约 344 字节 |
| Memo Program | `MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr` |
| 交易模式 | 自发自收（payer = signer = 当前钱包） |
| 交易费用 | ~0.000005 SOL (5000 lamports) 每条 |
| 确认级别 | `confirmed`，超时 60 秒（轮询间隔 2 秒） |
| 发送重试 | 最多 3 次，退避 500ms / 1s / 2s |
| 钱包加密 | scrypt (N=32768, r=8, p=1, keyLen=32) + AES-256-GCM |
| 明文钱包 | Solana CLI 兼容格式（64 字节 JSON 数组） |
| 缓存路径 | `~/.hash-diary/cache.json`（权限 600） |
| 缓存策略 | 增量同步 + 文件锁 + 原子写入 + 版本迁移 |
| MCP 传输 | stdio (JSON-RPC 2.0) |
| 文件权限 | 密钥文件 0600，目录 0700 |

### 错误处理

| 场景 | 行为 |
|------|------|
| 内容超 512 字节 | 报错提示当前大小及 ~344 字节明文上限 |
| 钱包不存在 | 引导执行 `wallet new` 或 `--keypair` |
| 密码错误 | 提示"密码错误" |
| 加密钱包未提供密码 | 提示使用 `--password` 或 `HASH_DIARY_PASSWORD` |
| mainnet airdrop | 拒绝，提示仅支持 devnet/testnet |
| 余额不足 | 提示余额不足并建议充值 |
| 网络故障 | 建议检查网络或更换 RPC 端点 (`--url`) |
| RPC 限流 | 退避重试 + 建议切换端点 |
| 交易超时 | 返回签名 + timeout 状态 |
| 解码失败 | 跳过记录，`--verbose` 显示详情 |

`--format json` 模式下，错误统一为 `{"error": "<描述>"}` 输出到 stdout，非零退出码。

### 依赖

| 库 | 版本 | 用途 |
|----|------|------|
| `github.com/gagliardetto/solana-go` | v1.14.0 | Solana SDK |
| `github.com/spf13/cobra` | v1.10.2 | CLI 框架 |
| `golang.org/x/crypto` | v0.48.0 | NaCl box / scrypt / curve25519 |
| `filippo.io/edwards25519` | v1.2.0 | Ed25519 → X25519 转换 |
| `github.com/gofrs/flock` | v0.13.0 | 文件锁 |
| `github.com/mark3labs/mcp-go` | v0.44.1 | MCP SDK |

## 开发

```bash
make build          # 编译
make test           # 运行测试（含 race detector）
make test-verbose   # 详细测试输出
make lint           # go vet 静态检查
make clean          # 清理构建产物
```

测试覆盖 7 个包、202 个测试函数，含加密 round-trip、钱包生命周期、RPC mock、缓存并发、MCP 工具参数、集成管道测试。

## 许可证

[MIT](LICENSE)
