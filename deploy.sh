#!/usr/bin/env bash
# WebSSH 一键构建 + 重启脚本（在项目根目录执行）
#
# 兼容性说明（重要）：
#   服务器装的是 docker-compose 1.29（v1，Python 实现），它「原地重建」容器时
#   会读取镜像 inspect 的 ContainerConfig 字段；Docker Engine API >= 1.45 已移除
#   该字段，因此 docker-compose up -d 在重建场景必抛 KeyError: 'ContainerConfig'，
#   且此时旧容器已被删除，服务中断。实测即使 export DOCKER_API_VERSION=1.44
#   也无法绕过（compose 实际仍以引擎默认 API 版本请求）。
#
#   稳妥做法：先显式删除旧容器，让 compose 走「全新创建」路径（不读旧镜像配置，
#   不会触发该缺陷）；万一 compose 仍失败，则回退为等价的 docker run。
#
set -euo pipefail

cd "$(dirname "$0")"

IMAGE="${IMAGE:-webssh:1.0.0}"
CONTAINER="${CONTAINER:-webssh}"
PORT="${PORT:-8888}"

echo "== 1/3 构建镜像 ${IMAGE} =="
docker build -t "${IMAGE}" .

echo "== 2/3 重建容器（先删旧容器，避开 compose v1 的 ContainerConfig 缺陷）=="
docker rm -f "${CONTAINER}" 2>/dev/null || true
if ! docker-compose up -d; then
  echo "compose 创建失败，回退为 docker run ..."
  docker rm -f "${CONTAINER}" 2>/dev/null || true
  docker run -d --name "${CONTAINER}" --restart unless-stopped \
    -p "${PORT}:${PORT}" \
    -e "authInfo=" -e "PORT=${PORT}" \
    -e "adminPass=${ADMIN_PASS:-}" \
    -e "rdpRequireAdmin=${RDP_REQUIRE_ADMIN:-true}" \
    -e "vncRequireAdmin=${VNC_REQUIRE_ADMIN:-false}" \
    -v "$(pwd)/servers.json:/webssh/servers.json" \
    --network tcb-front-nginx-network \
    "${IMAGE}"
fi

echo "== 3/3 健康检查 =="
for _ in $(seq 1 20); do
  status="$(docker inspect -f '{{.State.Health.Status}}' "${CONTAINER}" 2>/dev/null || echo unknown)"
  echo "  健康状态: ${status}"
  [ "${status}" = "healthy" ] && break
  sleep 3
done

docker ps --filter "name=${CONTAINER}" --format '{{.Names}} | {{.Image}} | {{.Status}} | {{.Ports}}'
curl -s -o /dev/null -w "  /check -> %{http_code}\n" "http://127.0.0.1:${PORT}/check"
curl -s -o /dev/null -w "  /protocols -> %{http_code}\n" "http://127.0.0.1:${PORT}/protocols"
