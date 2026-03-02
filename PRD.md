# HashDiary (哈希日记) - 产品需求文档

## 1. 产品概述

HashDiary 是一个基于 Solana 区块链的命令行日记工具。用户通过 CLI 将文本日记写入链上 Memo 程序，实现内容的不可篡改存储，并可随时读取历史日记记录。

同时，HashDiary 提供 MCP (Model Context Protocol) Server 模式，使 OpenClaw 等 AI Agent 能够直接调用日记的读写能力，实现 AI 驱动的链上日记管理。

## 2. 核心概念

| 概念 | 说明 |
|------|------|
| 链上存储 | 利用 Solana Memo Program 将日记内容附加到交易中，永久存储在区块链上 |
| 自发自收交易 | 交易的目标地址是当前钱包自身，形成"给自己写信"的模式 |
| 协议前缀 | 明文前缀 `HD:`，用于快速识别 HashDiary 交易，避免解密无关 Memo |
| 数据压缩 | 使用 Zlib 压缩日记内容，提升存储容量 |
| NaCl box 加密 | 将 Ed25519 密钥对转换为 X25519 密钥对，使用 NaCl box (Curve25519-XSalsa20-Poly1305) 进行加密，每次加密必须随机生成 24 字节 nonce，仅持有对应私钥的钱包可解密 |
| Base64 编码 | 加密后的内容在发送前必须进行 Base64 编码 |
| 512 字节限制 | 单条 Memo 最大 512 字节。加密开销 40 字节，Base64 膨胀约 33%。结合 Zlib 压缩，预计可存储 500-800 个中文字符 |
| MCP 协议 | 通过标准化的 MCP Server 暴露工具能力，供 AI Agent 调用 |

## 3. 目标用户

- 区块链爱好者，希望将个人记录永久存储在链上
- 开发者，用于学习 Solana 交易与 Memo 程序的交互
- AI Agent（如 OpenClaw），通过 MCP 协议自动化管理链上日记

## 4. 功能需求

### 4.1 网络配置

- **默认 RPC**: `https://api.devnet.solana.com`（devnet）
- 用户通过 `-u` / `--url` 参数指定 Solana RPC 端点 URL，例如 `-u https://api.mainnet-beta.solana.com`
- 常用 RPC 端点：
  - devnet: `https://api.devnet.solana.com`
  - testnet: `https://api.testnet.solana.com`
  - mainnet-beta: `https://api.mainnet-beta.solana.com`
- 也可指定自定义 RPC 端点（如 Helius、QuickNode 等）

### 4.2 钱包管理子命令 (wallet)

**功能描述**: 管理 HashDiary 使用的 Solana 钱包，包括创建、查看、导入密钥对。

- 读取本地钱包密钥文件，**默认路径 `~/.hash-diary/id.json`**
- 支持通过 `--keypair` 参数指定密钥文件路径
- 钱包地址既作为交易发送方，也作为交易接收方（自发自收）

#### `wallet new` — 创建新钱包

生成新的 Solana 密钥对，保存到默认路径 `~/.hash-diary/id.json`。

**流程**:
1. 检查目标路径是否已存在密钥文件
2. 若已存在，提示用户确认是否覆盖（`--force` 可跳过确认）
3. 提示用户输入密码（可选，允许为空，直接回车跳过）
4. 生成新的 Ed25519 密钥对
5. 若设置了密码，使用密码加密密钥后保存；否则以明文 JSON 格式保存
6. 输出新钱包的公钥地址

**密码说明**:
- 密码为可选项，允许为空（直接回车跳过）
- 设置密码后，密钥文件将以加密形式存储，每次使用钱包时需输入密码解锁
- 不设置密码时，密钥文件以明文 JSON 格式保存，使用时无需输入密码
- 密钥文件加密方案：使用 scrypt 从密码派生密钥，再通过 AES-256-GCM 加密私钥内容
- 明文钱包文件格式：兼容 Solana CLI 格式，JSON 数组存储 64 字节密钥对（`[byte_array]`），可与 `solana-keygen` 等工具互操作
- 加密钱包文件格式：JSON 结构，包含 `address`（明文公钥地址，便于无需密码即可识别钱包）、`encrypted`（Base64 密文）、`nonce`（Base64 AES-GCM nonce）、`salt`（Base64 scrypt salt）、`scrypt`（`{"N": 32768, "r": 8, "p": 1}`）字段

**命令示例**:
```bash
hash-diary wallet new
hash-diary wallet new --keypair ./my-wallet.json
hash-diary wallet new --force
hash-diary wallet new --no-password
```

**输出示例 (text)**:
```
请输入钱包密码（可选，直接回车跳过）:
钱包已创建，地址: 7xKX...3nPq
密钥文件已保存至: ~/.hash-diary/id.json
```

