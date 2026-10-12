# 智述 Lite · Windows 桌面客户端 — 交付说明

> 版本：v0.7.2（随仓库 VERSION）｜ 构建日期：2026-10-12 ｜ 平台：Windows 10/11 x64

## 一、这是什么

一个双模式 Windows 桌面客户端（Wails/WebView2 壳 + Go 桥接），替代"浏览器打开网页"的使用方式：

- **连接服务器模式（本项目目标形态）**：一台电脑部署智述系统服务，其他电脑用本客户端填入服务器地址即可使用。账号、注册/登录、知识库、会话等全部数据保存在服务器上，多账号各自登录、相互隔离。
- **本机模式（附赠能力）**：系统服务直接内嵌在客户端里本机运行（SQLite + 本地文件存储，零外部依赖），适合个人单机使用。

## 二、发行内容（dist/ZhiShuLite_0.7.2_windows_amd64/）

```
ZhiShu Lite.exe    主程序（双模式客户端 + 可选内嵌服务）
web\               本机模式的界面文件（连接服务器模式不需要）
config\            本机模式的服务配置
.env               本机模式运行配置（模型服务地址/密钥等）
README-windows.txt 快速使用说明
```

连接服务器模式的最终用户**只需要 exe 本身**，web/config/.env 仅本机模式需要。

## 三、服务器端准备（管理员做一次）

1. 服务器上按现有方式部署智述系统（现有 docker-compose 部署即可，无需任何改动）。
2. 放行服务端口（默认 8080）：
   - Windows 服务器：`netsh advfirewall firewall add rule name="ZhiShu" dir=in action=allow protocol=TCP localport=8080`
   - Linux 服务器：`firewall-cmd --add-port=8080/tcp --permanent && firewall-cmd --reload`（或 ufw/安全组放行）
3. 确认服务器防火墙/云安全组对该网段（如 192.168.x.x）开放。
4. **分发给同事的地址 = 平时浏览器访问系统用的地址**（前端端口，docker compose 默认映射 FRONTEND_PORT=80，即 `http://服务器IP`；
   ⚠️ 不是 8080——8080 是纯 API 后端，客户端直连它只会看到 JSON 报错。已在实测中确认此区别。）

## 四、客户端使用（每位用户）

1. 解压发行包，双击 `ZhiShu Lite.exe`（绿色软件，无需安装）。
2. 首次启动出现"启动设置"：
   - 选 **连接服务器** → 填入服务器地址 → 点 **测试连接**（显示"✓ 连接成功"）→ **保存并启动**；
   - 应用自动重启后即进入服务器上的智述系统登录页。
3. 登录 / 注册 / 退出登录与网页版完全一致（同一套系统、同一套账号）。
4. 首次运行如提示 Windows SmartScreen：点 **更多信息 → 仍要运行**（见"七、杀软说明"）。

## 五、本次改动（代码层面）

