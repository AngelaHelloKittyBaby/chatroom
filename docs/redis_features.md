# Redis 功能与运行说明

## 启动

```sh
docker compose up -d --build mysql redis server
docker compose run --rm client
```

Compose 增加 Redis 7.4、AOF、独立数据卷和健康检查；服务器等待 MySQL 与 Redis 健康后启动。Redis 映射到宿主机 `127.0.0.1:6379`。

本地运行：先启动 MySQL 和 Redis，再执行 `go run ./cmd/server` 和 `go run ./cmd/client`。`config.yaml` 默认启用两种存储。Redis 开关关闭时，聊天使用原来的内存投递路径，排名不可用，用户资料直接查询 MySQL。

## 客户端命令

- `/rank`：查看前 10 名；`/rank 20` 可指定数量（1–100）。
- `/profile`：查看当前账号 ID、用户名，需要启用 MySQL 认证。
- `/msg 用户名 消息`：异步私聊。
- 普通文字：异步群聊。
- `/help`：查看命令。

## 实现与边界

活跃度使用 Sorted Set `chatroom:activity`，按用户名累计。通过验证且成功入队的群聊、私聊各计 1 分，查询命令、无效消息不计分。排名查询按分数降序，同分按 Redis 的成员逆字典序排列。

消息队列使用 Stream `chatroom:messages` 和消费组 `delivery`。Lua 将入队和计分放在同一次执行中。后台消费者处理投递后 ACK 并删除 Stream 条目，避免已完成消息无限累积；启动时先处理同一消费者 `server` 的 pending 消息，再读取新消息。消费失败会等待后重试，未确认消息保留在 Redis。Redis 不可用时入队返回错误，不静默转为内存投递。

当前架构针对单台聊天服务器，固定消费组/消费者不支持多个聊天服务器共享同一前缀。消息交给房间/在线连接发送缓冲区后即可确认，未实现客户端接收确认、离线收件箱或历史消息存储。进程崩溃可能导致重复投递，也可能丢失已经交给内存发送缓冲区但尚未送达客户端的消息。接收方离线时私聊会通知发送者。Redis AOF 默认每秒同步，机器故障仍可能丢失最近写入。

用户资料缓存使用 `chatroom:user:<ID>`，默认 TTL 300 秒，仅存 ID 和用户名，不存密码或密码哈希。成功认证后更新缓存；`/profile` 优先读取缓存，缓存未命中、损坏或 Redis 不可用时回查 MySQL 并尝试回填。认证始终访问 MySQL，不通过资料缓存跳过密码校验。将来增加用户资料修改接口时，需要同步删除或更新该缓存。

## 配置

`config.yaml` 的 `redis` 节支持：`enable`、`addr`、`password`、`db`、`prefix`、`cache_ttl_sec`。

对应环境变量（优先于文件）：

```text
CHATROOM_REDIS_ENABLE=true
CHATROOM_REDIS_ADDR=127.0.0.1:6379
CHATROOM_REDIS_PASSWORD=
CHATROOM_REDIS_DB=0
CHATROOM_REDIS_PREFIX=chatroom:
CHATROOM_REDIS_CACHE_TTL_SEC=300
```

启用 Redis 后，连接失败会阻止服务器启动。不同实例应使用独立前缀或数据库。

## 验证

```sh
go test ./...
go vet ./...
docker compose config --quiet
```

集成测试会在随机本地端口启动独立的 `redis-server` 子进程，使用临时目录，结束后清理。测试环境需要 PATH 中有 `redis-server`，否则跳过这些集成测试。覆盖真实 TCP 群聊与私聊、身份防伪、排名计分、参数检查、pending 消息恢复、ACK 清理、缓存 TTL、缓存命中以及故障回源。
