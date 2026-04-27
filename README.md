# bcli

`bcli` 是一个个人命令中心，用来统一承载数据库客户端入口、Redis 客户端入口，以及常用的小工具。

当前项目通过 `go.mod` 和 `.tool-versions` 固定 Go 版本为 `1.24.2`，影响范围限定在本仓库。

## 构建

```bash
go build -o bcli .
```

## 使用

```bash
./bcli help
./bcli
./bcli tui
./bcli auth mysql --profile local
./bcli mysql --profile local -- -e "select 1"
./bcli auth redis --profile cache
./bcli redis --profile cache -- ping
./bcli tools uuid
./bcli tools now
./bcli tools urlencode "a b"
./bcli tools base64 encode hello
```

## 配置

推荐使用 TUI 管理连接配置：

```bash
./bcli
./bcli tui
```

TUI 支持新增、编辑、删除 MySQL/Redis profile，并统一维护认证状态。连接参数写入配置文件，密码写入系统凭据库。

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
bcli auth mysql [--profile name] [password]
bcli auth redis [--profile name] [password]
bcli mysql [--profile name] [-- mysql args...]
bcli redis [--profile name] [-- redis-cli args...]
bcli tui
bcli tools uuid
bcli tools now
bcli tools urlencode <text>
bcli tools urldecode <text>
bcli tools base64 encode <text>
bcli tools base64 decode <text>
bcli tools sha256 <text>
```
