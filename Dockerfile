# 使用 Alpine 作为基础镜像
FROM alpine:latest

# 创建应用目录
RUN mkdir -p /app

# 复制应用程序到 /app 目录
COPY target/zzdns /app/myapp

# 设置工作目录
WORKDIR /app

# 暴露 53 端口，用于 DNS 服务
EXPOSE 53/udp
EXPOSE 53/tcp

# 设置默认命令
CMD ["./myapp"]