**输出示例 (json)**:
```json
{"address": "7xKX...3nPq", "keypair": "~/.hash-diary/id.json"}
```

#### `wallet show` — 查看钱包信息

显示当前钱包的公钥地址和 SOL 余额。

**命令示例**:
```bash
hash-diary wallet show
hash-diary wallet show -u https://api.mainnet-beta.solana.com
hash-diary wallet show --keypair ./my-wallet.json
```

**输出示例 (text)**:
```
地址: 7xKX...3nPq
余额: 1.5 SOL
网络: devnet
```

**输出示例 (json)**:
```json
{"address": "7xKX...3nPq", "balance": 1.5, "network": "devnet"}
```

#### `wallet airdrop` — 领取测试代币

在 devnet/testnet 上领取测试用 SOL（仅限非 mainnet 网络）。

**命令示例**:
```bash
hash-diary wallet airdrop
hash-diary wallet airdrop --amount 2
```

**输出示例 (text)**:
```
已领取 1 SOL，当前余额: 2.5 SOL
```

**输出示例 (json)**:
```json
{"airdrop": 1, "balance": 2.5}
```

#### `wallet import` — 导入已有钱包

从已有的密钥文件或私钥导入钱包。导入时同样支持设置可选密码。

**命令示例**:
```bash
hash-diary wallet import /path/to/existing-keypair.json
hash-diary wallet import --private-key <base58-private-key>
hash-diary wallet import --private-key <base58-private-key> --no-password
```

**输出示例 (text)**:
```
请输入钱包密码（可选，直接回车跳过）:
钱包已导入，地址: 9aBC...xY2z
密钥文件已保存至: ~/.hash-diary/id.json
```

**输出示例 (json)**:
```json
{"address": "9aBC...xY2z", "keypair": "~/.hash-diary/id.json"}
```

### 4.3 写入子命令 (write)

**功能描述**: 将文本内容作为 Memo 写入 Solana 链上。

**流程**:
1. 用户输入文本内容（通过命令行参数或 stdin）
2. 将 Ed25519 密钥对转换为 X25519 密钥对
3. 使用 Zlib 对文本进行压缩
4. 使用 NaCl box 对压缩数据进行加密
5. 对加密后的内容进行 Base64 编码，并添加前缀 `HD:`
6. 校验编码后大小不超过 512 字节，超出则报错提示
7. 构建交易：目标地址为当前钱包，附带 Memo 指令
8. 签名并发送交易，默认等待交易达到 `confirmed` 状态
9. 返回交易签名（Transaction Signature）供用户查询

**输入方式**:
- 命令行参数：`hash-diary write "日记内容"`
- 标准输入：`echo "日记内容" | hash-diary write`  或  `hash-diary write < file.txt`
- 若同时提供参数和 stdin，优先使用命令行参数

**命令示例**:
```bash
hash-diary write "今天天气很好"
hash-diary write "Hello World" -u https://api.mainnet-beta.solana.com
hash-diary write "日记内容" --keypair ./my-wallet.json
hash-diary write "不等待确认" --no-wait
echo "通过管道写入" | hash-diary write
```

**输出示例 (text)**:
```
交易已发送，等待确认...
交易确认成功 (Confirmed)，签名: 5UfD...xK3m
```

**输出示例 (json)**:
```json
{"signature": "5UfD...xK3m"}
```

### 4.4 读取子命令 (read)

**功能描述**: 查询当前钱包地址的历史 Memo 交易，解码并展示日记内容。优先从本地缓存读取，仅拉取增量更新。

**流程**:
1. 读取本地缓存（如 `~/.hash-diary/cache.json`），获取已知的最新交易签名
2. 从 RPC 节点拉取当前钱包的新交易（使用 `until` 参数指向本地最新签名）
3. 筛选 Memo 包含 `HD:` 前缀的新交易，去除前缀
4. 对新数据进行 Base64 解码 -> NaCl 解密 -> Zlib 解压
5. 将新解密的日记追加到本地缓存中
6. 根据用户查询条件（时间、关键词等）过滤并展示日记内容

**命令示例**:
```bash
hash-diary read
hash-diary read --search "Solana"    # 关键词搜索
hash-diary read --limit 10
hash-diary read --since 2026-02-01
hash-diary read --force-refresh      # 强制全量拉取（忽略缓存）
```

**输出示例 (text)**:
```
[2026-03-01 18:30:00 CST] 今天天气很好
[2026-03-01 16:15:00 CST] Hello World (包含关键词 "Solana")
-- 更多记录: hash-diary read --before 3aBc...yZ9w
```

