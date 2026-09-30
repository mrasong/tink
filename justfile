# Tink 任务运行配置文件 (just)

# 默认列出所有任务
default:
    @just --list

# 构建所有组件 (Server 与 Client)
build: build-server build-client

# 构建 Go 后端服务到 build/tink-server
build-server:
    ./scripts/build_server.sh

# 按当前 Docker 主机架构构建 Docker 镜像
build-docker image="tink:latest":
    ./scripts/build_docker.sh "" {{ image }}

# 构建 x86_64 Docker 镜像
build-docker-amd64 image="tink:latest":
    ./scripts/build_docker.sh "" {{ image }} linux/amd64

# 构建并推送 ARM64 + x86_64 多架构镜像
build-docker-multi image="tink:latest":
    ./scripts/build_docker.sh "" {{ image }} linux/amd64,linux/arm64

# 构建并打包 macOS 客户端到 build/Tink.app
build-client:
    ./scripts/build_app.sh

# 运行后端测试
test:
    cd server && go test -v ./...

# 运行服务端 (支持自定义端口与数据目录: just run-server port=5021 data=./data)
run-server port="5021" data="./data" args="":
    mkdir -p {{ data }}
    @if [ ! -f ./build/tink-server ]; then ./scripts/build_server.sh; fi
    ./build/tink-server serve --port {{ port }} --data {{ data }} {{ args }}

# 查看所有 Token (自动支持在线/离线双模)
token-list args:
    ./build/tink-server token list {{ args }}

# 创建新 Token (例: just token-create name="ci-key")
token-create args:
    ./build/tink-server token create {{ args }}

# 查看已注册设备 (自动支持在线/离线双模)
device-list args:
    ./build/tink-server device list {{ args }}

# 运行/打开 macOS 客户端
run-client:
    @if [ ! -d ./build/universal/Tink.app ]; then ./scripts/build_app.sh; fi
    open build/universal/Tink.app

# Docker Compose 启动服务
docker-up:
    docker compose up -d

# Docker Compose 停止服务
docker-down:
    docker compose down

# 发送通知 (例: just send title="Deployment" body="Done" url="https://example.com" group="deploy")
send title="Tink Notification" body="Test message from Tink" url="" sound="default" group=env("TINK_GROUP", "") token=env("TINK_TOKEN", "") server=env("TINK_SERVER", "http://localhost:5021"):
    ./scripts/send.sh --server "{{ server }}" --token "{{ token }}" --title "{{ title }}" --body "{{ body }}" --url "{{ url }}" --sound "{{ sound }}" --group "{{ group }}"

# 运行 Web 前端开发服务器 (热更新调试)
web-dev:
    cd server/web && pnpm dev

# 清理构建产物与缓存
clean:
    rm -rf build/ client/.build/ server/internal/web/dist/
