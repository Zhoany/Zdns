#!/bin/bash

# 要监控的程序名称
PROGRAM_NAME="zzdns"

# 日志文件路径
LOG_FILE="program_top_log.txt"

while true; do
    # 获取程序的进程 ID
    PID=$(pgrep  "$PROGRAM_NAME")

    # 检查是否获取到 PID
    if [ -z "$PID" ]; then
        echo "[$(date)] Unable to find process: $PROGRAM_NAME" >> "$LOG_FILE"
    else
        # 获取进程的 top 信息并记录到日志文件
        echo "[$(date)] Top information for process $PROGRAM_NAME (PID: $PID):" >> "$LOG_FILE"
        top -b -n 1 -p "$PID" >> "$LOG_FILE"
        echo "----------------------------------------" >> "$LOG_FILE"
    fi

    # 等待 30 秒
    sleep 30
done