#!/usr/bin/env bash
# SecretDock 一键部署脚本（配合 DEPLOY.md 使用）
# 用法: ./deploy.sh
# 说明: 编译并启动 SecretDock，首次启动约 1-2 分钟。
set -euo pipefail
cd "$(dirname "$0")"

command -v docker >/dev/null 2>&1 || { echo "错误: 未找到 docker，请先安装 Docker 20.10+" >&2; exit 1; }
docker compose version >/dev/null 2>&1 || { echo "错误: 未找到 docker compose v2" >&2; exit 1; }

export APP_VERSION="${APP_VERSION:-$(date +%Y%m%d%H%M)}"
echo "==> 编译并启动 SecretDock (APP_VERSION=${APP_VERSION})"
docker compose up -d --build

echo "==> 等待服务就绪..."
for i in $(seq 1 30); do
  if curl -sf -o /dev/null "http://127.0.0.1:${HOST_PORT:-8080}/"; then
    echo "==> 启动成功，访问: http://服务器IP:${HOST_PORT:-8080}"
    echo "==> 首次访问会显示初始设置页，按 DEPLOY.md 创建管理员账号。"
    exit 0
  fi
  sleep 2
done
echo "错误: 服务 60 秒内未就绪，请执行 docker compose logs 查看日志" >&2
exit 1
