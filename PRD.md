# HashDiary (哈希日记) - 产品需求文档

## 1. 产品概述

HashDiary 是一个基于 Solana 区块链的命令行日记工具。用户通过 CLI 将文本日记写入链上 Memo 程序，实现内容的不可篡改存储，并可随时读取历史日记记录。

同时，HashDiary 提供 MCP (Model Context Protocol) Server 模式，使 OpenClaw 等 AI Agent 能够直接调用日记的读写能力，实现 AI 驱动的链上日记管理。

## 2. 核心概念

| 概念 | 说明 |
|------|------|
| 链上存储 | 利用 Solana Memo Program 将日记内容附加到交易中，永久存储在区块链上 |
| 自发自收交易 | 交易的目标地址是当前钱包自身，形成"给自己写信"的模式 |
| 公钥加密 | 日记内容使用钱包公钥加密后再进行 Base64 编码，仅持有对应私钥的钱包可解密 |
| Base64 编码 | 加密后的内容在发送前必须进行 Base64 编码 |
| 512 字节限制 | 单条 Memo 最大 512 字节（加密 + 编码后），超出需截断或拒绝 |
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

### 4.2 钱包管理

- 读取本地钱包密钥文件，**默认路径 `~/.hash-diary/id.json`**
- 支持通过参数指定密钥文件路径，例如 `--keypair /path/to/keypair.json`
- 钱包地址既作为交易发送方，也作为交易接收方（自发自收）

### 4.3 钱包管理子命令 (wallet)

**功能描述**: 管理 HashDiary 使用的 Solana 钱包，包括创建、查看、导入密钥对。

#### `wallet new` — 创建新钱包

生成新的 Solana 密钥对，保存到默认路径 `~/.hash-diary/id.json`。

**流程**:
1. 检查目标路径是否已存在密钥文件
2. 若已存在，提示用户确认是否覆盖（`--force` 可跳过确认）
3. 生成新的 Ed25519 密钥对
4. 将密钥对保存为 JSON 格式文件
5. 输出新钱包的公钥地址

**命令示例**:
```bash
hash-diary wallet new
hash-diary wallet new --keypair ./my-wallet.json
hash-diary wallet new --force
```

**输出示例**:
```
钱包已创建，地址: 7xKX...3nPq
密钥文件已保存至: ~/.hash-diary/id.json
```

#### `wallet show` — 查看钱包信息

显示当前钱包的公钥地址和 SOL 余额。

**命令示例**:
```bash
hash-diary wallet show
hash-diary wallet show -u https://api.mainnet-beta.solana.com
hash-diary wallet show --keypair ./my-wallet.json
```

**输出示例**:
```
地址: 7xKX...3nPq
余额: 1.5 SOL
网络: devnet
```

#### `wallet airdrop` — 领取测试代币

在 devnet/testnet 上领取测试用 SOL（仅限非 mainnet 网络）。

**命令示例**:
```bash
hash-diary wallet airdrop
hash-diary wallet airdrop --amount 2
```

**输出示例**:
```
已领取 1 SOL，当前余额: 2.5 SOL
```

#### `wallet import` — 导入已有钱包

从已有的密钥文件或私钥导入钱包。

**命令示例**:
```bash
hash-diary wallet import /path/to/existing-keypair.json
hash-diary wallet import --private-key <base58-private-key>
```

**输出示例**:
```
钱包已导入，地址: 9aBC...xY2z
密钥文件已保存至: ~/.hash-diary/id.json
```

### 4.4 写入子命令 (write)

**功能描述**: 将文本内容作为 Memo 写入 Solana 链上。

**流程**:
1. 用户输入文本内容
2. 使用钱包公钥对文本进行加密
3. 对加密后的内容进行 Base64 编码
4. 校验编码后大小不超过 512 字节，超出则报错提示
5. 构建交易：目标地址为当前钱包，附带 Memo 指令
6. 签名并发送交易
7. 返回交易签名（Transaction Signature）供用户查询

**命令示例**:
```bash
hash-diary write "今天天气很好"
hash-diary write "Hello World" -u https://api.mainnet-beta.solana.com
hash-diary write "日记内容" --keypair ./my-wallet.json
```

**输出示例**:
```
交易已发送，签名: 5UfD...xK3m
```

### 4.5 读取子命令 (read)

**功能描述**: 查询当前钱包地址的历史 Memo 交易，解码并展示日记内容。

