# HashDiary - 开发任务清单

> 基于 PRD.md 分析生成，按模块和依赖关系排列。
> 状态标记：`[x]` 已完成 / `[-]` 有意跳过

---

## 阶段一：项目初始化与基础设施

### 1.1 项目脚手架

- [x] 初始化 Go module（`go mod init github.com/hash-diary/hash-diary`）
- [x] 确定目录结构：
  ```
  cmd/           # CLI 入口
  internal/
    wallet/      # 钱包管理（密钥生成、加密、导入导出）
    crypto/      # 加密模块（NaCl box、Zlib、Base64、协议前缀）
    cache/       # 本地缓存管理
    mcp/         # MCP Server 模式
    rpc/         # Solana RPC 客户端封装
    output/      # 输出格式化（text/json）
  tests/         # 集成测试
  ```
  > [-] `internal/memo/` 和 `internal/reader/` 合并到 `internal/crypto`（EncodeMemo/DecodeMemo）和 `cmd/read.go`
- [x] 添加 `.gitignore`（Go 二进制、IDE 文件、测试缓存等）
- [x] 添加 `Makefile`（build、test、test-verbose、lint、install、clean 目标）

### 1.2 依赖引入

- [x] Solana Go SDK — `github.com/gagliardetto/solana-go v1.14.0`
- [x] CLI 框架 — `github.com/spf13/cobra v1.10.2`
- [x] NaCl box / crypto 库 — `golang.org/x/crypto v0.48.0`
- [x] Ed25519 → X25519 转换库 — `filippo.io/edwards25519 v1.2.0`
- [x] scrypt 库 — `golang.org/x/crypto/scrypt`
- [x] Zlib — 标准库 `compress/zlib`
- [x] Base64 — 标准库 `encoding/base64`
- [x] 文件锁库 — `github.com/gofrs/flock v0.13.0`
- [x] MCP SDK — `github.com/mark3labs/mcp-go v0.44.1`

---

## 阶段二：核心加密模块 (`internal/crypto`)

### 2.1 密钥转换

- [x] 实现 Ed25519 私钥 → X25519 私钥转换函数
- [x] 实现 Ed25519 公钥 → X25519 公钥转换函数
- [x] 单元测试：使用已知向量验证转换正确性

### 2.2 Zlib 压缩/解压

- [x] 实现 `Compress(data []byte) ([]byte, error)` — Zlib 压缩
- [x] 实现 `Decompress(data []byte) ([]byte, error)` — Zlib 解压
- [x] 单元测试：压缩 → 解压 round-trip 验证
- [x] 单元测试：空数据、大数据、中文字符

### 2.3 NaCl Box 加密/解密

- [x] 实现 `Encrypt(plaintext, publicKey, privateKey) (ciphertext, error)`
  - 每次随机生成 24 字节 nonce
  - nonce 前置拼接到密文
- [x] 实现 `Decrypt(ciphertext, publicKey, privateKey) (plaintext, error)`
  - 从密文头部提取 24 字节 nonce
  - 解密失败返回明确错误
- [x] 单元测试：加密 → 解密 round-trip
- [x] 单元测试：错误密钥解密失败
- [x] 单元测试：篡改密文解密失败

### 2.4 编码管道（完整流水线）

- [x] 实现 `EncodeMemo(plaintext string, keypair) (string, error)`
  - 流程：UTF-8 → Zlib 压缩 → NaCl box 加密 → Base64 编码 → 添加 `HD:` 前缀
  - 校验最终长度 ≤ 512 字节，超出返回错误（包含当前大小信息）
- [x] 实现 `DecodeMemo(memo string, keypair) (string, error)`
  - 流程：去除 `HD:` 前缀 → Base64 解码 → NaCl box 解密 → Zlib 解压 → UTF-8 字符串
  - 任一步骤失败返回对应错误
