# AGENT.md — AI Agent 工作规范

> 本文件是给所有在本仓库工作的 AI Agent 及人类协作者的**强制规范**。
> 自 v1.1 起生效。不满足本规范的修改视为未完成，不得交付。

## 背景

- 本仓库是 WeKnora（维娜拉）项目的本地改造副本，计划进行系统化改名（WeKnora → ZhiSu / 智枢）与二次开发。
- `v1.0` 标签 = 原始项目基线（commit `b007bfa`），是所有还原与对比的参考点。

---

## 规则一：任何修改必须通过验收测试后才算完成

1. **禁止"只改不验"。** 改完代码只完成了一半；**验收测试通过 + 提交入库**之后，此次修改才算完整，才允许向用户宣告"完成/可交付"。
2. **验收分级标准**（按改动类型执行对应项，全部通过才算验收通过）：

| 改动类型 | 必做验收 |
|---|---|
| Go 后端代码 | `go build ./...` 编译通过；`go test <相关包> -count=1` 通过；改动面大时加 `go vet ./...` |
| 前端代码 | `npm run type-check` 或 `npm run build` 通过；UI 改动需构建后实际打开页面验证 |
| 配置 / 环境变量 | 配置语法校验（yaml/json/env 可解析）；确认所有读取该配置的代码点已同步；服务能正常启动 |
| docker / compose | `docker compose config` 校验通过；改动后能正常 `up` 起核心服务 |
| 文档 / Markdown | 与代码现状核对一致（不虚构）；链接有效 |
| 脚本 | 至少以 `--dry-run` 或只读方式跑一遍关键路径 |

3. **验证失败的处理**：修复后重新验收；无法立即修复的，必须回滚本次改动（`git checkout -- <file>` 或 `git reset --hard <tag>`），不得把半成品留在工作区。
4. **验收结果必须写进提交信息**（见下方模板），让每次提交自带"已验证"的证据。

---

## 规则二：每次成功修改必须提交一次 git commit（带版本号 + 修改大纲）

1. **一次成功修改 = 一次提交。** 验收通过后立即提交，不得攒批、不得遗漏。
2. **版本号规则**：格式 `vX.Y`，在上一版本基础上 **Y 递增 1**（v1.1 → v1.2 → …）；大范围重构或破坏性变更时 X 递增 1 且 Y 归零（v1.9 → v2.0）。新版本号 = `git tag` 列表中最新号递增。
3. **提交信息模板**（严格遵守）：

   ```
   vX.Y: <一句话概括本次修改>

   修改大纲：
   - <改动点 1>
   - <改动点 2>

   验收测试：
   - <执行的验证命令 / 步骤>
   - 结果：通过

   影响范围：<涉及的模块 / 目录>
   ```

4. **提交同时打同名附注标签**：`git tag -a vX.Y -m "vX.Y <一句话概括>"`。
5. **提交前自查**：`git status` 确认改动范围与预期一致；`git diff` 逐处审阅；确认没有把 `.env`、`node_modules/`、`frontend/dist/`、日志、临时文件带进暂存区。
6. **红线**：禁止 `git push --force` 或改写历史；禁止跳过验收直接提交；禁止只写 "update" / "fix" 这类无信息量的提交说明。

---

## 标准工作流程（每次修改按此执行）

```
1. git status —— 确认工作区干净（有未提交残留先处理）
2. 实施修改（小步快跑，单次修改聚焦一个主题）
3. 按"验收分级标准"执行测试 → 全部通过
   （失败 → 修复重验；或回滚后回到第 2 步）
4. 确定新版本号（取 git tag 最新号递增）
5. git add <明确文件> → git commit（按模板）→ git tag -a vX.Y
6. 向用户汇报：版本号、修改大纲、验收方式与结果
```

---

## 还原与对比

```powershell
git diff vX.Y               # 对比任意版本间的全部差异
git checkout v1.0 -- <路径>  # 把单个文件还原到基线
git reset --hard v1.0       # 整体还原到基线（丢弃其后全部改动，慎用）
git tag                     # 查看全部版本号
```

---

## 本机环境注意事项（Windows + PowerShell）

1. **中文经 pwsh 内联命令传递会被 GBK 转码损坏**：提交信息或脚本内容含中文时，一律先写成 UTF-8 文件再 `git commit -F <file>`，不要在命令行直接内联长中文。
2. **PowerShell 会把弯引号 “ ”（U+201C/201D）当作字符串定界符**：.ps1 脚本中的中文文本一律用单引号字符串，并避免文本中出现弯引号。
3. **含中文的 .ps1 必须带 UTF-8 BOM**，否则被按 GBK 解析、直接报语法错误。
4. 本机已装 Microsoft Office，需要生成 Word 文档时可用 COM 自动化（`New-Object -ComObject Word.Application`）。

