# bcli

`bcli` 是一个个人命令中心，用来统一承载数据库客户端入口、Redis 客户端入口，以及常用的小工具。

## 构建

```bash
go build -o bcli .
```

## 使用

```bash
./bcli help
./bcli mysql auth --profile local
./bcli mysql --profile local -- -e "select 1"
./bcli redis auth --profile cache
./bcli redis --profile cache -- ping
./bcli tools uuid
./bcli tools now
./bcli tools urlencode "a b"
./bcli tools base64 encode hello
```

## 配置

默认配置文件路径：

```text
~/.bcli/config.json
```

也可以通过环境变量指定：

```bash
BCLI_CONFIG=/path/to/config.json ./bcli mysql --profile local
```

示例：

```json
{
  "mysql": {
    "local": {
      "executable": "mysql",
      "args": ["-h", "127.0.0.1", "-P", "3306", "-u", "root"]
    }
  },
  "redis": {
    "cache": {
      "executable": "redis-cli",
      "args": ["-h", "127.0.0.1", "-p", "6379"]
    }
  }
}
```

不要把密码、token、证书等敏感信息写进仓库。MySQL 和 Redis 的密码优先使用本机客户端支持的环境变量、交互输入或本机安全配置。

## 认证信息

密码通过系统凭据库保存。macOS 下会写入 Keychain，不会写入 `~/.bcli/config.json`。

```bash
./bcli mysql auth --profile local
./bcli redis auth --profile cache
```

也可以直接传入密码：

```bash
./bcli mysql auth --profile local "password"
./bcli redis auth --profile cache "password"
```

直接传参可能进入 shell history。日常使用更推荐省略密码，让 `bcli` 通过隐藏输入读取。

执行客户端时，`bcli` 会按 profile 读取凭据，并只注入到子进程环境：

```text
mysql: MYSQL_PWD
redis: REDISCLI_AUTH
```

## 当前命令

```text
bcli mysql [--profile name] [-- mysql args...]
bcli mysql auth [--profile name] [password]
bcli redis [--profile name] [-- redis-cli args...]
bcli redis auth [--profile name] [password]
bcli tools uuid
bcli tools now
bcli tools urlencode <text>
bcli tools urldecode <text>
bcli tools base64 encode <text>
bcli tools base64 decode <text>
bcli tools sha256 <text>
```