- [x] 单元测试：完整 round-trip（中文、英文、混合内容）
- [x] 单元测试：无 `HD:` 前缀的 memo 返回错误
- [x] 单元测试：超过 512 字节限制的内容报错
- [x] 单元测试：验证加密开销计算（nonce 24 + MAC 16 = 40 字节）

---

## 阶段三：钱包管理 (`internal/wallet`)

### 3.1 明文钱包（Solana CLI 兼容格式）

- [x] 实现密钥对生成（Ed25519）
- [x] 实现明文钱包文件读取（JSON 数组，64 字节密钥对）
- [x] 实现明文钱包文件写入
- [x] 确保与 Solana CLI `solana-keygen` 格式互操作
- [x] 单元测试：生成 → 保存 → 读取 round-trip
- [x] 单元测试：读取 Solana CLI 生成的密钥文件

### 3.2 加密钱包（scrypt + AES-256-GCM）

- [x] 实现密码派生（scrypt，参数：N=32768, r=8, p=1）
- [x] 实现 AES-256-GCM 加密私钥
- [x] 实现 AES-256-GCM 解密私钥
- [x] 实现加密钱包文件格式读写：
  - `address`：明文公钥地址
  - `encrypted`：Base64 密文
  - `nonce`：Base64 AES-GCM nonce
  - `salt`：Base64 scrypt salt
  - `scrypt`：`{"N": 32768, "r": 8, "p": 1}`
- [x] 密码错误时返回明确错误信息
- [x] 单元测试：加密 → 解密 round-trip
- [x] 单元测试：错误密码解密失败

### 3.3 钱包加载器（统一接口）

- [x] 实现自动判断钱包文件格式（明文/加密）
- [x] 实现密码获取优先级：`--password` > `HASH_DIARY_PASSWORD` 环境变量 > 交互输入
- [x] 实现交互式密码输入（从 stderr 提示，读取 stdin）
- [x] 默认路径处理：`~/.hash-diary/id.json`
- [x] 钱包文件不存在时返回友好错误（引导 `wallet new`）

---

## 阶段四：Solana RPC 客户端 (`internal/rpc`)

### 4.1 RPC 封装

- [x] 封装 Solana RPC 客户端，支持自定义端点 URL
- [x] 默认端点：`https://api.devnet.solana.com`
- [x] 实现 `GetBalance(address)` — 查询 SOL 余额
- [x] 实现 `RequestAirdrop(address, amount)` — 请求空投（仅 devnet/testnet）
- [x] 实现 `SendTransaction(tx)` — 发送交易
- [x] 实现 `GetSignaturesForAddress(address, opts)` — 获取交易签名列表
  - 支持 `until`、`before`、`limit` 参数
- [x] 实现 `GetTransaction(signature)` — 获取交易详情（含 Memo 数据）
- [x] 实现交易确认等待（轮询，最长 60 秒，至 `confirmed` 级别）

### 4.2 网络容错

- [x] 发送交易失败重试：最多 3 次，退避间隔 500ms / 1s / 2s
- [x] RPC 限流处理：退避重试 + 友好提示
- [x] 网络断开检测与可操作建议输出
- [x] 确认超时处理：返回签名 + `timeout` 状态

---

## 阶段五：CLI 框架与全局选项 (`cmd/`)

### 5.1 CLI 入口

- [x] 实现主命令 `hash-diary`，注册子命令：`wallet`、`write`、`read`、`mcp`
- [x] 实现全局选项：
  - `-u, --url`：RPC 端点 URL
  - `--keypair`：密钥文件路径
  - `--password`：钱包密码
  - `--format`：输出格式（text / json）
  - `--verbose`：详细日志
  - `--help`：帮助信息
  - `--version`：版本号

### 5.2 输出格式化 (`internal/output`)

- [x] 实现 text 格式输出器（人类可读）
- [x] 实现 json 格式输出器
- [x] `--format json` 错误输出统一为 `{"error": "<描述>"}` 到 stdout
- [x] 所有交互提示（密码、确认）输出到 stderr
- [x] `--verbose` 日志输出到 stderr，且不泄露敏感信息（私钥路径、明文密码、日记内容）

