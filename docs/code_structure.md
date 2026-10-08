# 代码结构说明

这次用户持久化相关代码按职责拆成了三层：

```text
internal/auth
  password.go        密码哈希和密码校验
  store.go           用户存储接口、用户模型、认证错误

internal/database
  mysql.go           连接 MySQL、关闭连接、自动建表
  schema.go          用户/角色/权限/菜单的建表 SQL 和种子数据
  user_store.go      查询用户、创建用户、校验密码、分配默认角色

internal/server
  server.go          聊天服务器流程：收到 join 后调用 userStore 做登录/注册
```

启动流程：

1. `cmd/server/main.go` 读取 `config.yaml`。
2. `internal/server.NewServer` 判断 `database.enable`。
3. 如果启用数据库，就调用 `database.OpenMySQL`。
4. 用户加入聊天室时，服务端调用 `userStore.AuthenticateOrCreate`。
5. MySQL 中没有该用户就注册，有该用户就校验密码。

这样拆分后：

- `auth` 不直接操作数据库。
- `database` 专门负责 MySQL 和 SQL。
- `server` 只负责聊天业务流程。
