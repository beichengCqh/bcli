# bcli MCP 正式路线项目规划

## 目标

`bcli` 要同时服务两类使用者：

- 人类：通过 TUI 管理本地 profile、cache、认证状态和常用工具。
- AI / vibe-coding 工具：通过 CLI 和 MCP 使用结构化能力，不依赖自然语言猜测命令。

正式路线是新增 `bcli mcp serve`，让 `bcli` 自身作为 MCP Server 暴露 tools、resources、prompts。CLI、TUI、MCP 三个入口必须复用同一套核心服务，避免各自维护一份业务逻辑。

## 设计原则

- 密码、token、证书等敏感信息只进入系统凭据库，不写入 JSON 配置、日志、MCP resource 或 tool 返回。
- 配置目录统一放在 `~/.bcli/` 下，连接配置默认放在 `~/.bcli/configs/connections.json`。
- CLI 面向脚本和 AI，输出必须可机器解析，关键命令支持 `--json`。
- TUI 面向人类操作，允许更强交互，但不能绕过核心服务直接改配置或凭据。
- MCP 只是一层协议适配，不拥有业务规则。

## 目标目录结构

```text
.
├── cmd/
│   └── bcli/
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── command.go
│   │   ├── auth.go
│   │   ├── profile.go
│   │   ├── external.go
│   │   ├── mcp.go
│   │   └── tools.go
│   ├── core/
│   │   ├── auth/
│   │   │   ├── service.go
│   │   │   └── credential_store.go
│   │   ├── profile/
│   │   │   ├── service.go
│   │   │   ├── model.go
│   │   │   └── args.go
│   │   ├── external/
│   │   │   └── service.go
│   │   └── tools/
│   │       └── service.go
│   ├── storage/
│   │   ├── config_store.go
│   │   ├── paths.go
│   │   └── keyring_store.go
│   ├── tui/
│   │   └── app.go
│   └── mcp/
│       ├── server.go
│       ├── tools.go
│       ├── resources.go
│       └── prompts.go
├── md/
├── README.md
├── go.mod
└── go.sum
```

## 模块职责

### `cmd/bcli`

只保留进程入口：

- 创建默认依赖。
- 调用 `internal/cli`。
- 不放业务逻辑。

### `internal/cli`

负责命令行协议：

- `bcli` 进入 TUI。
- `bcli tui` 进入 TUI。
- `bcli auth mysql|redis ...` 写入认证信息。
- `bcli profile list|get|set|delete ...` 管理非敏感连接信息。
- `bcli mysql|redis ...` 调用外部客户端。
- `bcli mcp serve` 启动 MCP Server。
- `bcli tools ...` 执行个人小工具。

CLI 层只解析参数、格式化输出、转换退出码。实际动作委托给 core service。

### `internal/core`

承载业务能力，供 CLI、TUI、MCP 共用。

- `core/profile`：管理 profile，负责结构化连接参数、默认 profile、命令参数生成。
- `core/auth`：管理认证信息，负责保存、读取、删除 credential。
- `core/external`：负责执行 `mysql`、`redis-cli` 等外部客户端。
- `core/tools`：负责 uuid、now、base64、sha256、url encode/decode 等小工具。

这一层不关心 TUI、MCP、命令行文本格式。

### `internal/storage`

承载基础设施适配：

- config JSON 文件读写。
- `~/.bcli` 路径解析。
- `BCLI_CONFIG` 覆盖逻辑。
- Keychain / keyring 凭据实现。

### `internal/tui`

只负责终端交互：

- 列表、表单、删除确认。
- 调用 core service 读写 profile/auth。
- 不直接 `os.ReadFile` / `os.WriteFile`。
- 不直接操作 keyring。

### `internal/mcp`

负责 MCP 协议适配：

- `server.go`：stdio MCP server 生命周期。
- `tools.go`：注册和执行 MCP tools。
- `resources.go`：暴露只读资源。
- `prompts.go`：暴露可复用工作流 prompt。

MCP 层调用 core service，禁止绕过 core 直接访问 storage。

## 命令规划

### 保留

```text
bcli
bcli tui
bcli auth mysql [--profile name] [password]
bcli auth redis [--profile name] [password]
bcli mysql [--profile name] [-- mysql args...]
bcli redis [--profile name] [-- redis-cli args...]
bcli tools ...
```

### 新增

```text
bcli mcp serve
bcli profile list [--json]
bcli profile get <mysql|redis> <name> [--json]
bcli profile set <mysql|redis> <name> [flags...]
bcli profile delete <mysql|redis> <name>
```

`profile` 命令只操作非敏感配置。密码仍然只能走 `auth` 或 TUI 的认证字段。

## MCP Tools 规划

第一阶段先暴露稳定且低风险的能力：

```text
bcli.profile.list
bcli.profile.get
bcli.profile.set
bcli.profile.delete
bcli.auth.mysql
bcli.auth.redis
bcli.tools.uuid
bcli.tools.now
bcli.tools.base64_encode
bcli.tools.base64_decode
bcli.tools.sha256
bcli.tools.urlencode
bcli.tools.urldecode
```

第二阶段再暴露可能触发外部系统操作的能力：

```text
bcli.mysql.exec
bcli.redis.exec
```

外部执行类 tool 默认要求显式参数，不从 prompt 中拼 shell 字符串，避免命令注入和错误转义。

## MCP Resources 规划

```text
bcli://profiles
bcli://profiles/mysql
bcli://profiles/redis
bcli://profiles/mysql/{name}
bcli://profiles/redis/{name}
bcli://config/paths
```

resource 返回内容必须去敏：

- 可以返回 kind、name、host、port、user、database、executable、args。
- 可以返回 `hasCredential: true|false`。
- 禁止返回 password、token、Keychain item 内容。

## MCP Prompts 规划

```text
bcli.prompt.create_mysql_profile
bcli.prompt.create_redis_profile
bcli.prompt.inspect_profiles
bcli.prompt.rotate_profile_password
```

prompts 只提供工作流模板，不承载真实业务逻辑。

## 迁移阶段

### 第一阶段：无行为变化重构

- 把 `internal/app` 拆成 `cli/core/storage/tui`。
- 保持所有现有命令行为不变。
- 测试目标：现有测试全部通过。

### 第二阶段：补齐 profile CLI

- 新增 `bcli profile list|get|set|delete`。
- 支持 `--json` 输出，供 AI 和脚本稳定读取。
- 测试目标：profile CRUD、JSON 输出、敏感信息不落盘。

### 第三阶段：接入 MCP Server

- 新增 `bcli mcp serve`。
- 实现 stdio transport。
- 注册第一批 MCP tools/resources/prompts。
- 测试目标：tools list、tool call、resource read、敏感信息不暴露。

### 第四阶段：外部客户端 MCP 能力

- 暴露 `bcli.mysql.exec` 和 `bcli.redis.exec`。
- 明确参数 schema，不接受未结构化 shell 字符串。
- 测试目标：参数转义、credential 注入、退出码和错误返回。

## 推荐下一步

先执行第一阶段和第二阶段。这样即使 MCP Server 还没接完，`bcli` 也已经具备稳定的机器接口，后续 MCP 只是把同一批 core service 暴露出去。
