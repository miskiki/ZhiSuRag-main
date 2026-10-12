#!/usr/bin/env bash
#
# 打包 ZhiShu Lite Windows 桌面客户端（本机模式 + 连接服务器双模式）
#
# 用法（在仓库根目录、Git Bash 下执行）:
#   ./scripts/package-windows.sh                  # 完整构建
#   SKIP_FRONTEND=1 ./scripts/package-windows.sh  # 复用已有 web/（跳过前端构建）
#   CC=/path/to/gcc ./scripts/package-windows.sh  # 指定 C 编译器（CGO 需要）
#
# 产物:
#   dist/ZhiShuLite_<ver>_windows_amd64/          # 绿色版目录（exe + web + config + .env）
#   dist/ZhiShuLite_<ver>_windows_amd64.zip       # 发行压缩包

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

# ── 版本号 ──
if [ -n "${1:-}" ]; then
    VERSION="$1"
else
    VERSION=$(cat VERSION 2>/dev/null | tr -d '\n\r' || echo "dev")
fi

ARCHIVE="ZhiShuLite_${VERSION}_windows_amd64"
DIST_DIR="dist/${ARCHIVE}"
DESKTOP_DIR="cmd/desktop"

echo "=== ZhiShu Lite Windows Packager ==="
echo "  Version : ${VERSION}"
echo ""

# ── 前端产物 ──
if [ "${SKIP_FRONTEND:-0}" != "1" ]; then
    if [ -f frontend/package.json ]; then
        echo ">> Building frontend..."
        (cd frontend && npm ci --prefer-offline --no-audit --registry=https://registry.npmmirror.com && npm run build)
        rm -rf web
        cp -r frontend/dist web
    fi
elif [ ! -f web/index.html ] && [ -f frontend/dist/index.html ]; then
    echo ">> SKIP_FRONTEND=1: copying existing frontend/dist -> web/"
    rm -rf web
    cp -r frontend/dist web
fi
if [ ! -f web/index.html ]; then
    echo "WARNING: web/index.html missing — 本机模式将没有界面（远程模式不受影响）"
fi

# ── 桌面端构建（CGO: sqlite/sqlite-vec）──
echo ">> Building ZhiShu Lite desktop exe (windows/amd64)..."
if [ -n "${CC:-}" ]; then
    export CC
fi
export CGO_ENABLED=1
export CGO_CFLAGS="-Wno-deprecated-declarations"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

# ── sqlite3.h：sqlite-vec 的 CGO 绑定需要 ──
# Windows 没有系统 sqlite3.h；复用 go-sqlite3 模块自带的 amalgamation 头文件
# （与链接进二进制的 sqlite3 实现同源同版本）。
SQLITE_INC="${SQLITE_INC:-$(cygpath -m "$(mktemp -d)")/sqlite-include}"
mkdir -p "${SQLITE_INC}"
MATTN_DIR="$(go env GOMODCACHE)/github.com/mattn/go-sqlite3@$(go list -m -f '{{.Version}}' github.com/mattn/go-sqlite3)"
cp "${MATTN_DIR}/sqlite3-binding.h" "${SQLITE_INC}/sqlite3.h"
cp "${MATTN_DIR}/sqlite3ext.h" "${SQLITE_INC}/"
export CGO_CFLAGS="${CGO_CFLAGS} -I${SQLITE_INC}"

# ── DuckDB 预编译库的 emutls 符号补桩 ──
# duckdb-go-bindings 的静态库按 emulated-TLS 编译，引用 libstdc++ 的两个
# __emutls_v 控制符号（见 scripts/windows/emutls_stub.c）。MSYS2 gcc 的
# libstdc++ 未启用 emutls，需注入弱符号桩。必须通过 -extldflags 注入：
# 普通 .o 仅在最终链接出现一次且无条件参与链接；CGO_LDFLAGS 会按包重复
# 传递导致 PE/COFF 弱符号段冲突。
STUB_OBJ="${STUB_OBJ:-$(mktemp -d)/emutls_stub.o}"
gcc -O2 -c "${SCRIPT_DIR}/windows/emutls_stub.c" -o "${STUB_OBJ}"
EXTLD="-extldflags $(cygpath -m "${STUB_OBJ}")"

# ── 版本资源（图标/版本号/manifest）──
# wails 自身的 winres 步骤在本仓库不会填充字符串表，改用 go-winres 预生成
# .syso（wails build 链接它）。未安装 go-winres 时跳过（exe 缺版本信息但不影响功能）。
if command -v go-winres >/dev/null 2>&1; then
    (cd "${DESKTOP_DIR}" && go-winres make --in winres.json --out wails_windows)
else
    echo "WARNING: go-winres 未安装，跳过版本资源生成（go install github.com/tc-hib/go-winres@latest）"
fi

LDFLAGS="-s -w ${EXTLD}"
if [ -f ./scripts/get_version.sh ]; then
    LDFLAGS="-s -w ${EXTLD} $(./scripts/get_version.sh ldflags 2>/dev/null || true)"
fi

(cd "${DESKTOP_DIR}" && wails build -platform windows/amd64 -tags "sqlite_fts5" -ldflags "${LDFLAGS}")

EXE_PATH="${DESKTOP_DIR}/build/bin/ZhiShu Lite.exe"
if [ ! -f "${EXE_PATH}" ]; then
    echo "ERROR: build output not found: ${EXE_PATH}"
    exit 1
fi

# ── 组装发行目录 ──
echo ">> Assembling portable package..."
rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

cp "${EXE_PATH}" "${DIST_DIR}/ZhiShu Lite.exe"
if [ -d web ] && [ -f web/index.html ]; then
    cp -r web "${DIST_DIR}/web"
fi
if [ -d config ]; then
    cp -r config "${DIST_DIR}/config"
fi
# .env：本机模式运行时配置（Lite 零依赖形态）。远程模式不需要它。
cp .env.lite.example "${DIST_DIR}/.env"
# 说明文档
cat > "${DIST_DIR}/README-windows.txt" <<'EOF'
ZhiShu Lite 桌面客户端 (Windows)
================================

【启动】双击 "ZhiShu Lite.exe"。

【首次运行】
  - 选择「本机模式」：系统服务在本电脑内运行，数据保存在本机
    (%AppData%\ZhiShu Lite\)，适合个人使用。
  - 选择「连接服务器」：填入已部署系统的地址（如 http://192.168.1.100:8080），
    账号、知识库、会话等所有数据都在服务器上，多账号各自登录、相互隔离。

【切换模式/服务器】菜单栏 ZhiShu Lite → 服务器设置…

【本机模式所需】
  - web\ 目录（界面文件）与本 exe 同目录，请勿删除。
  - config\ 目录（服务配置）与本 exe 同目录。
  - .env 文件：模型服务地址/密钥等配置（编辑后重启应用生效）。

【常见问题】
  - 连不上服务器：检查服务器是否启动、防火墙是否放行其端口（默认 8080）。
  - 首次运行提示 SmartScreen：点「更多信息」→「仍要运行」。
EOF

echo ">> Creating zip..."
rm -f "dist/${ARCHIVE}.zip"
powershell -NoProfile -Command "Compress-Archive -Path 'dist/${ARCHIVE}\\*' -DestinationPath 'dist/${ARCHIVE}.zip' -Force" >/dev/null

echo ""
echo "=== DONE ==="
echo "  Folder: ${DIST_DIR}"
echo "  Zip   : dist/${ARCHIVE}.zip"