---

## 版本记录

| 版本 | 日期 | 内容 |
|---|---|---|
| v1.0 | 2026-09-13 | 原始 WeKnora-main 基线（b007bfa） |
| v1.1 | 2026-09-13 | 新建本规范文件 AGENT.md（验收测试 + 版本化提交两条强制规则） |
| v1.2 | 2026-09-14 | 第一层改名：显示层（前端界面/文档/小程序/文档站，WeKnora→ZhiSu） |
| v1.3 | 2026-09-14 | 第二层改名：构建与部署标识（镜像 zhisu/*、容器与网络 ZhiSu-*、二进制 ZhiSu、环境变量 WEKNORA_→ZHISU_、Helm chart、systemd 单元、Go 显示串解锁、系统提示词身份）；验收：容器化 go build/vet/test 全绿 + docker compose config + 前端构建 |
| v1.4 | 2026-09-14 | 第三层 A 档：运行时数据标识（DB_NAME=ZhiSu、向量集合 zhisu_embeddings、OpenSearch 索引前缀 zhisu、对象存储前缀 zhisu/、lite 数据库 zhisu.db、前端 localStorage 键 ZhiSu_/zhisu_、JWT aud=zhisu、compose 项目名钉为 zhisu(-dev) 并修正 Makefile 卷目标）；旧数据兼容性放弃（清空卷重新初始化）；验收：容器化 go build/vet/test 全绿（发现并修复 aud 签发/校验两端不同步）+ 前端 test 428 通过/build 通过 + compose config；第四层（module path、X-WeKnora-Signature、weknoracloud、widget SDK、CLI 生态）按决议永久保留 |
| v1.5 | 2026-09-14 | 拼写纠正：zhisu→zhishu（智枢拼音勘误，此前的 Su 为笔误）。三形态全局替换 ZhiSu→ZhiShu(显示名)/ZHISU_→ZHISHU_(环境变量)/zhisu→zhishu(标识符)，346 文件约 2400 处 + 3 个 systemd 服务文件改名 + 用户 .env 同步；顺带修复 v1.3 遗留的 ZhiSuCloud 注释漂移（还原为 WeKnoraCloud，与冻结的 provider id 一致）并将 ELASTICSEARCH_INDEX/OPENSEARCH_INDEX 示例值规范为小写（ES/OpenSearch 索引名必须小写）；新栈尚未启动故零数据迁移，仅重建镜像 zhishu/app:latest；git 历史与 v1.0–v1.4 标签保持原样 |
| v1.6 | 2026-09-14 | 部署修复：compose build.args 补声明 GOPROXY_ARG（默认 https://goproxy.cn,direct）与 docreader APT_MIRROR 默认值（http://mirrors.aliyun.com，注释说明 sed 连 scheme 替换必须带前缀）。根因：compose 不转发未声明的 build args，导致 compose 构建时 go 直连 proxy.golang.org 被墙必挂在 migrate 步（上游原版缺口）；此前直传 --build-arg 的 docker build 不受影响。验收：docker compose config ×2 通过 + docker compose build 完整实测通过（app 走 goproxy.cn 通过 migrate 步、docreader 从源码全链路重建成功）。插曲：验收期间 E 盘写满致引擎崩溃，重建 docker-desktop 引导 distro（wsl --unregister docker-desktop，数据盘 docker_data.vhdx 完好）+ 清理构建缓存 19GB/旧卷 9.7GB/未用镜像 19GB 后恢复，zhishu 全栈容器由 restart 策略自动拉起 |
| v1.7 | 2026-09-14 | 品牌资源统一（图片型残留）：frontend/src/assets/img/weknora.png→zhishu.png（400×126 透明底、原色系 #081828 的 ZhiShu 文字标，GDI+ Segoe UI Bold 生成），Login.vue/menu.vue 引用同步（登录注册页左上角+侧边栏两处生效）；favicon.ico 重做为藏青圆角方+白色 Z；本地启动脚本改名 start_weknora.bat→start_zhishu.bat、stop_weknora.bat→stop_zhishu.bat（echo 文案同步 ZhiShu；两文件实为此前已入库文件，本次一并入库）。验收：前端 428 测试+vite 构建通过、zhishu/ui 镜像重建并重启、/ 与 /favicon.ico 均 HTTP 200 |
| v1.8 | 2026-09-14 | logo 文艺化重设计（v1.7 迭代）：应用户反馈粗黑体单调，zhishu.png 由 Segoe UI Bold 单色字标重绘为 Georgia Italic "ZhiShu"（墨蓝 #14355E）+ 楷体「· 智 枢 ·」（哑金 #B08A54）双行衬线组合，400×126 透明底；三候选方案（楷体印章/衬线双色/青瓷渐变）由用户选定方案二。验收：vite 构建通过、zhishu/ui 镜像重建重启、/ HTTP 200 |
| v1.9 | 2026-09-14 | 登录页 logo 显示尺寸对齐标语字号：Login.vue .logo-image 四断点同比例放大（120→144/100→120/80→96/70→84px），桌面端主字渲染 62×(144/400)≈22.3px 与 .showcase-subtitle 22px 对齐。验收：vite 构建通过、zhishu/ui 重建重启、/ HTTP 200 |
| v2.0 | 2026-09-14 | 登录页 logo 再放大一号（v1.9 迭代）：.logo-image 四断点 144→176/120→146/96→116/84→102px，桌面端主字渲染约 27px。验收：vite 构建通过、zhishu/ui 重建重启、/ HTTP 200 |
| v2.1 | 2026-09-14 | 登录页轮换插画文案中文化：screenshot-1~4.svg 共 56 处英文文本替换为对应中文（字号/颜色/坐标零改动），术语与 i18n 官方译法对齐；顺带清除插画内 WeKnora 品牌残留（→ZhiShu · 知识检索）；格式专名 PDF/Word/Excel/Markdown 保留。验收：锚点全命中 + XML 解析通过 + vite 构建 + zhishu/ui 重建重启 + / HTTP 200 |
| v2.2 | 2026-09-14 | 系统对话人设名改用中文「智枢」：system_prompt.yaml 6 处人设句改为 You are 智枢 (ZhiShu),（volume 挂载启动加载不落 DB，重启 app 即生效，模型中文对话自称智枢）+ zh-CN.ts 对话开场 title 改「Hi，我是智枢…」；界面产品指称与其他语言包保持 ZhiShu。验收：前端 428 测试+vite 构建、zhishu/ui 重建、app+frontend 重启全栈 healthy、/ HTTP 200 |
| v2.3 | 2026-09-15 | 设置弹窗隐藏 Claw Skill 与 WeKnora Cloud 两个功能入口（仅前端隐藏，后端/路由/数据不动）：Settings.vue 导航移除 weknoracloud 固定项与 integration-claw 页签并新增 HIDDEN_SETTING_SECTIONS 守卫（canSeeSection 对隐藏节返回 false，深链接 ?section=… 落 role-denied 兜底页，navItems 回退 general）；integrations.ts 从 INTEGRATION_TABS/INTEGRATION_PREVIEW_ITEMS 移除 claw（侧边栏悬停预览同隐、旧书签 tab=claw 回退 integration-im）；ModelEditorDialog 提供商下拉过滤 weknoracloud；解析引擎列表过滤 weknoracloud。验收：前端 428 测试（含 settingsRoute claw 回退断言更新）、vite 构建、zhishu/ui 重建重启、/ HTTP 200、Settings chunk 导航残留 0 处 |
| v2.4 | 2026-09-17 | 品牌定名「智枢文渊」（响应"不要英文、直接中文名"）：zhishu.png 重绘为纯中文字标「智枢文渊」（思源宋体 Medium，墨蓝 #14355E，1600×504 透明底，GDI+ 生成，中文串经 UTF-8 文本文件进脚本规避 GBK 损坏）+ 哑金 #B08A54 白字「渊」印章，替换原 Georgia Italic "ZhiShu"+楷体「·智枢·」双语组合；Login.vue 登录页 logo 下新增口号行「问渠智枢，知识活水」（淡金 #F5EDD8、字距 0.32em、146/116px 断点缩字号、≤480px 隐藏），alt 改「智枢文渊」；index.html 标签页标题去尾部「 ZhiShu」、meta description 去「（ZhiShu）」；顺带入库 zh-CN.ts 此前未提交残留（「首次使用 ZhiShu？」→「首次使用智枢？」）。验收：字形探测（「渊」轮廓 318 点，非缺字）+ 像素 QA（墨蓝 9.8 万/印金 7.6 万/白字 2.1 万 px、垂直居中偏差 0、四边留白对称）；npm run build 通过、npm test 428 全过；ZhiShu-frontend 绑定挂载 dist 直接生效，/ 200 且标题解码为「智枢数据库知识管理系统」、新 logo 资源 HTTP 200 字节数一致 |
| v2.5 | 2026-09-17 | logo 去重「渊」：主字标去掉未被印章框住的 plain「渊」（即「智枢文渊」的第 4 字），主字标改 3 字「智枢文」，由金印「渊」补全名义，合读仍为「智枢文渊」；印章由 316 放大至 360（升格为名字组成部分，与字高 347 匹配，「渊」字 em 0.62×印边），画布仍 1600×504 居中。验收：像素 QA（墨蓝 10.6 万/印金 10.0 万/白字 2.8 万 px、垂直居中偏差 0、留白对称）、npm run build 通过、npm test 428 全过、/ 200、新 logo 资源 HTTP 200 字节数一致 |
| v2.6 | 2026-09-17 | 登录页移除口号行「问渠智枢，知识活水」（用户反馈页面上仍在：根因为删除只落源码，而前端容器绑定挂载的 dist 未重建）：Login.vue 模板删除 logo 下方 .logo-slogan div，样式同步清理主规则与 1024/768/480px 三处响应式覆盖；构建期间定位并绕过本机环境确定性故障——C 盘用户 %TEMP% 上 esbuild 临时文件删除必报 Access is denied（沙箱与否均复现、磁盘空间充足、无遗留进程，判定为杀软对该 1.3MB CJS 产物临时文件的实时扫描锁），将 TMP/TEMP 重定向至 E:\…\.build-tmp 后构建成功。验收：grep frontend/src 与 dist 口号/logo-slogan 均为 0；npm run build 通过（vite 7.3.6，6399 模块）；docker compose up -d 全栈 healthy；curl 127.0.0.1/ 200 且线上 assets/Login-CknghQi6.js 内容核对不含口号；浏览器刷新即可见效果 |
| v2.7 | 2026-09-17 | README 四语言（EN/CN/JA/KO）按当前系统现状修订：logo 引用换为智枢文渊中文字标（新增 docs/images/zhishu-logo.png，旧 945×650 logo.png 不再被引用）；产品名定名「智枢文渊（ZhiShu）」并标注基于 Tencent WeKnora（MIT）二次开发；新增「本仓库与上游 WeKnora 的差异」节（品牌、对话人设「智枢」、设置页隐藏 Claw Skill 与 WeKnora Cloud 入口、部署标识 zhishu/* 与 ZhiShu-* 与 ZHISHU_ 与 DB ZhiShu、Windows 一键脚本）；最新更新节加本仓库改造版本记录指引（指向 AGENT.md）；模型管理行标注 WeKnora Cloud 入口默认隐藏；快速开始补 Windows 脚本与默认端口说明（FRONTEND_PORT=80 / APP_PORT=8080）；服务地址表补端口变量列；移除上游渠道徽章（trendshift/官网/微信对话开放平台）与「使用微信对话开放平台」章节。验收：四文件 grep 上游徽章域名与旧 logo 引用均为 0；引用的本地文档路径全部 Test-Path 通过；与 docker-compose.yml/.env.example/VERSION 现状核对一致 |
| v2.8 | 2026-10-08 | 设置中心「发布集成」整组下线（用户需求：现阶段 IM 集成/网页嵌入/Chrome 插件用不上，所有账号类型不可见且不可编辑）：Settings.vue HIDDEN_SETTING_SECTIONS 新增 integration-im/embed/api/chrome 四节（隐藏判断在 canSeeSection 首位、先于角色门槛与 canAccessAllTenants bypass，全部账号生效；四项过滤后 navGroups 空组过滤使「发布集成」分组整体不渲染；深链 ?section=integration-* 与 ChromeExtensionLanding/ClawSkillLanding 的 openSettings('integration-api') 均落 role-denied 兜底页）。影响控制：后端路由、Agent 编辑器内渠道面板（AgentEmbedChannelPanel/IMChannelPanel）、已发布 Embed Widget 与 IM 渠道运行链路、既有 API Key 凭据全部不动——仅收起 UI 管理入口（含 integration-api 的空间 API Key 管理，恢复删 4 行即可）。验收：npm test 通过、npm run type-check 通过、npm run build 通过（TMP/TEMP 重定向 .build-tmp 规避 esbuild 临时文件锁，沿用 v2.6 方案）、ZhiShu-frontend 绑定挂载 dist 直接生效、/ HTTP 200 |
| v2.9 | 2026-10-09 | 后端守卫收口（权限收口方案阶段 1，用户需求：模型/数据与扩展/检索配置写权限收归系统管理员统一管理）：routes_infra.go 八组平台配置写路由 g.Admin()/AdminOrSystemAdmin→g.SystemAdmin()（models 增删改/凭据/debug、ollama check+download、sandbox-configs 写+技能上传改删、mcp-services 写/test/凭据/审批策略、web-search-providers 写/test/凭据、vector-stores 写/test、storage-backends 写/test/default）；routes_auth_tenant.go parser-engines/check+docreader/reconnect+storage-engine-check+sandbox-check→SystemAdmin、PUT /tenants/kv/:key 路由守卫→AdminOrSystemAdmin；handler/tenant.go UpdateTenantKV 平台键（retrieval-config/parser-engine-config/storage-engine-config/web-search-config）key 级判定 SystemAdmin-only，chat-history/memory 维持空间 Admin；extract/* 图谱抽取测试与全部读路由维持不变；API Key 策略零改动。新增 middleware/rbac_platform_config_test.go（空间 Owner/Admin/Viewer 全 403、系统管理员放行矩阵）+ handler/tenant_platform_kv_test.go（平台键 4×空间 Admin 全 403、系统管理员双门通过、chat-history-config 维持 Admin）；TestPutTenantParserConfigAdminPreservesRedactedSecrets 语义更新为 SystemAdmin 上下文（脱敏断言不变）。验收：容器化 golang:1.26 go build ./internal/... ./cmd/server → BUILD_OK（本机 Windows 缺 CGO/sqlite3.h/duckdb 构建约束，沿用 v1.3 容器化验收惯例）；go vet 三包通过；go test middleware/handler/router 三包全绿；docker compose build app + up -d 全栈 healthy；/system/parser-engines 与 /models 路由存活冒烟 401 |
| v2.10 | 2026-10-09 | 前端只读锁定（权限收口方案阶段 2，与 v2.9 后端守卫对齐；可见性阶梯与 KB 级选用能力不变）：settingsAccess.ts 新增 PLATFORM_MANAGED_SECTIONS 八分区语义标注（明确不复用 SYSTEM_ADMIN_SETTINGS_SECTIONS——其挂着关闭跳转专属行为）；九组件写控件锁定 canManagePlatform=isSystemAdmin：ModelSettings（debug 入口/添加卡片/canEditModel/canDeleteModel/editModel/getModelOptions 六处）、OllamaSettings（重测隐藏+下载禁用）、VectorStoreSettings（菜单/添加/点击）、StorageBackendSettings（添加/canEdit/canDelete/canSetDefault）、ParserEngineSettings（抽屉 footer/onSave/onCheck）、SandboxSettings（添加卡片/卡片菜单/工作区脚本开关；技能清单浏览保留）、WebSearchSettings（添加/点击/菜单）、McpSettings（添加/点击/双菜单含 builtin）、RetrievalSettings（canEdit→isSystemAdmin，⌘K 面板）；i18n 四语言新增 platformManagedReadOnly 只读提示键——插入用 Node 脚本处理 UTF-8 字节（PowerShell Get-Content/Set-Content 管道曾致 GBK→UTF-8 双重编码损坏四语言包，git checkout 还原后重做）。KBModelConfig/KBVectorStore/KBStorage/ModelSelector 选用层与对话页/数据源/提示词模板/消息管理/长期记忆零改动。验收：npm run check-i18n 11 用例全过、npm test 428 全过、type-check 通过、build 通过（42s）、线上 tdesign-icon-offline chunk 含 platformManagedReadOnly×4（HTTP 200）确认绑定挂载生效 |
| v2.12 | 2026-10-10 | 平台统一配置八分区对非系统管理员整体隐藏（响应"只能看不能改很难受"反馈，产品语言统一「看得见 = 管得了」；后端零改动，读接口维持开放）：Settings.vue canSeeSection 对 PLATFORM_MANAGED_SECTIONS 八分区改判 authStore.isSystemAdmin（先于角色/超管 bypass；非系统管理员的「模型」「数据与扩展」两组随空组过滤整体消失，深链落 role-denied）；GlobalCommandPalette.vue ⌘K 检索设置两触发入口（齿轮图标+空态按钮）v-if=isSystemAdmin；UserMenu.vue「模型管理」快捷入口 canManageModels → 仅系统管理员（members 快捷入口不变）。九组件 v2.10 内部写控件锁保留作纵深防御；platformManagedReadOnly 四语言键保留（暂无渲染面）。KB 级选用/对话/Agent/数据源/消息管理/长期记忆零改动。验收：npm test 428 全过、type-check 通过、build 通过（1m23s）、线上 Settings-DljjBAbl.js（589402 字节与本地一致）含收口后八分区集合，绑定挂载即时生效 |
| v2.13 | 2026-10-10 | 智能体管理权收归空间 Admin+（业务资产治理：普通成员只读+使用，对齐知识库管理模型）：routes_agent.go agentsWrite 组四条写路由收紧——POST /agents 创建 g.Contributor()→g.Admin()、POST /:id/copy g.Contributor()→g.Admin()、PUT/DELETE /:id g.OwnedAgentOrAdmin()→g.Admin()（creator 分支不再覆盖写路径，共建智能体普通成员不可再改）；读路由维持 Viewer+，对话中选用/提及/共享智能体使用不受影响。新增 middleware/rbac_agent_guard_test.go（viewer/contributor 403、admin/owner 放行矩阵）。验收：容器化 golang:1.26 build ./internal/... ./cmd/server → BUILD_OK、vet 两包通过、middleware/router 测试全绿 |
| v2.14 | 2026-10-10 | 前端使用面收口（与 v2.13 后端对齐）：AgentList.vue 创建按钮（头部/两处空态）hasRole('contributor')→canManageAnyAgent(=hasRole('admin'))、canManageAgent 去掉 creator 分支、卡片/列表双菜单 v-if 简化、默认 scope 与空态判定同步（无管理权者默认 all 视图）；Input-field.vue 模型选择器按身份收口——canSelectModel=isSystemAdmin，模型显示/下拉/「+添加模型」仅系统管理员渲染，两处 send-msg emit 非 SystemAdmin 传空 model_id（后端 resolveChatModelID 默认链兜底：请求缺省→Agent 配置→KB 摘要模型→首个可用对话模型，本地单一模型部署即命中管理员配置）。验收：npm test 428 全过、type-check 通过（TS2448 use-before-declaration 已修正：canManageAnyAgent 定义上移）、build 通过（1m28s）、线上 AgentList chunk 含压缩后守卫（oe=h(()=>Z.hasRole("admin"))）绑定挂载生效 |
| v2.15 | 2026-10-10 | 多端会话管理后端（用户需求：同账号多设备登录的安全收敛，方案 B=多端并存+会话面板，明确不做互踢/不做按设备隔离历史）：迁移 000088 双写（versioned+sqlite 000013）auth_tokens 补 session_id/device_label/device_ip 三列+索引；AuthToken+3 字段；LoginSessionMeta context 载体（WithLoginSessionMeta/LoginSessionMetaFromContext）；generateTokensForTenant 五条签发路径贯通（SessionID 新登录新生成、RefreshToken 轮换继承原会话+设备信息、JWT claim 带 session_id）；仓储扩展 RevokeBySessionID（user_id 双重限定）/GetLatestSessionToken；service ListActiveSessions（按 session_id 聚合、撤销/过期排除、存量空 id 归「早期会话」桶、Current 高亮）+RevokeSession（自踢保护）+ParseSessionIDFromToken；handler 三 API（GET /auth/sessions、GET /auth/sessions/current、DELETE /auth/sessions/:session_id，g.Viewer() 本人会话；API Key default-deny）+loginSessionMetaFromRequest（UA 解析五浏览器四系统）+五注入点（Login/OIDC/invite/AutoSetup/SwitchTenant）；改密码联动核实：上游已有 RevokeTokensByUserID 全端失效。新增 service/user_sessions_test.go 4 例（聚合/早期桶/自踢保护/继承合约）；既有 stub 补新接口方法。验收：容器化 build OK、vet 全 internal 通过、database（sqlite version 13）/middleware/router/handler 测试 ok、service 4 例新单测 PASS；service 1 例（MCP）+service/file 3 例（OSS）经 git stash 鉴别为既有失败；docker compose build+up 全栈 healthy、实库确认 3 列+索引、三路由 401 存活冒烟 |
| v2.16 | 2026-10-10 | 多端会话管理面板（方案 B 阶段 2）：新增 api/auth/sessions.ts（list/current/revoke）+ views/settings/SessionSettings.vue（会话列表：本机高亮"当前设备"标签、设备描述/IP/登录/活跃/有效期、逐条踢下线（当前会话行不提供按钮+后端自踢保护双保险）、确认弹窗、被踢设备走现有 401→跳登录链路）；Settings.vue 账户组注册 sessions 分区（navItems/navGroups/content 三处）；settingsAccess.ts sessions:'viewer'（个人资产全员可见自己的）；i18n 四语言新增 sessionSettings.* 块——并修正 v2.10 遗留错位：platformManagedReadOnly 当时被插到 settings 顶层级之外（四语言一致故审计未报警），本次将 sessionSettings 正确放回 settings 内部并保持四语言键位语义一致。验收：check-i18n 11 全过、npm test 428 全过、type-check 通过、build 通过（39s，首次失败为 v2.6 已知 C 盘 esbuild 临时文件锁重试通过）、线上 Settings/tdesign-icon-offline 双 chunk MD5 与本地一致（FFA9996D…） |
| v2.17 | 2026-10-10 | 多端会话管理收尾：双账号终验指引（部署内天然双账号：857900666@qq.com 系统管理员+Owner、2801618154@qq.com 空间 admin——各登录两台设备验证面板互不可见对方会话、踢除闭环）；文档定稿（PRD §5.1 新增登录设备管理小节+修订 v1.4、架构书 §8.8 多端会话条目、本方案文档标记完成、AGENT.md 三行记录）。验收：文档与代码现状核对一致、git tag v2.15/v2.16/v2.17 三阶段留档 |
| v2.19 | 2026-10-10 | 账号管理后端（账号管理实施方案阶段 1，用户需求：系统管理员平台级账号治理——增查改禁、跨空间角色、多端会话查看与强制下线；决策：入口进设置中心系统管理组、权限编辑=管理员开关+跨空间角色、先禁用后删除、零迁移）：仓储 ListUsersPaged（搜索/三态过滤/分页计数，LOWER() LIKE 双库安全）；新窄接口 SystemUserAdminService 九方法挂同一 userService（dig 形状不变，SystemHandler 构造时接口断言注入，胖 UserService 接口与其大量测试 stub 零改动）；handler system_user_admin.go 九端点挂 /system/admin 组（组级 SystemAdmin 守卫继承，API Key default-deny）：列表/详情(跨空间成员关系+空间名)/改资料(唯一性校验)/禁用启用(is_active+RevokeTokensByUserID=下一请求 401，登录路径已有 IsActive 检查)/强制下线/会话查看/踢单会话/跨空间角色调整/移出空间（后两者转发 TenantMemberService——核实其服务层无调用者权限检查、ErrLastOwner 保护与 rbac.member_* 审计自带，管理员通道零改动直接继承）；自我保护 target==caller 一律 ErrAdminSelfAction→400；审计新增 system.user_updated/user_status_changed/user_force_logout/session_revoked/membership_role_changed/membership_removed 六动作（emitAdminAudit 平台侧，成员操作补 system-scope 行溯源）；路由树冒烟测试锁定 POST /users/{create,reset-password} 与 /users/:user_id/force-logout 同方法树静态/参数段共存（gin v1.12）。新增服务层 14 例+handler 14 例+路由树 1 例；既有 stub 补 ListUsersPaged；顺带修复 v2.15 遗留：contextCloneAcrossDetach 补 LoginSessionMetaContextKey 条目（types 包 TestEveryContextKey 恢复绿）。验收：容器化 weknora-buildenv:g1.26（对齐 Dockerfile.app 构建依赖，本机 Windows 缺 CGO/gcc）build ./internal/... ./cmd/server BUILD_OK、vet 全 internal 通过、types/service/handler/router/middleware/repository 测试全绿（仅 2 例既有环境失败：MCP 与 parser-secret 测试的 SSRF 校验把 example.com 解析到 198.18.0.0/15 受限段——本机代理 fake-ip 所致，git stash 干净基线复现鉴定与本次无关）；docker compose build app && up -d healthy；实库 \d users 零 schema 变化；curl 矩阵实测（临时提升 2801618154 测试账号+备份/复原原密码哈希）：未认证 401×3、列表/搜索/详情(空间名 enrichment)/会话列表正向 200、防自操作 400×4、禁用后登录 Account is disabled→恢复→正常、审计表 system.user_status_changed×4 |
| v2.20 | 2026-10-10 | 账号管理面板（阶段 2）：settingsAccess.ts SYSTEM_ADMIN_SETTINGS_SECTIONS 新增 account-management（继承系统管理组关闭跳转/深链 role-denied 兜底，测试锁定五键）；Settings.vue navItems+system_administration 组+content 分支+import 四处注册、handleClose 跳转列表同步；api/system 九函数（adminListUsers/adminGetUserDetail/adminUpdateUser/adminSetUserStatus/adminForceLogout/adminListUserSessions/adminRevokeUserSession/adminUpdateMembershipRole/adminRemoveMembership）+四类型；新增 views/system/AccountManagement.vue（用户表格：搜索/状态过滤/分页/状态标签/系统管理员徽标/操作列禁用-启用 popconfirm/自身行隐藏危险操作；新建账号抽屉复用 createSystemUser 自动生成密码一次性展示）+ views/system/AccountDetailDrawer.vue（SettingDrawer 三区块：基本信息 PATCH 编辑+邮箱变更确认；空间与角色——角色下拉即时应用失败回滚重载、移出空间 danger popconfirm、状态标签；登录会话——设备/IP/登录/活跃+逐条踢除+全部下线；isSelf 隐藏写入口对齐后端 400 守卫引导走个人面板）；i18n 四语言 accountManagement.*（Node 脚本插入沿用 v2.10/v2.16 惯例；formEmailPlaceholder 改「user at example dot com」规避 @ 前缀 linked-format 误判）。验收：check-i18n 11 全过、npm test 428 全过（settingsAccess 断言更新）、type-check 通过、build 通过（43.6s，TMP 重定向惯例）；docker compose build frontend && up -d，线上 Settings-Bsyws3a2.js 含 account-management、locale chunk 字节级确认「账号管理」四语言文本 |
| v2.21 | 2026-10-10 | 账号管理收尾：文档定稿（PRD §5.14 系统管理组四分区→五分区+修订 v1.5、架构书 §8.8 账号管理条目、本方案文档标记完成、AGENT.md 三行记录）；端到端验证已在 v2.19 验收中以临时提升的测试账号完成全矩阵（正向+自我保护+禁用闭环+审计），非系统管理员可见性由 settingsAccess 单测与 role-denied 兜底机制锁定。验收：文档与代码现状核对一致、git tag v2.19/v2.20/v2.21 三阶段留档 |
| v2.22 | 2026-10-10 | 文档同步（用户问询「README 与说明性文件是否要同步」后的盘点与收敛）：同步三处——README.md/README_CN.md「Access Control/权限控制」能力行补平台级账号管理（JA/KO 功能表本无此行，不补）；website-docs/03-features/20-platform-admin.md 平台管理页（能力对比表「管什么」加账号治理、截图说明去分区计数防漂移、控制台分区表四→五并新增账号管理行含接口与自我保护/审计要点）；website-docs/04-api/02-api-system.md 系统管理节新增九条 users 系列端点条目（用途/请求体/响应/curl 格式对齐，标注 API Key default-deny 与 target==caller 400 自我保护）。明确不同步并留档理由：CHANGELOG.md 是上游 0.x 版本线（最新 0.7.2），本地 v2.x 系列按惯例只记 AGENT.md；docs/docs.go 为 swaggo 上游 0.7.2 快照（2026-09-14 生成，v2.15 /auth/sessions 起即未再生成，惯例不重生成——handler godoc 注释已备好未来重生成）；README「差异/最新更新」节自述定位为品牌差异与上游演进、本地功能记录外包给 AGENT.md（56 行明示）。验收：四文件与代码现状核对一致；git tag v2.22 留档 |
| v2.23 | 2026-10-11 | P1-1 修复（测试报告-核心业务.md 缺陷 #1，约束：不改任何正常链路）：knowledge_process.go processChunks 的「KB 开着 vector/keyword 索引但 GetEmbeddingModel 失败（典型 embedding_model_id 为空）」早退分支——原实现只打日志直接 return，文档永久卡死 processing；现复用同函数 chunking 失败路径的同款回写（ParseStatus=failed + ErrorMessage="embedding model unavailable: …" + repo.UpdateKnowledge + failStage(EMBEDDING_PROVIDER_FAIL)，既有错误码零前端改动）。正常链路零触碰：NeedsEmbeddingModel()=false 的 KB（合法关闭向量/关键词索引）仍走跳过分支原样返回；嵌入模型存在时路径一行未动。新增 knowledge_process_p11_test.go 2 例（缺失→failed 显式回写锁定；禁用索引的 KB 不得产生 failed 回写）。E-1（SSRF vs 本机代理 fake-ip）判定为**配置问题不改代码**：ValidateURLForSSRF:1304 白名单先行（IsSSRFWhitelisted 跳过重检查）、ssrf.whitelist 系统设置运行时热生效（applySSRFWhitelist→SetSSRFWhitelistFromRaw）、支持精确域名/*通配/IP/CIDR（198.18.0.0/15 可整段加入），修复路径=设置中心 ssrf.whitelist 加 CIDR 或调整代理 fake-ip，安全权衡留运维决策。验收：容器化 build ./internal/... ./cmd/server BUILD_OK、vet service 包 OK、TestProcessChunks_ 2 例 PASS、service 包全量回归仅剩既有环境失败（MCP SSRF/fake-ip 与 GroupsBySession 时间 flake，与 v2.22 测试报告鉴定一致） |