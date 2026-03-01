# hash-diary
HashDiary (哈希日记)

需求：
1. Solana，默认是devnet，用户可以指定mainnet或testnet
2. 存储方案memo，最多发送512字节的内容，必须使用base64编码
3. 发送交易的目标地址是当前钱包

要求：
1. 命令行工具，提供读和写的子命令
2. 只支持写文本
