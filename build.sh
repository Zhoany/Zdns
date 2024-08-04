#!/bin/bash

# 定义项目目录和应用名
PROJECT_DIR=$(pwd)
APP_NAME="zzdns"
DOCKER_IMAGE_NAME="zzdns"
DOCKER_CONTAINER_NAME="zzdns-container"

# 编译 zzdns.go
echo "编译 ${APP_NAME}.go..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o target/${APP_NAME} ${APP_NAME}.go

if [ $? -ne 0 ]; then
    echo "编译失败，请检查代码。"
    exit 1
fi
echo "编译完成。"

# 创建 Dockerfile
cat <<EOF > Dockerfile
# 使用 Alpine 作为基础镜像
FROM alpine:latest

# 设置时区为上海
RUN apk add --no-cache tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && apk del tzdata

# 安装必要的依赖（如果有）
RUN apk add --no-cache ca-certificates

# 创建应用目录
RUN mkdir -p /app

# 复制应用程序到 /app 目录
COPY target/${APP_NAME} /app/myapp

# 设置工作目录
WORKDIR /app

# 暴露 53 端口，用于 DNS 服务
EXPOSE 53/udp
EXPOSE 53/tcp

# 设置默认命令
CMD ["./myapp"]
EOF

# 创建 docker-compose.yml
cat <<EOF > docker-compose.yml
services:
  myapp:
    build: .
    image: ${DOCKER_IMAGE_NAME}
    container_name: ${DOCKER_CONTAINER_NAME}
    ports:
      - "5300:53/udp"
      - "5300:53/tcp"
    volumes:
      - ./cfg.data:/app/cfg.data
    environment:
      - TZ=Asia/Shanghai
EOF

# 构建 Docker 镜像
echo "构建 Docker 镜像..."
docker compose build

if [ $? -ne 0 ]; then
    echo "Docker 镜像构建失败。"
    exit 1
fi
echo "Docker 镜像构建完成。"

# 启动 Docker Compose 服务
echo "启动 Docker Compose 服务..."
docker compose up -d

if [ $? -ne 0 ]; then
    echo "Docker Compose 启动失败。"
    exit 1
fi
echo "Docker Compose 服务已启动。"