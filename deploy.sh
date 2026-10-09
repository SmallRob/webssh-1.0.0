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

# ---- 持久配置防护（servers.json / presets.json）----
# 背景：这两个文件是管理员在页面上配置的运行时数据（bind mount 进容器），
# 曾被「全量 tar 上传解压」覆盖导致配置丢失。此处做三件事：
#   1) 部署前带时间戳备份到 /data/_backup；
#   2) 幂等预创建缺失文件（bind mount 源缺失时 docker 会自动建【目录】，
#      导致容器内该路径变成目录且应用读写失败）；
#   3) 部署后验证容器内挂载点仍是普通文件。
CONF_FILES=(servers.json presets.json)
BACKUP_DIR=/data/_backup
mkdir -p "${BACKUP_DIR}"
for f in "${CONF_FILES[@]}"; do
  if [ -f "${f}" ]; then
    cp -a "${f}" "${BACKUP_DIR}/${f}.$(date +%Y%m%d-%H%M%S).bak"
    # 仅保留最近 10 份备份（ls 无匹配时返回非零，pipefail 下须 || true 兜底）
    (ls -1t "${BACKUP_DIR}/${f}."*.bak 2>/dev/null || true) | tail -n +11 | xargs -r rm -f
  fi
  # JSON 合法性预检：损坏时尝试从最近备份恢复，仍失败则重置为空配置
  if [ -f "${f}" ] && ! python3 -c "import json,sys; json.load(open(sys.argv[1]))" "${f}" 2>/dev/null; then
    latest="$(ls -1t "${BACKUP_DIR}/${f}.*.bak" 2>/dev/null | head -1 || true)"
    if [ -n "${latest}" ] && python3 -c "import json,sys; json.load(open(sys.argv[1]))" "${latest}" 2>/dev/null; then
      echo "  [warn] ${f} JSON 损坏，已从 ${latest} 恢复"
      cp -a "${latest}" "${f}"
    else
      echo "  [warn] ${f} 缺失/损坏且无可用备份，重置为空配置"
      echo '[]' > "${f}"
    fi
  fi
done

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
    -v "$(pwd)/presets.json:/webssh/presets.json" \
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

# 持久配置挂载验证：容器内必须仍是普通文件（若被建成目录则立即告警）
for f in "${CONF_FILES[@]}"; do
  if docker exec "${CONTAINER}" sh -c "[ -f /webssh/${f} ]" 2>/dev/null; then
    echo "  持久配置 ${f}: OK"
  else
    echo "  [error] 持久配置 ${f} 在容器内不是普通文件！请检查挂载"
    exit 1
  fi
done