> 分页提示仅当返回条数 = limit 时显示。

**输出示例 (json)**:
```json
[
  {"timestamp": "2026-03-01 18:30:00 CST", "content": "今天天气很好", "signature": "5UfD...xK3m"},
  {"timestamp": "2026-03-01 16:15:00 CST", "content": "Hello World", "signature": "3aBc...yZ9w"}
]
```

### 4.5 内容约束

- **仅支持文本**: 不支持图片、文件等非文本内容
- **大小限制**: Memo 最大 512 字节。扣除前缀 `HD:` 和加密开销，Zlib 压缩后的数据上限约 340 字节（对应明文约 500-800 字符）
- **加密方式**: Ed25519 → X25519 密钥转换 + NaCl box (Curve25519-XSalsa20-Poly1305)
- **数据压缩**: 使用 Zlib 算法压缩明文数据
- **加密开销**: 24 字节随机 nonce + 16 字节 Poly1305 MAC = 40 字节（nonce 必须每次随机生成，不可复用）
- **编码方式**: 加密后使用 Standard Base64 编码，并添加 `HD:` 前缀

### 4.6 MCP Server 模式

**功能描述**: 以 MCP Server 运行，通过 stdio 传输向 AI Agent 暴露日记读写工具。

**密码处理**: MCP 模式通过 stdio 通信，无法交互输入密码。若使用加密钱包文件，需通过以下方式提供密码：
- 环境变量 `HASH_DIARY_PASSWORD`（推荐，避免进程列表泄露密码）
- 启动参数 `--password <password>`（注意：命令行参数在 `ps` 进程列表中明文可见）
- 密码优先级：`--password` > `HASH_DIARY_PASSWORD` > 交互输入
- 若加密钱包未提供密码，启动时报错退出
- 无密码钱包可直接使用

**启动方式**:
```bash
hash-diary mcp
hash-diary mcp -u https://api.mainnet-beta.solana.com
hash-diary mcp --password <password>
HASH_DIARY_PASSWORD=<password> hash-diary mcp
```

**暴露的 MCP Tools**:

#### Tool: `diary_write`

向链上写入一条日记。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| text | string | 是 | 日记文本内容 |

返回值：交易签名字符串。

#### Tool: `diary_read`

读取链上日记记录。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| limit | number | 否 | 返回记录数量，默认 7 |
| since | string | 否 | 起始日期过滤，格式 YYYY-MM-DD |
| search | string | 否 | 关键词过滤 |
| before | string | 否 | 游标分页，返回指定交易签名之前的记录 |

返回值：日记条目数组，每条包含 `timestamp`、`content` 和 `signature` 字段。

**AI Agent 集成配置示例** (OpenClaw `~/.openclaw/workspace/skills/hash-diary/SKILL.md` 或 MCP 配置):
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

### 4.7 本地缓存机制 (新)

**功能描述**: 为了提高读取性能并减少 RPC 调用，HashDiary 将在本地维护一份已解密日记的缓存。

- **缓存文件**: `~/.hash-diary/cache.json` (或 SQLite)
- **缓存结构**:
  ```json
  {
    "wallet_address": {
      "last_signature": "5UfD...xK3m", // 最新已同步的交易签名
      "entries": [
        {"sig": "...", "ts": 1740825000, "content": "..."}
      ]
    }
  }
  ```
- **同步策略**:
  - `read` 时先读取缓存。
  - 调用 RPC `getSignaturesForAddress` 时带上 `until=<last_signature>` 参数，仅拉取增量交易。
  - 解密新交易并追加到缓存。
  - 若用户指定 `--force-refresh`，则忽略 `last_signature` 全量拉取并重建缓存。
- **并发控制**: 读写缓存文件时必须使用文件锁（File Lock），防止多终端并发操作导致数据损坏。
- **安全性**: 缓存文件存储的是明文日记内容，因此必须确保 `~/.hash-diary/` 目录及 `cache.json` 仅当前用户可读写 (chmod 600)。**警告：若本地环境被入侵，攻击者可直接读取缓存中的所有日记内容。**

## 5. 命令行接口设计