---

## 阶段六：Wallet 子命令实现

### 6.1 `wallet new` — 创建新钱包

- [x] 检查目标路径是否存在密钥文件
- [x] 已存在时提示确认覆盖（`--force` 跳过）
- [x] 提示用户输入密码（可选，回车跳过；`--no-password` 直接跳过）
- [x] 生成 Ed25519 密钥对
- [x] 根据密码选择：有密码 → 加密保存；无密码 → 明文保存
- [x] 输出钱包地址和文件路径（text / json 格式）
- [x] 确保 `~/.hash-diary/` 目录存在，权限 700
- [x] 确保密钥文件权限 600

### 6.2 `wallet show` — 查看钱包信息

- [x] 加载钱包（支持密码解锁）
- [x] 查询链上 SOL 余额
- [x] 检测当前网络（devnet / testnet / mainnet-beta / 自定义）
- [x] 输出地址、余额、网络（text / json 格式）

### 6.3 `wallet airdrop` — 领取测试代币

- [x] 加载钱包获取地址
- [x] 检测网络，mainnet 上拒绝并报错
- [x] 请求空投（默认 1 SOL，`--amount` 自定义）
- [x] 查询空投后余额
- [x] 输出空投数量和当前余额（text / json 格式）

### 6.4 `wallet import` — 导入已有钱包

- [x] 支持从文件导入（参数为文件路径）
- [x] 支持从 Base58 私钥导入（`--private-key`）
- [x] 验证导入的密钥格式有效性
- [x] 提示用户输入密码（可选；`--no-password` 跳过）
- [x] 保存到目标路径（加密或明文）
- [x] 输出钱包地址和文件路径（text / json 格式）

---

## 阶段七：Write 子命令实现 (`cmd/write`)

### 7.1 输入处理

- [x] 解析命令行参数文本
- [x] 检测 stdin 输入（管道或重定向）
- [x] 优先级：命令行参数 > stdin
- [x] 未提供任何输入时报错

### 7.2 交易构建与发送

- [x] 调用 `EncodeMemo()` 对日记内容编码
- [x] 构建自发自收交易（目标地址 = 当前钱包地址）
- [x] 附加 Memo Program 指令（Program ID: `MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr`）
- [x] 签名交易
- [x] 发送交易（含失败重试逻辑）
- [x] 默认等待 `confirmed` 确认（`--no-wait` 跳过）
- [x] 确认超时处理（60 秒超时，返回签名 + `timeout` 状态）
- [x] 输出交易签名（text / json 格式）

---

## 阶段八：本地缓存系统 (`internal/cache`)

### 8.1 缓存文件管理

- [x] 定义缓存文件路径：`~/.hash-diary/cache.json`
- [x] 定义缓存 JSON 结构（cache_version + 按钱包地址隔离的条目）
- [x] 实现缓存文件读取
- [x] 实现缓存文件写入（原子写入：临时文件 + rename）
- [x] 文件权限设置：缓存文件 600，目录 700

### 8.2 并发控制

- [x] 实现文件锁（File Lock），防止多终端并发写入损坏

### 8.3 缓存迁移与损坏处理

- [x] `cache_version` 不匹配时：备份为 `cache.json.bak` → 全量重建
- [x] JSON 解析失败时：备份为 `cache.json.bak` → 全量重建

### 8.4 多钱包隔离

- [x] 按钱包地址作为一级键，不同钱包数据互不影响

---

## 阶段九：Read 子命令实现 (`cmd/read`)

### 9.1 增量同步

- [x] 读取本地缓存，获取 `last_signature`
- [x] 调用 `getSignaturesForAddress`，带 `until=<last_signature>` 拉取增量
- [x] 获取新交易详情，提取 Memo 数据
- [x] 筛选包含 `HD:` 前缀的交易
- [x] 调用 `DecodeMemo()` 解密新记录
- [x] 追加到本地缓存并更新 `last_signature`

