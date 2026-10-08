# Docker Compose 启动

当前 `docker-compose.yml` 会启动三个服务：

```text
mysql   MySQL 8.4 数据库
server  聊天室服务端，连接 mysql:3306
client  命令行客户端，可按需启动
```

启动服务端和 MySQL：

```bash
docker compose up -d --build mysql server
```

查看日志：

```bash
docker compose logs -f server
```

启动一个客户端：

```bash
docker compose run --rm client
```

停止并保留数据库数据：

```bash
docker compose down
```

停止并删除数据库数据卷：

```bash
docker compose down -v
```

Compose 中服务端通过环境变量开启数据库：

```text
CHATROOM_DB_ENABLE=true
CHATROOM_DB_DSN=chatroom:456+@tcp(mysql:3306)/chatroom?charset=utf8mb4&parseTime=true&loc=Local
CHATROOM_DB_AUTO_MIGRATE=true
```

MySQL 首次启动会创建 `chatroom` 数据库，服务端启动后会自动创建用户、角色、权限、菜单相关表。

数据库名和服务端使用的账号都是 `chatroom`，密码是 `456+`；root 密码也是 `456+`。连接串里的 `mysql:3306` 表示数据库服务名和容器内端口。`456+` 中的加号在此连接串中直接写即可。

`ports` 是“电脑端口:容器端口”，`build` 指定 Go 镜像如何构建，`depends_on` 让服务端等待数据库就绪，`volumes` 保存数据库数据。Compose 自动建立网络，无需单独配置 networks。

如果 mysql_data 卷已经初始化过，修改 Compose 中的密码不会修改已有数据库账号的密码，需要在原数据库中修改账号密码后再启动服务端。不要为了改密码删除已有数据卷。