```
hash-diary <subcommand> [options]

子命令:
  wallet             钱包管理
  write [text]       将文本写入链上（可通过参数或 stdin 输入）
  read               读取链上日记记录
  mcp                以 MCP Server 模式运行

全局选项:
  -u, --url <rpc>    指定 Solana RPC 端点 URL (默认: https://api.devnet.solana.com)
  --keypair <path>   指定钱包密钥文件路径 (默认: ~/.hash-diary/id.json)
  --password <pwd>   解锁加密钱包的密码（优先级：--password > HASH_DIARY_PASSWORD 环境变量 > 交互输入）
  --format <fmt>     输出格式: text | json (默认: text)
  --verbose          显示详细日志（如 RPC 调用耗时、缓存命中情况）
  --help             显示帮助信息
  --version          显示版本号

write 选项:
  --no-wait          发送交易后立即返回，不等待确认 (默认: 等待 confirmed)

wallet 子命令:
  wallet new         创建新钱包密钥对
  wallet show        查看钱包地址和余额
  wallet airdrop     领取测试代币 (仅 devnet/testnet)
  wallet import      导入已有钱包

wallet 选项:
  --force            覆盖已有密钥文件（wallet new）
  --no-password      跳过密码设置，不加密密钥文件（wallet new / wallet import）
  --amount <n>       领取代币数量，默认 1 SOL（wallet airdrop）
  --private-key <k>  通过 Base58 私钥导入（wallet import）

read 选项:
  --limit <n>        限制返回记录数量 (默认: 7)
  --since <date>     起始日期过滤，格式 YYYY-MM-DD（按本地时区）
  --search <kw>      关键词搜索（本地过滤）
  --before <sig>     游标分页，返回指定交易签名之前的记录
  --force-refresh    忽略缓存，强制全量拉取链上数据

## 6. 技术要求

| 项目 | 说明 |
|------|------|
| 语言 | Go |
| 区块链 | Solana |
| 链上程序 | Memo Program (`MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr`) |
| 日记加密 | Zlib 压缩 + Ed25519 → X25519 密钥转换 + NaCl box |
| 密钥文件加密 | scrypt 密码派生 + AES-256-GCM |
| 编码格式 | `HD:` 前缀 + Standard Base64 (RFC 4648) |
| 单条上限 | 512 字节 (含前缀)，Zlib 压缩后数据约 340 字节 |
| 交易模式 | 自发自收 (目标地址 = 当前钱包地址) |
| 工具形态 | 命令行 CLI + MCP Server |
| 钱包路径 | `~/.hash-diary/id.json` |
| MCP 传输 | stdio |

## 7. 错误处理

| 场景 | 处理方式 |
|------|----------|
| 内容超过 512 字节 | 报错提示，告知当前大小及明文上限（约 344 字节），拒绝发送 |
| 钱包文件不存在 | 报错提示，引导用户执行 `wallet new` 创建或指定路径 |
| 钱包文件已存在（wallet new） | 提示确认覆盖，或使用 `--force` 跳过确认 |
| 钱包密码错误 | 报错提示，告知密码不正确，无法解锁密钥文件 |
| 加密钱包未提供密码 | 报错提示，告知需通过 `--password` 参数或 `HASH_DIARY_PASSWORD` 环境变量提供密码 |
| MCP 模式下加密钱包未提供密码 | 启动时报错退出，提示通过环境变量或 `--password` 参数提供密码 |
| 在 mainnet 上执行 airdrop | 报错提示，airdrop 仅支持 devnet/testnet |
| 导入的私钥格式无效 | 报错提示，告知支持的格式 |
| 余额不足 | 报错提示，告知当前余额和所需费用 |
| 网络连接失败 | 报错提示，建议检查网络或 RPC 端点 |
| write 未提供文本且无 stdin 输入 | 报错提示，告知需通过命令行参数或 stdin 提供日记内容 |
| 无历史记录 | 提示暂无日记记录 |
| MCP 协议错误 | 返回标准 MCP error response，包含错误码和描述 |

## 8. 非功能需求

- 交易发送后需等待确认（至少 confirmed 级别）再返回结果
- read 命令应按时间倒序展示记录，默认返回最近 7 条
- CLI 默认输出格式为 text（人类可读），可通过 `--format json` 切换为 JSON 格式（方便程序解析）
- `--format json` 模式下错误输出统一为 `{"error": "<错误描述>"}` 到 stdout，并以非零退出码退出
- 时间戳必须使用 Solana 区块时间 (Block Time)，确保不可篡改性。显示时转换为本地时间，格式为 `YYYY-MM-DD HH:MM:SS TZ`（如 `2026-03-01 18:30:00 CST`）；`--since` 参数按本地时区日期过滤
- 所有交互提示（密码输入、覆盖确认等）必须输出到 stderr，保持 stdout 干净，确保 `--format json` 和管道场景正常工作
- CLI 输出应简洁友好，支持中英文内容
- MCP Server 模式下所有输出必须为 JSON-RPC 格式，不得输出人类可读的提示信息到 stdout
- **费用说明**: 每条日记仅消耗 Solana 基础交易费（5000 lamports = 0.000005 SOL），在当前币价下几乎可忽略不计。
- **缓存安全**: 缓存文件应使用严格的文件权限（600/700），避免被其他用户读取。