**流程**:
1. 获取当前钱包地址的交易历史
2. 筛选包含 Memo 指令的交易
3. 对 Memo 数据进行 Base64 解码
4. 使用钱包私钥解密内容，解密失败的记录自动忽略（跳过）
5. 展示日记内容列表，包含时间戳和内容

**命令示例**:
```bash
hash-diary read
hash-diary read -u https://api.mainnet-beta.solana.com
hash-diary read --limit 10
hash-diary read --since 2026-02-01
```

**输出示例**:
```
[2026-03-01 10:30:00] 今天天气很好
[2026-03-01 08:15:00] Hello World
```

### 4.6 内容约束

- **仅支持文本**: 不支持图片、文件等非文本内容
- **大小限制**: Memo 最大 512 字节（加密 + Base64 编码后）
- **加密方式**: 使用钱包公钥加密，确保链上数据仅钱包持有者可读
- **编码方式**: 加密后统一使用 Base64 编码

### 4.7 MCP Server 模式

**功能描述**: 以 MCP Server 运行，通过 stdio 传输向 AI Agent 暴露日记读写工具。

**启动方式**:
```bash
hash-diary mcp
hash-diary mcp -u https://api.mainnet-beta.solana.com
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

返回值：日记条目数组，每条包含 `timestamp` 和 `content` 字段。

**AI Agent 集成配置示例** (OpenClaw `~/.openclaw/workspace/skills/hash-diary/SKILL.md` 或 MCP 配置):
```json
{
  "mcpServers": {
    "hash-diary": {
      "command": "hash-diary",
      "args": ["mcp"],
      "env": {}
    }
  }
}
```

## 5. 命令行接口设计

```
hash-diary <subcommand> [options]

子命令:
  wallet             钱包管理
  write <text>       将文本写入链上
  read               读取链上日记记录
  mcp                以 MCP Server 模式运行

全局选项:
  -u, --url <rpc>    指定 Solana RPC 端点 URL (默认: https://api.devnet.solana.com)
  --keypair <path>   指定钱包密钥文件路径 (默认: ~/.hash-diary/id.json)
  --help             显示帮助信息
  --version          显示版本号

wallet 子命令:
  wallet new         创建新钱包密钥对
  wallet show        查看钱包地址和余额
  wallet airdrop     领取测试代币 (仅 devnet/testnet)
  wallet import      导入已有钱包

wallet 选项:
  --force            覆盖已有密钥文件（wallet new）
  --amount <n>       领取代币数量，默认 1 SOL（wallet airdrop）
  --private-key <k>  通过 Base58 私钥导入（wallet import）

read 选项:
  --limit <n>        限制返回记录数量 (默认: 7)
  --since <date>     起始日期过滤，格式 YYYY-MM-DD
```

## 6. 技术要求

| 项目 | 说明 |
|------|------|
| 区块链 | Solana |
| 链上程序 | Memo Program (`MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr`) |
| 内容加密 | 钱包公钥加密 |
| 编码格式 | Base64 |
| 单条上限 | 512 字节 (加密 + 编码后) |
| 交易模式 | 自发自收 (目标地址 = 当前钱包地址) |
| 工具形态 | 命令行 CLI + MCP Server |
| 钱包路径 | `~/.hash-diary/id.json` |
| MCP 传输 | stdio |

## 7. 错误处理

| 场景 | 处理方式 |
|------|----------|
| 内容超过 512 字节 | 报错提示，拒绝发送 |
| 钱包文件不存在 | 报错提示，引导用户执行 `wallet new` 创建或指定路径 |
| 钱包文件已存在（wallet new） | 提示确认覆盖，或使用 `--force` 跳过确认 |
| 在 mainnet 上执行 airdrop | 报错提示，airdrop 仅支持 devnet/testnet |
| 导入的私钥格式无效 | 报错提示，告知支持的格式 |
| 余额不足 | 报错提示，告知当前余额和所需费用 |
| 网络连接失败 | 报错提示，建议检查网络或 RPC 端点 |
| 无历史记录 | 提示暂无日记记录 |
| MCP 协议错误 | 返回标准 MCP error response，包含错误码和描述 |

## 8. 非功能需求

- 交易发送后需等待确认（至少 confirmed 级别）再返回结果
- read 命令应按时间倒序展示记录，默认返回最近 7 条
- CLI 输出应简洁友好，支持中英文内容
- MCP Server 模式下所有输出必须为 JSON-RPC 格式，不得输出人类可读的提示信息到 stdout