### 9.2 异常处理

- [x] Base64 解码失败：跳过该记录（`--verbose` 提示）
- [x] NaCl 解密失败：跳过该记录（`--verbose` 提示）
- [x] Zlib 解压失败：跳过该记录（`--verbose` 提示）
- [x] `blockTime` 为空：跳过该记录

### 9.3 查询与过滤

- [x] `--limit <n>`：限制返回条数（默认 7）
- [x] `--since <date>`：按本地时区日期过滤
- [x] `--search <keyword>`：关键词搜索（本地过滤）
- [x] `--before <sig>`：游标分页（返回严格早于该签名的记录）
- [x] 默认按时间倒序排列
- [x] 时间戳相同时按签名字典序倒序保证稳定输出

### 9.4 输出与分页

- [x] text 格式：`[YYYY-MM-DD HH:MM:SS TZ] 内容`
- [x] json 格式：`[{"timestamp": "...", "content": "...", "signature": "..."}]`
- [x] 当返回条数 = limit 时显示分页提示：`-- 更多记录: hash-diary read --before <last_sig>`
- [x] `--force-refresh`：忽略缓存，全量拉取重建

### 9.5 时间戳处理

- [x] 使用 Solana 区块时间（Block Time）作为时间源
- [x] 显示时转换为本地时区
- [x] 格式：`YYYY-MM-DD HH:MM:SS TZ`

---

## 阶段十：MCP Server 模式 (`internal/mcp`)

### 10.1 MCP Server 基础

- [x] 实现 stdio 传输的 MCP Server
- [x] 启动时加载钱包（处理密码：`--password` > `HASH_DIARY_PASSWORD` > 报错退出）
- [x] 加密钱包未提供密码时，启动报错退出
- [x] stdout 仅输出 JSON-RPC 格式，不输出人类可读信息

### 10.2 Tool: `diary_write`

- [x] 参数：`text`（string，必填）
- [x] 调用 write 逻辑写入链上
- [x] 成功返回交易签名
- [x] 失败返回带错误码的错误信息

### 10.3 Tool: `diary_read`

- [x] 参数：`limit`（number，默认 7）、`since`（string）、`search`（string）、`before`（string）
- [x] 调用 read 逻辑读取日记
- [x] 返回日记条目数组（`timestamp`、`content`、`signature`）

### 10.4 错误码定义

- [x] 定义业务错误码范围：`-32000` 到 `-32099`
- [x] 常见错误码映射：
  - `-32001`：wallet locked（钱包未解锁）
  - `-32002`：content too large（内容超限）
  - `-32003`：insufficient balance（余额不足）
  - `-32004`：network error（网络错误）

---

## 阶段十一：错误处理完善

- [x] 内容超过 512 字节：报错提示当前大小及明文上限
- [x] 钱包文件不存在：引导 `wallet new`
- [x] 钱包文件已存在（wallet new）：提示确认覆盖
- [x] 钱包密码错误：明确提示密码不正确
- [x] 加密钱包未提供密码：提示通过 `--password` 或环境变量提供
- [x] mainnet airdrop：报错提示仅支持 devnet/testnet
- [x] 导入私钥格式无效：报错提示支持的格式
- [x] 余额不足：提示当前余额和所需费用
- [x] 网络连接失败：建议检查网络或 RPC 端点
- [x] write 未提供文本：提示通过参数或 stdin 提供
- [x] 无历史记录：提示暂无日记
- [x] 交易确认超时：返回签名 + timeout 状态
- [x] 解码/解密失败：跳过记录，`--verbose` 提示
- [x] RPC 限流/超时：退避重试 + 建议更换 RPC

---

## 阶段十二：测试

### 12.1 单元测试

