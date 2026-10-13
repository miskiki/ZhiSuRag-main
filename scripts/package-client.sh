#!/usr/bin/env bash
#
# 打包智枢文渊精简客户端（仅连接服务器模式，无内嵌后端）
#
# 用法（仓库根目录、Git Bash）:
#   ./scripts/package-client.sh
#
# 产物:
#   dist/ZhiShuClient_<ver>_windows_amd64/ZhiShu Client.exe   # 单文件绿色客户端
#   dist/ZhiShuClient_<ver>_windows_amd64.zip                 # 发行压缩包

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

VERSION="${1:-$(cat VERSION 2>/dev/null | tr -d '\n\r' || echo dev)}"
ARCHIVE="ZhiShuClient_${VERSION}_windows_amd64"
DIST_DIR="dist/${ARCHIVE}"
CLIENT_DIR="cmd/desktop-client"

echo "=== ZhiShu Client Packager（精简客户端）==="
echo "  Version : ${VERSION}"
echo ""

export PATH="/c/Users/intel/build-tools/go126/bin:/c/Users/intel/build-tools/msys64/msys64/ucrt64/bin:/c/Users/intel/go/bin:$PATH"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export CGO_ENABLED=1 CC="${CC:-gcc}"
export CGO_CFLAGS="-Wno-deprecated-declarations"

# 版本资源（图标/版本号）
if command -v go-winres >/dev/null 2>&1; then
    (cd "${CLIENT_DIR}" && go-winres make --in winres.json --out wails_windows) || true
else
    echo "WARNING: go-winres 未安装，跳过版本资源（go install github.com/tc-hib/go-winres@latest）"
fi

# emutls 链接桩（DuckDB 不在精简客户端里，但保持与完整版一致的构建参数无害）
STUB_OBJ="${STUB_OBJ:-$(mktemp -d)/emutls_stub.o}"
gcc -O2 -c "${SCRIPT_DIR}/windows/emutls_stub.c" -o "${STUB_OBJ}"

echo ">> Building ZhiShu Client.exe..."
(cd "${CLIENT_DIR}" && wails build -platform windows/amd64 -skipbindings -s \
    -tags "sqlite_fts5" \
    -ldflags "-s -w -X main.clientVersion=${VERSION} -extldflags $(cygpath -m "${STUB_OBJ}")")

EXE_PATH="${CLIENT_DIR}/build/bin/ZhiShu Client.exe"
[ -f "${EXE_PATH}" ] || { echo "ERROR: ${EXE_PATH} not found"; exit 1; }

echo ">> Assembling package..."
rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"
cp "${EXE_PATH}" "${DIST_DIR}/ZhiShu Client.exe"

cat > "${DIST_DIR}/README-client.txt" <<'EOF'
智枢文渊客户端（精简版 · 连接服务器模式）
==========================================

【是什么】单文件绿色客户端，约 12MB，无需安装。
  它连接您部门部署的智述系统服务器使用服务：
  账号、知识库、配置全部保存在服务器上，与网页端完全一致。

【使用】
  1. 双击 "ZhiShu Client.exe"
  2. 首次使用填入服务器地址（浏览器访问系统用的地址），
     点「测试连接」显示 ✓ 后「保存并连接」
  3. 登录 / 注册 / 退出，与网页端完全一致

【常见问题】
  - 首次运行 SmartScreen 提示：更多信息 → 仍要运行
  - 提示缺少运行组件：按弹窗指引安装 WebView2（一次性）
  - 无法连接：确认服务器已启动、防火墙已放行端口
  - 启动无反应：把 exe 旁自动生成的 startup-error.log 发给管理员
EOF

cd dist
rm -f "${ARCHIVE}.zip"
powershell -NoProfile -Command "Compress-Archive -Path '${ARCHIVE}\\*' -DestinationPath '${ARCHIVE}.zip' -Force"

echo ""
echo "=== DONE ==="
echo "  Folder: ${DIST_DIR}"
echo "  Zip   : dist/${ARCHIVE}.zip"