| 文件 | 内容 |
|---|---|
| `cmd/desktop/main.go` | 三模式启动（未配置→设置页；local→内嵌后端；remote→反向代理到远程服务器）；Windows 数据目录 `%AppData%\ZhiShu Lite\`；菜单新增"服务器设置…" |
| `cmd/desktop/prefs.go` | 新增 mode/server_url 持久化与地址规范化 |
| `cmd/desktop/setup.go` | 新增首次启动/切换模式的设置页（本地 HTML，无外部依赖） |
| `cmd/desktop/desktop_inject.go` | 客户端注入脚本：外链转系统浏览器、拦截 blob 下载走原生"另存为"、保存完成提示"打开文件/打开所在文件夹" |
| `cmd/desktop/app.go` | 新增绑定：TestServerURL / ApplyDesktopSetup / GetDesktopSetupState / SaveBlobFile / OpenFilePath / RevealInFolder；restartApp |
| `scripts/package-windows.sh` | 一键打包（构建 exe + 组装绿色版 + zip） |
| `scripts/windows/emutls_stub.c` | DuckDB 预编译库在 MinGW 工具链下的 emutls 符号补桩（链接期注入） |
| `internal/container/container.go` | `ZHISHU_DISABLE_DUCKDB=1` 时跳过 DuckDB 初始化（Windows Lite 下预编译库与 MinGW 运行时 ABI 不兼容，禁用后智能体数据分析工具优雅降级） |
| `internal/agent/tools/data_analysis.go` | DuckDB 不可用时工具返回友好提示而非崩溃 |

服务器端代码**零改动**，现有部署原样使用。

## 六、已验证项（2026-10-12 实测）

- [x] 首次启动显示设置页；"连接服务器"模式卡片、地址输入、**测试连接**（Go 侧探测 /health）通过
- [x] 保存配置后自动重启，进入远程模式加载服务器前端（登录页/主界面/数据全部来自服务器）
- [x] 注册两个账号（alice/bob）并各自登录：**账号隔离验证通过**（各自仅见自己的知识库）
- [x] 服务端 API 上传文档成功（上传组件为 WebView2 标准 Chromium 行为，窗口内拖拽/点选与浏览器一致）
- [x] Windows Defender 对发行 exe 自定义扫描：**无检出**（机器上另有历史威胁记录，均为此前其他软件，与本产品无关）
- [x] 打包脚本产出绿色版目录与 zip
- [x] **登录页"记住密码"**（网页端与 exe 端同一套前端，同时生效）：勾选后登录成功即保存凭据（localStorage，base64 轻度混淆），会话失效/退出后回到登录页自动回填邮箱密码并保持勾选，直接点"登录"即可进入；取消勾选再登录一次即清除保存
- [x] 实测记住密码全链路（exe 与浏览器）：勾选登录 → 凭据落盘 → 回登录页自动回填 → 一键登录进入
- [x] 已禁用客户端对腾讯上游 GitHub 的自动更新检查（避免被提示"升级"覆盖为原版 WeKnora；菜单里的"检查更新"会提示联系管理员）

## 七、杀软 / 签名说明（重要）

- 已采用的标准措施：不打壳/不压缩、嵌入正规版本信息与图标（`build/windows/info.json` + `icon.ico`）、标准 PE 结构。
- **"保证任何杀软永不报毒"没有 100% 手段**。唯一可靠方案是购买代码签名证书（国内 OV 版约数百至两千元/年；EV 版可直接消除 SmartScreen 警告）并对 exe 与发行 zip 签名。建议首批试用反馈无问题后采购。
- 未签名版本的实际表现：SmartScreen 首次运行提示"更多信息→仍要运行"（一次性）；国内主流杀软对 Wails/Go 正常构建一般不报，个别误报可向厂商提交白名单申请。

## 八、Windows 网关架构（v0.7.2 重要变更）

早期版本把界面通过 WebView2 虚拟域名（wails.localhost）转发加载。实测发现
WebView2 的虚拟协议通道会把流式响应**整体缓冲**后再交给页面，导致 exe 端
聊天没有流式输出、也没有检索/思考过程（网页端正常）。

现已重构：Windows 版 exe 内置一个只监听 127.0.0.1 的**本地网关**，webview
改为从网关的真实 HTTP 地址加载界面（网关再把请求转发给服务器端）。由此：

- 聊天恢复与网页端一致的**流式输出和检索/思考过程**（已用自动化测试验证
  SSE 逐块到达，见 `cmd/desktop/gateway_test.go`）；
- 下载、Cookie、会话行为与网页端完全一致；
- 桌面增强（另存为、打开文件/所在文件夹、外链转系统浏览器）改经网关的
  `/__desktop__/*` 端点实现，不再依赖 Wails 页面绑定；
- 角色提示词/模型接入不受影响：它们由服务器端管道统一注入，客户端只是
  载体；exe 与网页端连的是同一个服务端，回答内容应当一致。若模型自报
  身份，请确认对话使用的是配置了角色提示词的智能体/知识库会话（同一会话
  在网页端的表现即为基准）。

## 十、已知限制与说明

1. **DuckDB（智能体数据分析工具）在 Windows Lite 形态被禁用**（`ZHISHU_DISABLE_DUCKDB=1`）：Windows 版预编译 DuckDB 静态库与 MinGW 运行时存在 ABI 不兼容（链接需 emutls 桩、运行期打开即崩溃）。已做优雅降级：智能体的数据分析工具返回"不可用"提示，其余功能不受影响。**连接服务器模式下无影响**（该能力在服务器上运行）。后续如需启用，可评估 TDM-GCC 工具链或自行编译 DuckDB。
2. 下载增强按 <a download> 的 blob 下载路径拦截（覆盖知识库文档下载、FAQ 导出、记忆导出、图表导出等主要场景，单文件 ≤ 48MB 走原生另存为；超大文件自动回退到 WebView2 默认下载到"下载"文件夹，同样可用）。
3. 拖拽上传、粘贴上传依赖 WebView2（Chromium 内核）标准行为，与浏览器一致。
4. **客户端自动更新已禁用**：本分支基于腾讯 WeKnora 二开，上游自动更新会用原版覆盖定制功能。已将启动自检与菜单"检查更新"改为提示联系管理员；待本分支建立自己的发布渠道后可恢复。
5. exe 体积约 254MB（内嵌了完整 Go 后端与 DuckDB 等依赖，本机模式必需；远程模式下这些代码不运行）。后续可做"纯客户端精简版"（不含内嵌后端与 DuckDB，预计可减半以上），需要时提出即可。

## 十一、构建指南（重新出包）

```bash
# 依赖：Go 1.26.x（建议 1.26.9，wails CLI v2.12 与其 x/tools 兼容）、
#       MSYS2 UCRT64 gcc、wails CLI v2.12.0、Node 18+（仅本机模式需要重新构建前端时）
export PATH="<go126>/bin:<msys2>/ucrt64/bin:<wails>/bin:$PATH"
export GOPROXY=https://goproxy.cn,direct CGO_ENABLED=1 CC=gcc
export CGO_CFLAGS="-I<sqlite-include目录> -Wno-deprecated-declarations"

./scripts/package-windows.sh                 # 完整构建（含前端）
SKIP_FRONTEND=1 ./scripts/package-windows.sh # 复用 frontend/dist 现有产物
```

sqlite-include 目录准备（sqlite-vec 的 CGO 需要 sqlite3.h）：

```bash
mkdir -p /c/Users/intel/build-tools/sqlite-include
MATTN=$(go env GOMODCACHE)/github.com/mattn/go-sqlite3@v1.14.24
cp "$MATTN/sqlite3-binding.h" /c/Users/intel/build-tools/sqlite-include/sqlite3.h
cp "$MATTN/sqlite3ext.h"      /c/Users/intel/build-tools/sqlite-include/
```

## 十二、常见问题

- **连不上服务器**：检查服务器进程存活、ping 通、端口放行；在客户端"服务器设置…"里重新测试。
- **切换服务器/模式**：菜单栏 ZhiShu Lite → 服务器设置… → 重新选择并保存（应用会重启）。
- **本机模式数据在哪**：`%AppData%\ZhiShu Lite\data\`（SQLite 库 + 上传文件）。
- **重置客户端配置**：删除 `%AppData%\ZhiShu Lite\desktop-prefs.json` 后重启（回到首次设置页）。
