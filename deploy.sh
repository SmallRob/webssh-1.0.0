#!/usr/bin/env bash
# WebSSH 一键构建 + 重启脚本（在项目根目录执行）
#
# 为什么需要固定 DOCKER_API_VERSION=1.44：
#   服务器上装的是 docker-compose 1.29（v1，Python 实现），重建容器时会读取镜像
#   inspect 返回的 ContainerConfig 字段；Docker Engine API >= 1.45 已移除该字段，
#   直接执行 docker-compose up -d 会抛 KeyError: 'ContainerConfig'，且此时旧容器
#   已被删除，服务会中断。而引擎的最低兼容 API 是 1.44（1.44 仍返回 ContainerConfig），
#   因此固定 1.44：既被守护进程接受，又能规避 compose v1 的该缺陷。
#
set -euo pipefail

cd "$(dirname "$0")"

IMAGE="${IMAGE:-webssh:1.0.0}"
CONTAINER="${CONTAINER:-webssh}"
PORT="${PORT:-8888}"
export DOCKER_API_VERSION="${DOCKER_API_VERSION:-1.44}"

echo "== 1/3 构建镜像 ${IMAGE} =="
docker build -t "${IMAGE}" .

echo "== 2/3 重建容器（DOCKER_API_VERSION=${DOCKER_API_VERSION}）=="
if ! docker-compose up -d; then
  echo "compose 重建失败，回退为 docker run ..."
  docker rm -f "${CONTAINER}" 2>/dev/null || true
  docker run -d --name "${CONTAINER}" --restart unless-stopped \
    -p "${PORT}:${PORT}" \
    -e "USER=${USER:-}" -e "PASS=${PASS:-}" -e "authInfo=" -e "PORT=${PORT}" \
    -v "$(pwd)/servers.json:/webssh/servers.json:ro" \
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
