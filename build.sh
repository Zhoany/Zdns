#!/bin/bash

# =========================
# 1. 定义项目和应用名称
# =========================
APP_NAME="zzdns"
CONTAINER_NAME="zzdnsv6-container"

# =========================
# 2. 检测本地已有的 zzdnsv6 镜像版本
# =========================

echo "尝试获取本地已有的 zzdnsv6 镜像版本..."
OLD_VERSION=$(docker images --format "{{.Repository}}:{{.Tag}}" \
    | grep -E "^zzdnsv6:" \
    | grep -v ":latest" \
    | awk -F: '{print $2}' \
    | sort -rV \
    | head -n1)

if [ -z "$OLD_VERSION" ]; then
    echo "未发现已有 zzdnsv6 镜像，使用默认版本 1.0.0"
    OLD_VERSION="1.0.0"
else
    echo "检测到本地最高版本号: $OLD_VERSION"
fi

# 拆分版本号
IFS='.' read -r MAJOR MINOR PATCH <<< "${OLD_VERSION}"
if [ -z "${MAJOR}" ] || [ -z "${MINOR}" ] || [ -z "${PATCH}" ]; then
    echo "版本号格式不合法, 使用默认版本 1.0.0"
    MAJOR=1
    MINOR=0
    PATCH=0
fi

PATCH=$((PATCH+1))
NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"
echo "即将构建新版本号: ${NEW_VERSION}"

DOCKER_IMAGE_VERSIONED="zzdnsv6:${NEW_VERSION}"
DOCKER_IMAGE_LATEST="zzdnsv6:latest"

# =========================
# 3. 编译 Go 应用
# =========================
echo "编译 ${APP_NAME}.go..."
mkdir -p target
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o target/${APP_NAME} "${APP_NAME}.go"

if [ $? -ne 0 ]; then
    echo "编译失败，请检查代码。"
    exit 1
fi
echo "编译完成。"


# =========================
# 4. 生成 Dockerfile
# =========================
cat <<EOF > Dockerfile
# 使用 Alpine 作为基础镜像
FROM alpine:latest

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


# =========================
# 5. 生成 docker-compose.yml
# =========================
cat <<EOF > docker-compose.yml
version: '3'
services:
  myapp:
    build: .
    image: ${DOCKER_IMAGE_VERSIONED}
    container_name: ${CONTAINER_NAME}
    ports:
      - "5300:53/udp"
      - "5300:53/tcp"
    volumes:
      - ./cfg.data:/app/cfg.data
      - /etc/localtime:/etc/localtime:ro
      - /etc/timezone:/etc/timezone:ro
EOF


# =========================
# 6. 构建并标记 Docker 镜像
# =========================
echo "构建 Docker 镜像: ${DOCKER_IMAGE_VERSIONED} ..."
docker compose build

if [ $? -ne 0 ]; then
    echo "Docker 镜像构建失败。"
    exit 1
fi
echo "Docker 镜像构建完成。"

docker tag "${DOCKER_IMAGE_VERSIONED}" "${DOCKER_IMAGE_LATEST}"

# =========================
# 7. 保存镜像（便于移植）
# =========================
SAVE_FILE="zzdnsv6_${NEW_VERSION}.tar"
echo "保存镜像到文件 ${SAVE_FILE} ..."
docker save -o "${SAVE_FILE}" "${DOCKER_IMAGE_VERSIONED}"

if [ $? -ne 0 ]; then
    echo "保存镜像失败。"
    exit 1
fi
echo "镜像已保存到 ${SAVE_FILE}"
echo "可使用 'docker load -i ${SAVE_FILE}' 在其他环境加载该镜像"


# =========================
# 8. 启动 Docker Compose 服务（用于测试），稍后关闭
# =========================
echo "启动 Docker Compose 服务进行测试..."
docker compose up -d

if [ $? -ne 0 ]; then
    echo "Docker Compose 启动失败。"
    exit 1
fi

# 这里示例：休眠10秒再关闭
echo "正在进行测试，等待 10 秒后关闭容器..."
sleep 10

# 如需查看容器日志，可在此加上:
# docker compose logs myapp

echo "测试完毕，关闭并移除容器与网络..."
docker compose down

echo "==========================================="
echo "  测试已完成，容器已关闭并移除"
echo "  新构建镜像版本号: ${NEW_VERSION}"
echo "  本地保存文件: ${SAVE_FILE}"
echo "==========================================="