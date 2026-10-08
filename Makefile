# Chatroom Makefile
# 构建和运行聊天室项目

.PHONY: all build run-server run-client clean vet test

# 默认目标
all: build

# 编译
build:
	@echo "=== 构建聊天室 ==="
	go build -o build/chat-server.exe ./cmd/server/
	go build -o build/chat-client.exe ./cmd/client/
	@echo "✅ 构建完成: build/chat-server.exe, build/chat-client.exe"

# 运行服务器
run-server:
	go run ./cmd/server/

# 运行客户端
run-client:
	go run ./cmd/client/

# 代码检查
vet:
	go vet ./...

# 清理
clean:
	@echo "=== 清理构建产物 ==="
	rm -rf build/
	@echo "✅ 清理完成"

# 测试 (占位)
test:
	go test ./...

# 格式化代码
fmt:
	go fmt ./...

# 开发模式 (同时启动服务器，需要另一个终端运行客户端)
dev:
	@echo "🚀 启动开发服务器..."
	go run ./cmd/server/
