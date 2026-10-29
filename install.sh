#!/usr/bin/env bash
# SecretDock 一键安装（远程执行）：
#   curl -fsSL https://raw.githubusercontent.com/pal537/SecretDock/main/install.sh | bash
# 行为：缺 Docker 就装 Docker（官方 get.docker.com），拉取预编译镜像并跑起来。
# 数据只存在 ./data（可用 SECRETDOCK_DATA 改位置），重装、升级都不会动它。
set -euo pipefail

IMAGE="${SECRETDOCK_IMAGE:-pal537/secretdock:latest}"
CONTAINER="${SECRETDOCK_CONTAINER:-secretdock}"
PORT="${SECRETDOCK_PORT:-8080}"
DATA_DIR="${SECRETDOCK_DATA:-$PWD/data}"

# ---- 找到可用的 docker 调用方式 ----
if ! command -v docker >/dev/null 2>&1; then
  echo "==> 未找到 Docker，正在安装（官方脚本）…"
  if [ "$(id -u)" -eq 0 ]; then
    curl -fsSL https://get.docker.com | sh
  elif command -v sudo >/dev/null 2>&1; then
    curl -fsSL https://get.docker.com | sudo sh
  else
    echo "错误: 未找到 Docker，且当前用户不是 root、也没有 sudo，请先手动安装 Docker 20.10+" >&2
    exit 1
  fi
fi

if docker ps >/dev/null 2>&1; then
  DOCKER="docker"
elif command -v sudo >/dev/null 2>&1 && sudo docker ps >/dev/null 2>&1; then
  DOCKER="sudo docker"
else
  echo "错误: 无法使用 Docker（可能需要把当前用户加进 docker 组，或用 root / sudo 运行）" >&2
  exit 1
fi

# ---- 拉取并启动 ----
mkdir -p "$DATA_DIR"
echo "==> 拉取镜像 ${IMAGE}…"
$DOCKER pull "$IMAGE"

if $DOCKER ps -a --format '{{.Names}}' 2>/dev/null | grep -qx "$CONTAINER"; then
  echo "==> 已有同名容器 ${CONTAINER}，先停掉重建（数据在 ${DATA_DIR}，不动）"
  $DOCKER stop "$CONTAINER" >/dev/null 2>&1 || true
  $DOCKER rm "$CONTAINER" >/dev/null 2>&1 || true
fi

echo "==> 启动 SecretDock（端口 ${PORT}，数据目录 ${DATA_DIR}）…"
$DOCKER run -d --name "$CONTAINER" \
  -p "127.0.0.1:${PORT}:8080" \
  -v "${DATA_DIR}:/data" \
  --restart unless-stopped \
  "$IMAGE" >/dev/null

echo "==> 等待服务就绪…"
for _ in $(seq 1 30); do
  if curl -sf -o /dev/null "http://127.0.0.1:${PORT}/"; then
    echo "==> 启动成功，访问: http://服务器IP:${PORT}"
    echo "==> 首次访问显示初始设置页，创建管理员账号（密码须 >10 位，含大写 + 小写 + 数字）；此页只出现一次。"
    echo "==> 数据目录: ${DATA_DIR}（备份它即可迁移；含解密主密钥，请妥善保管）"
    echo "==> 查看日志: ${DOCKER} logs -f ${CONTAINER}"
    exit 0
  fi
  sleep 2
done
echo "错误: 服务 60 秒内未就绪，请执行 ${DOCKER} logs ${CONTAINER} 查看日志" >&2
exit 1
