#!/bin/bash

# =========================
# 定义变量
# =========================
APP_NAME="zzdns"
CONTAINER_NAME="zzdnsv6-container"
DOCKER_IMAGE="zzdnsv6:latest"

# =========================
# 帮助信息
# =========================
function show_help {
    echo "用法: $0 {start|stop|restart|status|logs|remove}"
    echo
    echo "管理 ${APP_NAME} 的运行容器:"
    echo "  start      启动容器"
    echo "  stop       停止容器"
    echo "  restart    重启容器"
    echo "  status     查看容器状态"
    echo "  logs       查看容器日志"
    echo "  remove     移除容器"
    echo
}

# =========================
# 启动容器
# =========================
function start_container {
    echo "启动 ${CONTAINER_NAME}..."
    docker run -d --name ${CONTAINER_NAME} \
        -p 5300:53/udp \
        -p 5300:53/tcp \
        -v "$(pwd)/cfg.data:/app/cfg.data" \
        -v "$(pwd)/logs/:/app/logs" \
        -v /etc/localtime:/etc/localtime:ro \
        -v /etc/timezone:/etc/timezone:ro \
        ${DOCKER_IMAGE}

    if [ $? -eq 0 ]; then
        echo "${CONTAINER_NAME} 启动成功。"
    else
        echo "${CONTAINER_NAME} 启动失败。"
        exit 1
    fi
}

# =========================
# 停止容器
# =========================
function stop_container {
    echo "停止 ${CONTAINER_NAME}..."
    docker stop ${CONTAINER_NAME}
    if [ $? -eq 0 ]; then
        echo "${CONTAINER_NAME} 已停止。"
    else
        echo "无法停止 ${CONTAINER_NAME}，请检查容器是否运行中。"
    fi
}

# =========================
# 重启容器
# =========================
function restart_container {
    echo "重启 ${CONTAINER_NAME}..."
    stop_container
    start_container
}

# =========================
# 查看容器状态
# =========================
function container_status {
    echo "查看 ${CONTAINER_NAME} 状态..."
    docker ps -a | grep ${CONTAINER_NAME}
    if [ $? -ne 0 ]; then
        echo "${CONTAINER_NAME} 未运行。"
    fi
}

# =========================
# 查看容器日志
# =========================
function container_logs {
    echo "查看 ${CONTAINER_NAME} 日志..."
    docker logs ${CONTAINER_NAME}
}

# =========================
# 移除容器
# =========================
function remove_container {
    echo "移除 ${CONTAINER_NAME}..."
    docker rm -f ${CONTAINER_NAME}
    if [ $? -eq 0 ]; then
        echo "${CONTAINER_NAME} 已移除。"
    else
        echo "无法移除 ${CONTAINER_NAME}，请检查容器是否存在或已停止。"
    fi
}

# =========================
# 脚本主逻辑
# =========================
case "$1" in
    start)
        start_container
        ;;
    stop)
        stop_container
        ;;
    restart)
        restart_container
        ;;
    status)
        container_status
        ;;
    logs)
        container_logs
        ;;
    remove)
        remove_container
        ;;
    *)
        show_help
        ;;
esac