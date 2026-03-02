# HashDiary (哈希日记)

基于 Solana 区块链的命令行加密日记工具。将日记内容加密后写入链上 Memo 程序，实现不可篡改的私密存储。

## 功能特性

- **端到端加密** -- 使用 NaCl box (Curve25519-XSalsa20-Poly1305) 加密，仅持有私钥的钱包可解密
- **Zlib 压缩** -- 自动压缩日记内容，单条可存储约 500-800 个中文字符
- **本地缓存** -- 增量同步链上数据，大幅提升读取性能
- **MCP Server 模式** -- 支持 Model Context Protocol，供 AI Agent 直接调用日记读写能力
- **密码保护钱包** -- 支持 scrypt + AES-256-GCM 加密密钥文件
- **多网络支持** -- 支持 devnet / testnet / mainnet-beta 及自定义 RPC 端点
- **管道友好** -- 支持 stdin 输入和 JSON 输出格式，便于脚本集成

## 安装

### 从源码编译

```bash
go install github.com/hash-diary/hash-diary@latest
```

或者克隆仓库后编译：

```bash
git clone https://github.com/hash-diary/hash-diary.git
cd hash-diary
make build
```

编译完成后，`hash-diary` 二进制文件将生成在项目根目录下。

### 系统要求

- Go 1.25 或更高版本

## 快速开始

### 1. 创建钱包

```bash
hash-diary wallet new
```

按提示设置密码（可选，直接回车跳过）。钱包文件默认保存至 `~/.hash-diary/id.json`。

### 2. 领取测试代币 (devnet)

```bash
hash-diary wallet airdrop
```

默认在 devnet 上领取 1 SOL 测试代币。

### 3. 写入日记

```bash
hash-diary write "今天天气很好"
```

也可以通过管道写入：

```bash
echo "通过管道写入" | hash-diary write
```

### 4. 读取日记

```bash
# 读取最近日记（默认 7 条）
hash-diary read

# 关键词搜索
hash-diary read --search "天气"

# 限制返回数量
hash-diary read --limit 10

# 按日期过滤
hash-diary read --since 2026-01-01
```

## 命令参考

### 全局选项

| 选项 | 缩写 | 默认值 | 说明 |
|------|------|--------|------|
| `--url` | `-u` | `https://api.devnet.solana.com` | Solana RPC 端点 URL |
| `--keypair` | | `~/.hash-diary/id.json` | 钱包密钥文件路径 |
| `--password` | | | 解锁加密钱包的密码 |
| `--format` | | `text` | 输出格式: `text` 或 `json` |
| `--verbose` | | `false` | 显示详细日志 |
| `--version` | | | 显示版本号 |
| `--help` | | | 显示帮助信息 |

### wallet 子命令

管理 HashDiary 使用的 Solana 钱包。

#### `wallet new` -- 创建新钱包

```bash
hash-diary wallet new                        # 创建并设置可选密码
hash-diary wallet new --force                 # 覆盖已有密钥文件
hash-diary wallet new --no-password           # 跳过密码，明文存储
hash-diary wallet new --keypair ./my.json     # 指定保存路径
```

| 选项 | 说明 |
|------|------|
| `--force` | 覆盖已有密钥文件，不再确认 |
| `--no-password` | 跳过密码设置，以明文保存密钥 |

#### `wallet show` -- 查看钱包信息

显示当前钱包的公钥地址和 SOL 余额。

```bash
hash-diary wallet show
hash-diary wallet show -u https://api.mainnet-beta.solana.com
```

#### `wallet airdrop` -- 领取测试代币

在 devnet/testnet 上领取测试用 SOL（mainnet 不可用）。

```bash
hash-diary wallet airdrop              # 默认 1 SOL
hash-diary wallet airdrop --amount 2   # 指定数量
```

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `--amount` | `1` | 领取的 SOL 数量 |

#### `wallet import` -- 导入已有钱包

从已有的密钥文件或 Base58 私钥导入钱包。

```bash
hash-diary wallet import /path/to/keypair.json
hash-diary wallet import --private-key <base58-private-key>
hash-diary wallet import --private-key <key> --no-password
```

| 选项 | 说明 |
|------|------|
| `--private-key` | 通过 Base58 编码的私钥导入 |
| `--no-password` | 跳过密码设置，以明文保存密钥 |

### write 命令

将文本日记写入 Solana 链上。

```bash
hash-diary write "日记内容"
hash-diary write "内容" --no-wait           # 发送后立即返回，不等待确认
echo "管道输入" | hash-diary write
hash-diary write < file.txt
```

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `--no-wait` | `false` | 发送交易后立即返回，不等待确认 |

输入方式优先级：命令行参数 > stdin。

### read 命令

读取链上日记记录。优先从本地缓存读取，仅拉取增量更新。

```bash
hash-diary read                              # 最近 7 条
hash-diary read --limit 20                   # 最近 20 条
hash-diary read --search "关键词"            # 关键词搜索
hash-diary read --since 2026-01-01           # 指定日期之后
hash-diary read --before <signature>         # 游标分页
hash-diary read --force-refresh              # 忽略缓存，全量拉取
```

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `--limit` | `7` | 最大返回记录数 |
| `--since` | | 起始日期过滤，格式 `YYYY-MM-DD`（按本地时区） |
| `--search` | | 关键词过滤（大小写不敏感） |
| `--before` | | 游标分页，返回指定签名之前的记录 |
| `--force-refresh` | `false` | 忽略缓存，强制全量拉取链上数据 |

### mcp 命令

以 MCP Server 模式运行，通过 stdio 与 AI Agent 通信。

```bash
hash-diary mcp
hash-diary mcp -u https://api.mainnet-beta.solana.com
hash-diary mcp --password <password>
HASH_DIARY_PASSWORD=<password> hash-diary mcp
```

MCP 模式暴露以下工具：

| 工具 | 说明 |
|------|------|
| `diary_write` | 写入一条日记（参数: `text`） |
| `diary_read` | 读取日记记录（参数: `limit`, `since`, `search`, `before`） |

## MCP Server 集成

在 AI 客户端（如 Claude Desktop、OpenClaw）中配置 HashDiary 作为 MCP Server：

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

> 提示：若使用无密码钱包，可省略 `env` 字段。

密码优先级：`--password` 参数 > `HASH_DIARY_PASSWORD` 环境变量。MCP 模式下不支持交互式密码输入。

## 技术细节

| 项目 | 说明 |
|------|------|
| 日记加密 | Ed25519 -> X25519 密钥转换 + NaCl box (Curve25519-XSalsa20-Poly1305) |
| 数据压缩 | Zlib |
| 编码格式 | `HD:` 前缀 + Standard Base64 (RFC 4648) |
| 加密开销 | 24 字节随机 nonce + 16 字节 Poly1305 MAC = 40 字节 |
| Memo 上限 | 512 字节（含前缀），明文约 500-800 个中文字符 |
| 交易模式 | 自发自收（目标地址 = 当前钱包地址，Memo Program） |
| 交易费用 | ~0.000005 SOL（5000 lamports）每条 |
| 钱包加密 | scrypt (N=32768, r=8, p=1) + AES-256-GCM |
| 钱包格式 | 明文兼容 Solana CLI 格式（JSON 字节数组） |
| 缓存文件 | `~/.hash-diary/cache.json`，文件锁保护并发访问 |
| MCP 传输 | stdio (JSON-RPC) |

## 许可证

[MIT](LICENSE)