- [x] `internal/crypto`：密钥转换、压缩、加密、编码管道（37+ 测试）
- [x] `internal/wallet`：密钥生成、明文/加密文件读写、密码验证（46+ 测试）
- [x] `internal/cache`：缓存读写、版本迁移、损坏恢复、文件锁（26+ 测试）
- [x] `internal/output`：text / json 格式化（16 测试）
- [x] `internal/mcp`：MCP 工具参数验证、错误码、服务器构造（19 测试）
- [x] `internal/rpc`：RPC 封装、重试逻辑、网络检测（31+ 测试）

### 12.2 集成测试 (`tests/`)

- [x] 钱包生命周期测试（明文/加密 round-trip、错误密码）
- [x] 加密钱包完整流程（创建 → 加密保存 → 解密加载 → 用于 EncodeMemo/DecodeMemo）
- [x] Crypto → Cache round-trip（编解码 + 缓存存取验证）
- [x] Read 过滤逻辑（排序、--since、--search、--before 游标、--limit、组合过滤）
- [x] `--force-refresh` 全量重建验证（含多钱包隔离）
- [x] Base58 私钥导入 → 保存 → 加载验证
- [x] 完整管道测试（钱包 → 加密 → 缓存 → 过滤）
- [-] write → read round-trip（devnet）— 骨架已就绪，需 `go test -tags integration` 运行

### 12.3 边界测试

- [x] 最大长度内容写入（接近 512 字节限制）
- [x] 超长内容写入（超出限制报错）
- [x] 空内容写入
- [x] 特殊字符（emoji、换行、制表符、null 字节）
- [x] 并发多终端读写缓存（10 goroutine 并发测试）

---

## 阶段十三：构建与发布

- [x] 编写 `Makefile`（build、test、test-verbose、lint、install、clean）
- [x] 确保 `go build` 生成 `hash-diary` 二进制
- [x] 更新 `README.md`（安装、使用说明、示例、MCP 集成、技术细节）
- [x] 添加 LICENSE 文件（MIT）

---

## 依赖关系图

```
阶段一（项目初始化）        ✅
  ↓
阶段二（核心加密模块）      ✅
  ↓
阶段三（钱包管理）          ✅  ← 依赖阶段二（密钥转换）
  ↓
阶段四（RPC 客户端）        ✅
  ↓
阶段五（CLI 框架）          ✅  ← 依赖阶段三、四
  ↓
阶段六（Wallet 子命令）     ✅  ← 依赖阶段三、四、五
阶段七（Write 子命令）      ✅  ← 依赖阶段二、四、五
  ↓
阶段八（缓存系统）          ✅
  ↓
阶段九（Read 子命令）       ✅  ← 依赖阶段二、四、五、八
  ↓
阶段十（MCP Server）        ✅  ← 依赖阶段七、九
  ↓
阶段十一（错误处理完善）    ✅  ← 贯穿所有阶段
  ↓
阶段十二（测试）            ✅  ← 贯穿所有阶段
  ↓
阶段十三（构建与发布）      ✅
```

---

## 技术备忘

| 项 | 值 |
|---|---|
| 语言 | Go |
| Memo Program ID | `MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr` |
| 协议前缀 | `HD:` |
| 加密方案 | NaCl box (Curve25519-XSalsa20-Poly1305) |
| 密钥转换 | Ed25519 → X25519 |
| 压缩算法 | Zlib |
| 编码 | Standard Base64 (RFC 4648) |
| 密钥文件加密 | scrypt (N=32768, r=8, p=1) + AES-256-GCM |
| Memo 上限 | 512 字节（含 `HD:` 前缀） |
| 加密开销 | 24 字节 nonce + 16 字节 MAC = 40 字节 |
| 默认 RPC | `https://api.devnet.solana.com` |
| 钱包路径 | `~/.hash-diary/id.json` |
| 缓存路径 | `~/.hash-diary/cache.json` |
| 默认 read limit | 7 条 |
| 交易确认超时 | 60 秒 |
| 发送重试 | 3 次，退避 500ms / 1s / 2s |
| MCP 错误码 | -32001 wallet locked / -32002 content too large / -32003 insufficient balance / -32004 network error |
