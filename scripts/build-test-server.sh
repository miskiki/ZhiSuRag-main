#!/usr/bin/env bash
# 组装一个本机测试用的 ZhiShu Lite "远程服务器"（cmd/server + SQLite + 本地存储）
# 用途：桌面客户端"连接服务器"模式的端到端测试
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TEST_DIR="${TEST_DIR:-/c/Users/intel/zhishu-test-server}"

cd "$ROOT_DIR"
export PATH="/c/Users/intel/build-tools/go/bin:/c/Users/intel/build-tools/msys64/msys64/ucrt64/bin:$PATH"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export CGO_ENABLED=1 CC=gcc
export CGO_CFLAGS="-Wno-deprecated-declarations"

# sqlite3.h include（sqlite-vec 需要）
SQLITE_INC="/c/Users/intel/build-tools/sqlite-include"
export CGO_CFLAGS="-I$(cygpath -m "$SQLITE_INC") ${CGO_CFLAGS}"

mkdir -p "$TEST_DIR"
echo ">> Building ZhiShu-lite server binary..."
go build -tags "sqlite_fts5" -o "$TEST_DIR/ZhiShu-lite.exe" ./cmd/server

echo ">> Assembling server dir: $TEST_DIR"
rm -rf "$TEST_DIR/web" "$TEST_DIR/config" "$TEST_DIR/migrations"
cp -r frontend/dist "$TEST_DIR/web"
cp -r config "$TEST_DIR/config"
[ -d migrations/sqlite ] && mkdir -p "$TEST_DIR/migrations" && cp -r migrations/sqlite "$TEST_DIR/migrations/"

# .env：Lite 零依赖形态；端口固定 8080，注册开放
cat > "$TEST_DIR/.env" <<'EOF'
GIN_MODE=debug
LOG_LEVEL=info
DB_DRIVER=sqlite
DB_PATH=./data/zhishu.db
RETRIEVE_DRIVER=sqlite
STORAGE_TYPE=local
LOCAL_STORAGE_BASE_DIR=./data/files
STREAM_MANAGER_TYPE=memory
TENANT_AES_KEY=test-local-aes-key-32chars-long!!!!
JWT_SECRET=test-local-jwt-secret-not-for-prod
NEO4J_ENABLE=false
ZHISHU_SANDBOX_MODE=disabled
ENABLE_GRAPH_RAG=false
DISABLE_REGISTRATION=false
APP_PORT=8080
EOF

echo ">> Server ready. Start with:"
echo "   cd $TEST_DIR && ./ZhiShu-lite.exe   # 监听 :8080"
