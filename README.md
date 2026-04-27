# bcli

`bcli` 是一个个人命令中心，用来统一承载数据库客户端入口、Redis 客户端入口，以及常用的小工具。

当前项目通过 `go.mod` 和 `.tool-versions` 固定 Go 版本为 `1.24.2`，影响范围限定在本仓库。

## 构建

```bash
go build -o bcli .
go build -o bcli ./cmd/bcli
```

## 使用

```bash
./bcli help
./bcli init
./bcli
./bcli tui
./bcli profile list --json
./bcli profile set mysql local --host 127.0.0.1 --port 3306 --user root --database app
./bcli profile get mysql local --json
./bcli auth mysql --profile local
./bcli mysql --profile local -- -e "select 1"
./bcli auth redis --profile cache
./bcli redis --profile cache -- ping
./bcli tools uuid
./bcli tools now
./bcli tools urlencode "a b"
./bcli tools base64 encode hello
```

## 项目结构

项目已经按正式 MCP 路线拆分入口层、核心层和存储层：

```text
cmd/bcli                  进程入口
internal/cli              CLI 参数解析、输出格式和退出码
internal/core/auth        认证核心能力
internal/core/profile     profile 核心能力
internal/core/external    外部客户端执行
internal/core/tools       小工具核心能力
internal/storage          配置文件和系统凭据库适配
internal/tui              人类操作的终端界面
internal/mcp              MCP Server 适配层
```

`internal/mcp` 提供 `bcli mcp serve`，通过 stdio JSON-RPC 暴露 MCP tools、resources 和 prompts，并复用 `internal/core` 的能力。

## MCP

启动 MCP Server：

```bash
./bcli mcp serve
```

第一批 MCP tools：

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

第一批 MCP resources：

```text
bcli://profiles
bcli://profiles/mysql
bcli://profiles/redis
bcli://profiles/mysql/{name}
bcli://profiles/redis/{name}
bcli://config/paths
```

第一批 MCP prompts：

```text
bcli.prompt.create_mysql_profile
bcli.prompt.create_redis_profile
bcli.prompt.inspect_profiles
bcli.prompt.rotate_profile_password
```

MCP tools 和 resources 只返回 profile 的非敏感字段，以及 `hasCredential` 状态；不会返回已保存的密码。

## 配置

推荐使用 TUI 管理连接配置：

```bash
./bcli
./bcli tui
```

TUI 是一个终端 command center，不只管理 profile：

- Home 展示 MySQL/Redis 客户端状态、profile 数量和认证状态概览。
- Profiles 支持新增、编辑、删除 MySQL/Redis profile，并统一维护认证状态。连接参数写入配置文件，密码写入系统凭据库。
- Tools 提供 UUID、当前时间、URL encode/decode、Base64 encode/decode、SHA256 等常用工具。

首次进入 TUI 且还没有外部 CLI 配置时，`bcli` 会先进入初始化向导。也可以随时手动运行：

```bash
./bcli init
```

初始化向导是一个上下移动、空格多选、回车确认的 TUI。它会让你选择要启用的外部 CLI，例如 MySQL 或 Redis；如果本机没有对应客户端，会进入安装选择页，让你继续用多选框选择是否安装。

在新增或编辑 profile 的表单里，可以按 `ctrl+t` 测试当前输入的连接配置。测试时 MySQL 会执行 `select 1`，Redis 会执行 `ping`；如果表单里填了密码，会优先使用本次输入的密码，否则使用已保存凭据。测试不会把密码写入输出。

新建 profile 时，TUI 会优先使用初始化向导保存的客户端路径；如果没有保存路径，会自动探测本机的 `mysql` 或 `redis-cli`。探测顺序包括当前 `PATH` 和 macOS Homebrew 常见安装路径；如果找不到，测试连接会按当前系统给出安装提示。

默认配置目录：

```text
~/.bcli/configs/
```

连接 profile 默认保存到：

```text
~/.bcli/configs/connections.json
```

也可以通过环境变量指定：

```bash
BCLI_CONFIG=/path/to/connections.json ./bcli mysql --profile local
```

示例：

```json
{
  "mysql": {
    "local": {
      "host": "127.0.0.1",
      "port": 3306,
      "user": "root",
      "database": "app"
    }
  },
  "redis": {
    "cache": {
      "host": "127.0.0.1",
      "port": 6379,
      "user": "default",
      "database": "0"
    }
  }
}
```

`executable` 和 `args` 仍然可用，适合少量高级参数；常规 host、port、user、database 优先使用结构化字段。

不要把密码、token、证书等敏感信息写进仓库或 `~/.bcli/configs/connections.json`。

## 认证信息

密码通过系统凭据库保存。macOS 下会写入 Keychain，不会写入 `~/.bcli/configs/connections.json`。

```bash
./bcli auth mysql --profile local
./bcli auth redis --profile cache
```

也可以直接传入密码：

```bash
./bcli auth mysql --profile local "password"
./bcli auth redis --profile cache "password"
```

直接传参可能进入 shell history。日常使用更推荐省略密码，让 `bcli` 通过隐藏输入读取。

执行客户端时，`bcli` 会按 profile 读取凭据，并只注入到子进程环境：

```text
mysql: MYSQL_PWD
redis: REDISCLI_AUTH
```

## 当前命令

```text
bcli
bcli init
bcli auth mysql [--profile name] [password]
bcli auth redis [--profile name] [password]
bcli profile list [--json]
bcli profile get <mysql|redis> <name> [--json]
bcli profile set <mysql|redis> <name> [--host host] [--port port] [--user user] [--database database] [--executable path] [--arg value...]
bcli profile delete <mysql|redis> <name>
bcli mysql [--profile name] [-- mysql args...]
bcli redis [--profile name] [-- redis-cli args...]
bcli tui
bcli mcp serve
bcli tools uuid
bcli tools now
bcli tools urlencode <text>
bcli tools urldecode <text>
bcli tools base64 encode <text>
bcli tools base64 decode <text>
bcli tools sha256 <text>
```
