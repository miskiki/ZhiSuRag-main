# 智枢文渊（ZhiShu）Web 端产品需求文档（PRD）

| 项 | 内容 |
|---|---|
| 产品名称 | 智枢文渊 —— 智枢数据库知识管理系统 |
| 文档版本 | v1.0（正式版） |
| 基线版本 | WeKnora v0.7.2 二次开发（本仓库改造版本线 v1.0–v2.5，见 AGENT.md） |
| 文档范围 | Web 前端（含网站嵌入 Widget）的界面功能与前后端交互 |
| 明确排除 | 无前端入口或已舍弃的后端能力（见 §3 产品边界） |

---

## 1. 产品概述

### 1.1 产品定位

智枢文渊是一款基于大语言模型（LLM）的企业级 RAG 知识库管理系统。用户将文档、网页、FAQ、外部平台数据源汇入知识库，系统完成解析、分块、向量化后，通过 **RAG 快速问答**、**ReAct Agent 智能推理** 与 **Wiki 知识库生成** 三大核心能力，把分散文档沉淀为可查询、可推理、可持续演进的专属知识资产。

本 PRD 只覆盖 **Web 前端展示** 与 **前端—后端交互**：即用户在浏览器（以及嵌入外部网站的 Widget）中看到什么、能做什么、前端与后端之间发生哪些请求。后端侧仅为支撑这些界面而存在的接口才纳入范围。

### 1.2 产品目标

1. 提供开箱即用的知识库管理界面：建库、传文档、看解析进度、管分块与标签。
2. 提供可靠的对话体验：流式回答、引用溯源、Agent 工具调用过程可视化、工具调用人工审批。
3. 提供可控的知识加工链路：分块编辑与版本回滚、Wiki 自动生成与人工修订、数据源自动同步。
4. 提供企业级多空间协作与治理：RBAC 四级角色、共享空间、成员与审计管理。
5. 提供面向外部的发布能力：网站嵌入 Widget、API Key 集成、IM 渠道接入配置。

### 1.3 目标用户

| 用户 | 典型场景 |
|---|---|
| 普通成员（Viewer） | 检索知识、与机器人问答、阅读 Wiki |
| 内容贡献者（Contributor） | 上传/维护文档、编辑分块、维护 FAQ、编辑 Wiki |
| 空间管理员（Admin/Owner） | 配置模型、检索、存储、解析引擎；管理成员与共享；管理 API Key |
| 系统管理员 | 平台级设置、平台 API Key、运行时队列、系统审计、管理员账户 |
| 外部访客 | 通过嵌入 Widget 或 IM 渠道与已发布智能体对话 |

### 1.4 术语表

| 术语 | 说明 |
|---|---|
| 空间（Tenant / Workspace） | 数据隔离的基本单位，拥有独立的成员、知识库、会话与设置 |
| 共享空间（Organization） | 跨空间的协作组织，可共享知识库与智能体 |
| 知识库（Knowledge Base） | 文档/FAQ 的集合，支持文件夹树与标签 |
| 知识（Knowledge） | 知识库内的一条文档/FAQ 记录，经解析生成分块 |
| 分块（Chunk） | 文档解析后的检索单元，可人工编辑并保留版本历史 |
| RAG | 检索增强生成：先检索知识库片段，再由 LLM 组织回答 |
| Agent 模式 | ReAct 多步推理：自主编排知识检索、MCP 工具、网络搜索 |
| Wiki 模式 | Agent 从原始文档自治生成互相链接的 Markdown Wiki 页面与知识图谱 |
| MCP | Model Context Protocol，外部工具服务接入协议 |
| Embed 渠道 | 面向外站的网站嵌入 Widget 通道，凭 Embed Token 鉴权 |
| Lite 模式 | 单用户轻量部署形态（无共享空间、自动建号） |
| 能力探测（Deployment Capabilities） | 后端按部署实际注册的路由返回能力清单，前端据此隐藏无后端支撑的入口 |

---

## 2. 系统总体结构（面向前端视角）

```
浏览器 ─── Web SPA（Vue 3 + TDesign + Pinia，路由 /platform/*）
   │            │
   │            ├─ 主应用：认证 / 知识库 / 对话 / 智能体 / 共享空间 / 设置
   │            └─ Embed SPA（独立入口 embed.html，路由 /embed/:channelId）
   │
   ▼  REST /api/v1/*（JSON，Bearer JWT）+ SSE 流式对话
Go 后端（Gin）
   ├─ docreader（文档解析服务，gRPC）
   ├─ sandbox（Agent 技能沙箱）
   └─ Postgres / Redis / 对象存储 / 向量库 / 搜索服务 / Langfuse 等基础设施
```

### 2.1 前端技术栈

- Vue 3.5 + TypeScript + Vite + Pinia + Vue Router + vue-i18n（zh-CN / en-US / ko-KR / ru-RU）
- TDesign Vue Next 组件库；Markdown 渲染：marked + highlight.js + KaTeX + Mermaid + DOMPurify
- HTTP：axios 统一封装（`utils/request.ts`）；SSE：`@microsoft/fetch-event-source`
- 文件预览：docx-preview、@vue-office/pptx、xlsx（本地打包版）

### 2.2 前后端交互约定

| 约定 | 说明 |
|---|---|
| 基础路径 | `getApiBaseUrl()` 返回应用基路径（默认 `/`；LocalHub 反代部署为 `/app/weknora`），接口路径自带 `/api/v1` 前缀 |
| 认证 | 请求头 `Authorization: Bearer <JWT>`；Embed 渠道使用 `Authorization: Embed <token>` |
| Token 刷新 | 401 时自动用 `zhishu_refresh_token` 调 `/auth/refresh` 重放；公开接口（登录/注册/邀请/OIDC/Embed）401 不触发刷新 |
| 多空间 | 激活空间后所有请求附 `X-Tenant-ID` 头 |
| 语言 | 所有请求附 `Accept-Language` |
| 链路追踪 | 每请求生成 `X-Request-ID`（12 位随机串），对话流式另计 TTFB 埋点 |
| 响应包装 | `{ code, msg, data }` 或 `{ success, data, message }`；错误提取 `error.message` |
| 流式对话 | POST SSE（`Content-Type: text/event-stream`），事件 JSON 含 `response_type`（answer/引用/工具调用/RAG 进度等） |
| 上传 | `multipart/form-data`（`postUpload`，带上传进度回调）；下载为 Blob（`getDown`） |
| 能力探测 | 前端启动时拉取 `GET /api/v1/system/capabilities`，`supported=false` 的功能入口整体隐藏（如 Lite 版无共享空间） |

---

## 3. 产品边界（明确的"不做"清单）

> 本仓库后端保留了上游 WeKnora 的几乎全部能力，但以下内容 **没有 Web 前端调用方、被前端隐藏，或由非 Web 客户端消费**，不属于本 PRD 的功能范围。边界判定方法：逐一核对前端 API 层（frontend/src/api/ 全部 28 个文件）与视图调用关系，并对照后端路由注册表（internal/router/）。

### 3.1 整组无 Web 前端调用方的后端路由

| 路由组 | 前缀 | 功能 | 消费方 |
|---|---|---|---|
| IM 平台回调 | `/api/v1/im/callback/:channel_id`（公开） | 接收飞书/企业微信/钉钉/Slack/Telegram/QQ 机器人/个人微信/云之家/Mattermost 等平台的回调消息 | 各 IM 平台服务器。前端仅拼出该 URL **展示给管理员配置到 IM 平台**，从不调用 |
| Presigned 文件访问 | `/api/v1/files/presigned` | HMAC 签名免鉴权文件访问 | IM 平台渲染机器人回复中的图片 |
| 资源授权短链 | `/r/:token` | 短期可撤销资源 URL | 无法附加鉴权头的客户端（IM 等） |
| 评估 | `POST/GET /api/v1/evaluation` | 知识库检索问答质量评估（dataset + chat/rerank 模型） | Go SDK / CLI；前端无 HTTP 调用（仅作为 API Key 能力文案出现） |
| 解析诊断 | `/api/v1/files/presigned-preview` | 运维诊断端点 | 运维人员 |

### 3.2 同域内的散点未使用端点（该域其余端点被前端正常使用）

混合检索单库端点（hybrid-search，Go SDK / MCP Server 使用）、跨库复制（/knowledge-bases/copy，前端实际用 /duplicate）、清空知识库、解析阶段别名路由（/stages）、图片 OCR 更新、FAQ 相似问题生成、分块删除类、单条消息删除、组织 search-users（废弃别名）、`GET /tenants` 与 `GET /tenants/:id`（前端用 /tenants/all 与 /tenants/search）、`POST /auth/switch-tenant`（前端切空间经 `X-Tenant-ID` 头实现，未调用）、`GET /auth/oidc/start`（直连 302 场景保留）。以上均由 Go SDK（cli/）、MCP Server 或小程序消费。

### 3.3 前端隐藏、后端保留（品牌裁剪）

| 功能 | 说明 |
|---|---|
| WeKnora Cloud（云托管模型/解析） | v2.3 起设置页硬隐藏（`HIDDEN_SETTING_SECTIONS`），深链落"权限不足"页；后端 `/weknoracloud/credentials`、`/models/weknoracloud/status` 两条路由保留 |
| Claw Skill / ClawHub | v2.3 起集成页签移除（后端本无专属路由，原为外链落地页）；旧书签回退到 IM 页签 |

### 3.4 非 Web 客户端及其消费端点（不属 Web 端功能，但说明后端保留原因）

| 客户端 | 消费的代表性端点 |
|---|---|
| Go SDK / `cli/`（weknora CLI） | 评估、单库混合检索、分块/消息删除、知识库复制、租户列表详情等 |
| `mcp-server/`（MCP 工具服务，29 工具） | 知识库 CRUD、共享库查询、混合检索、知识入库 |
| `miniprogram/`（微信小程序） | 知识库列表、URL 入库、创建会话、快速问答 |
| `docreader`（文档解析服务） | gRPC 内部服务（无 HTTP 路由）；后端经 `/system/docreader/reconnect` 管理 |
| Chrome 浏览器插件 | 独立发行物；Web 端仅提供安装落地页（外链 Chrome 应用商店） |

### 3.5 反向差异（前端调用但后端未注册的端点，技术债提示）

- `POST /api/v1/knowledge-bases/:id/rebuild-index`：前端知识库设置弹窗调用，后端路由未注册（线上 404）——"重建索引"按钮当前不可用。
- `GET /api/v1/auth/tenant`：前端 API 层定义且无调用方，后端亦无此路由（双重死代码）。

---

## 4. 角色与权限（UI 可见性）

### 4.1 角色模型

| 角色 | 层级 | 说明 |
|---|---|---|
| 系统管理员（System Admin） | 平台级 | 管理平台设置、平台 API Key、运行时队列、系统审计、管理员账户；由 `POST /system/admin/promote` 授予 |
| Owner | 空间级 | 空间全部权限：成员管理、删除空间、API Key 与 Principal 配置 |
| Admin | 空间级 | 知识库/会话/模型等管理配置；除成员删除/空间删除外的管理操作 |
| Contributor | 空间级 | 内容维护：上传文档、编辑分块、维护 FAQ/Wiki |
| Viewer | 空间级 | 只读：检索、对话、浏览 Wiki |
| 组织角色 | 组织级 | 共享空间内的 Owner/Admin/Member（组织维度另有成员/加入审批体系） |

### 4.2 设置中心分区可见性（`config/settingsAccess.ts`）

设置中心是一个全屏弹窗（路由 `/platform/settings?section=<key>` 可深链），左侧导航按 7 组组织，空组自动隐藏：**账户 / 空间 / 模型 / 发布集成 / 数据与扩展 / 系统管理 / 平台**。可见性由三重过滤决定：① 角色门槛；② 部署能力开关；③ 硬隐藏清单（WeKnora Cloud、Claw Skill，见 §3）。深链到无权限/被隐藏分区时显示「权限不足」兜底页。

| 分区 key | 中文名称 | 可见性（v2.12 起） | 写权限（v2.9 起） |
|---|---|---|---|
| general | 通用设置 | viewer | 本人（个人偏好） |
| userprofile | 个人资料 | viewer | 本人 |
| mymemory | 我的记忆 | viewer | 本人 |
| tenant | 空间信息 | viewer | owner（空间自身管理） |
| members | 成员管理 | viewer（管理入口快捷方式要求 owner） | owner |
| models | 模型管理 | **仅系统管理员** | **系统管理员** |
| system | 系统信息 | viewer | —（只读） |
| ollama | Ollama 本地模型 | **仅系统管理员** | **系统管理员** |
| memory | 空间记忆配置 | admin | admin（维持空间管理） |
| chathistory | 消息管理 | admin | admin（维持空间管理） |
| websearch | 网络搜索 | **仅系统管理员** | **系统管理员** |
| vectorstore | 向量库 | **仅系统管理员** | **系统管理员** |
| parser | 解析引擎 | **仅系统管理员** | **系统管理员** |
| storage | 存储引擎 | **仅系统管理员** | **系统管理员** |
| sandbox | 沙箱 | **仅系统管理员** | **系统管理员** |
| mcp | MCP 服务 | **仅系统管理员** | **系统管理员** |
| integration-im / integration-embed / integration-api / integration-chrome | 集成中心各页签 | v2.8 起整组前端隐藏，后端保留 | 后端保留 |
| 检索配置（⌘K 命令面板，非设置分区） | 检索设置 | **仅系统管理员**（面板与触发入口隐藏） | **系统管理员** |

> **平台统一配置（v2.9 写权限 / v2.12 可见性）**：模型、Ollama、网络搜索、向量库、解析引擎、存储引擎、技能沙箱、MCP 服务八个分区及检索配置**整体仅系统管理员可见可改**（产品语言：「看得见 = 管得了」）；空间管理员/Owner 在知识库设置、对话、Agent 编辑器中的**选用**能力不受影响（选模型/选实例/选沙箱/选技能）。消息管理与长期记忆维持空间 Admin。API Key 能力体系不变。

系统管理员专属分区（`SYSTEM_ADMIN_SETTINGS_SECTIONS`）：`system-global`（平台设置与管理员）、`runtime-queues`（运行时队列）、`platform-api-keys`（平台 API Key）、`system-audit-log`（系统审计日志）。

### 4.3 权限模型补充说明

- 角色序：owner(40) > admin(30) > contributor(20) > viewer(10)；成员状态：active / invited / suspended。
- 双身份轴：空间内角色（按当前生效空间解析，随空间切换实时变化）与平台级系统管理员相互独立；跨租户超级管理员（can_access_all_tenants）在非成员空间临时映射为 admin，永不映射 owner（删除空间等 owner 专属操作保留给目标空间真实 owner）。
- 前端角色值仅用于 UI 渲染门控（localStorage 可被篡改），真实鉴权由后端每个请求的服务端角色解析 + RBAC 中间件兜底。
- 角色不足的兜底：设置分区显示「权限不足」页；组织/空间操作按钮按角色隐藏。

### 4.4 部署能力开关（`GET /system/capabilities`）

能力键与前端入口的对应关系（`supported=false` 时入口隐藏）：

| 能力键 | 控制的入口 |
|---|---|
| organizations | 共享空间菜单与组织页（Lite 版强制 false） |
| agents | 智能体页 |
| integrations.im | 集成中心 IM 页签 |
| integrations.embed | 集成中心嵌入页签 |
| integrations.api | 集成中心 API 页签 |
| settings.mcp / settings.websearch / settings.vectorstore / settings.storage / settings.sandbox | 对应设置分区 |

能力探测失败时前端 fail-open（保持可见），真正权限仍由后端接口校验。

---

## 5. 功能需求

> 每个功能点按「界面描述 → 用户操作 → 关键交互 → 涉及接口」组织；接口编号指向 §8 附录。

### 5.1 认证与账户

#### 5.1.1 登录 / 注册（`/login`、`/register`）

**界面**：品牌 Logo（智枢文渊）+ 动态"知识节点"背景动画（支持 reduced-motion 降级与明暗主题）；左侧产品展示区（副标题、RAG/Agent/Wiki/混合检索特性标签、4 页产品截图轮播）；右侧表单卡（登录卡 / 注册卡二选一）；顶部语言切换（简体中文 / English / Русский / 한국어）；带邀请 token 进入时显示邀请横幅。

**用户操作与规则**：

| 功能 | 规则/流程 |
|---|---|
| 邮箱密码登录 | 邮箱格式校验；密码 8–32 位且须含字母+数字；回车提交；按钮 loading |
| 自助注册 | 用户名 2–20 字符限定字符集 + 邮箱 + 密码 + 确认密码；部署为 invite_only 模式时隐藏注册入口；成功后回填邮箱切回登录卡 |
| OIDC 单点登录 | 启用时显示按钮（展示 IdP 显示名）；跳转 IdP 后回跳结果由应用消费 URL hash；跳转前暂存邀请 token，回调后自动兑换 |
| 邀请链接 | `?token=xxx`：校验 token → 已登录用户直接兑换入空间（token 失效仍放行进入应用）；未登录时 invite_only 停登录卡（登录后自动兑换），self_serve 走注册即自动登录 |
| 语言切换 | 四语言，写入 localStorage 并即时生效 |

**登录成功后**：持久化 user / token / refresh_token / 多空间成员关系（memberships）/ 能力位（can_create_tenant、auto_accept_invitation）；区分主空间（home tenant）与激活空间（active tenant），激活≠主时后续请求带 `X-Tenant-ID`。跳转：有空间 → 知识库列表；无空间 → 工作区初始化页。

**Lite/桌面版透明初始化**：进入登录页先尝试 auto-setup，成功即以 Lite 模式直进主界面（用户无感知）；失败则标记不再重试。

**接口**：§8.1。

#### 5.1.2 工作区初始化页（`/onboarding/workspace`）

- 居中卡片：标题/描述随 canCreateTenant 策略切换（可自助创建 vs 仅受邀加入）；策略加载失败带重试；invite-only 部署显示锁定提示条。
- 操作：创建工作区（弹窗表单）；查看我的邀请（弹窗，按钮带待处理数量徽标）；退出登录。
- 流程：创建成功后自动切换到新空间并跳转知识库列表。

#### 5.1.3 邀请加入（`/join`)

`/join?code=xxx` 重定向到共享空间列表页并携带 invite_code，由组织页接续加入流程（见 §5.12）。

#### 5.1.4 登录设备管理（v2.17，多端会话管理）

设置中心「账户」组新增「登录设备」分区（个人资产，全员可见自己的）：

- **会话列表**：当前账号的全部活跃登录会话（一次登录=一个会话，含设备描述、登录 IP、登录时间、最近活跃、有效期）；**本机高亮**「当前设备」标签。
- **踢下线**：任一非本机会话可一键踢除（确认弹窗 → 撤销该会话全部令牌 → 被踢设备下次请求 401 跳回登录页）；本机不提供踢除按钮（防自锁，后端双重保护）。
- **多端并存**：不做登录互踢——多设备同时在线是允许的，管理目标从"禁止"升级为"可见可控"。
- **边界**：API Key / Embed Widget 不受影响（独立鉴权体系）；改密码已有全端失效语义（上游既有）。
- 接口：§8.1 补充 `GET /auth/sessions`、`GET /auth/sessions/current`、`DELETE /auth/sessions/{session_id}`。

### 5.2 工作区初始化向导（模型与知识库初始化）

首次使用引导用户完成模型与知识库的初始化配置，全部操作围绕 `initialization` 接口族（§8.2）：

| 步骤 | 能力 |
|---|---|
| 模型配置 | 配置对话/Embedding/Rerank/ASR/多模态模型；远端模型连通性测试（remote/check、embedding/test、rerank/check、asr/check） |
| Ollama 本地模型 | 连接状态检测、模型列表、按名称下载模型（进度轮询）、已下载模型管理 |
| 文本加工测试 | text-relation / fabri-text / fabri-tag 抽取测试 |
| 初始化执行 | 保存配置（PUT config）即对目标知识库生效；旧版一键初始化接口已弃用（见 §8.16） |

配套新手引导体系：首访全屏引导（NewUserGuide，用户菜单可重开）+ 页面级分步引导（知识库列表、知识库详情、Agent 创建均有埋点）。

### 5.3 平台框架与导航

**左侧导航**（`/platform` 布局）：

| 菜单项 | 可见性 | 说明 |
|---|---|---|
| 新对话 | 全员 | 下方挂会话子列表：按时间分组、置顶、搜索、来源筛选（我的对话/API 会话）、重命名、删除、批量管理 |
| 知识库 | 全员 | 默认首页 |
| 智能体 | 能力 `agents` 开启 | 路由层校验，缺失时 toast 并回知识库列表 |
| 共享空间 | 能力 `organizations` 开启且用户角色 admin+；Lite 隐藏 | 组织入口 |
| 系统设置 | 全员（分区内容按角色过滤） | 打开设置中心弹窗 |
| 退出登录 | 非 Lite | — |

**顶部/全局**：空间切换器（切换后所有请求附 `X-Tenant-ID`，成员关系实时刷新）；用户菜单；待处理邀请铃铛（有待处理邀请时渲染，轻量轮询未读数）；全局命令面板（⌘K，支持旧路径 `/platform/knowledge-search?q=` 深链自动打开）；全局文件拖拽遮罩（拖入文件按当前页面分流：知识库详情页 → 文档上传，会话页 → 会话附件；Wiki 内部元素拖拽不拦截）；桌面端（Wails）Ctrl/Cmd+R 软刷新。

**路由守卫**：token 恢复会话（用户/空间/成员关系/能力位回填）；无有效空间强制跳 onboarding；部署能力校验；系统管理员门控；Lite 版会话内路径记忆（硬刷新恢复最后访问页）；OIDC 回跳放行。

**兼容重定向**：`/platform/tenant`、`/platform/integrations`、`/platform/system/*` 等旧 URL 全部归一跳转到设置中心对应分区，不构成独立页面。

### 5.4 知识库管理

#### 5.4.1 知识库列表（`/platform/knowledge-bases`，默认首页）

**界面**：
- 左侧空间侧栏（非 Lite 显示）：全部 / 我的空间 / 收藏 / 最近 / 按组织空间分组，各项带计数。
- 卡片网格，分段展示（可折叠、带计数）：置顶 / 我创建的 / 本空间·仅查看 / 共享给我·可编辑 / 共享给我·仅查看；空间视图下另有"我共享的"。
- 卡片信息：名称、Wiki 徽标、描述、类型徽标（文档数或 FAQ 条数，处理中转圈）、知识图谱徽标、多模态徽标、问题生成徽标、已共享数徽标、收藏星标、创建者来源徽标。
- 未初始化知识库提示横幅；按知识库聚合的上传进度面板（进度条、保持页面提示、错误提示）；加载骨架屏；各视图独立空态文案（全部/我的空态带"新建知识库"按钮）。
- 列表视图状态写入 URL（`?scope=`，viewer 默认"全部"、contributor+ 默认"我的"）。

**用户操作**：

| 操作 | 说明 |
|---|---|
| 新建知识库 | 弹窗创建，支持 文档库（document）/ FAQ 库（faq）两种类型 |
| 置顶/取消置顶 | 按用户维度，人人可用 |
| 复制知识库 | 满足条件时卡片菜单提供 |
| 设置 | 打开知识库设置弹窗（见 §5.4.3），可管理者可见 |
| 删除 | 二次确认对话框，可管理者可见 |
| 收藏/取消收藏 | 每用户维度 |
| 共享详情抽屉 | 共享卡片"查看详情"：来源类型（KB 共享 / Agent 注入）、来源空间或 Agent、Agent 知识库选择策略、共享时间、我的权限标签（owner/admin/editor/viewer）、前往知识库 |
| 上传进度查看 | 全局事件聚合上传进度，完成后自动刷新列表 |

#### 5.4.2 知识库详情（`/platform/knowledge-bases/:kbId`）

**界面**：顶部（KB 切换下拉、KB 信息浮层、面包屑——Wiki 库带"索引中"指示与队列数 tooltip、上传来源下拉、网格/列表视图切换、批量管理入口）；三页签 **文档 / Wiki / 图谱**（tab 深链 `?tab=`，图谱页支持 `&slug=` 从 Wiki 页面跳转定位）。

**文档页筛选工具条**：关键词搜索；标签多选筛选（搜索、分页、多选、清空、管理标签入口）；文件类型（PDF/DOCX/DOC/PPTX/PPT/EPUB/MHTML/TXT/MD/URL/手动/MP3/WAV/M4A/FLAC/OGG）；解析状态（待处理/处理中/已完成/失败/已取消/收尾中/草稿）；来源（上传/URL/手动/API/浏览器插件/飞书/飞书云盘/Notion/语雀/GitLab/ima/微信/企业微信/钉钉/Slack/IM）；更新时间区间（禁选未来日期）。

**文件夹树**（存在文件夹时显示）：可折叠（记忆状态）；"浏览"只列当前一层（子文件夹作为行内条目），"筛选"自动递归全部子目录。

**用户操作**：

| 类别 | 操作 |
|---|---|
| 入库 | 本地文件/文件夹上传、URL 导入、手动录入；上传前经**上传确认对话框**确认分块与处理参数；解析引擎可用性驱动可上传文件类型（缺失时引导跳解析引擎设置）；缺存储引擎时阻止上传并引导 |
| 单文档 | 查看正文/预览（docx/pptx/xlsx 等在线预览）、重新解析、取消解析、下载原件（Contributor+ 且共享权限 editor 及以上；Viewer 一律不可）、编辑标签、移动到文件夹（纯列更新不重解析）、跨知识库移动（"复用向量"或"重新解析"两种模式 + 进度轮询）、查看解析处理时间线（有 trace 时） |
| 批量 | 多选（支持框选拖拽）、批量删除、批量重解析（自动跳过进行中文档并提示；乐观置为待处理并轮询确认）、批量打标签（预选所选文档标签交集）、批量移动到文件夹 |
| 文件夹 | 树选择浏览、重命名（非法移动校验，重命名后停留新路径） |
| 标签 | 多选筛选/清除、标签管理抽屉（新建/编辑/删除，删除联动清空筛选） |
| 问答 | 从知识库发起对话（进入该 KB 上下文的新会话） |

**权限门控**：canEdit/canManage 区分"创建者 / 空间 Admin / 共享授权"；跨空间共享知识库只认共享授权（本地 admin 身份不放大权限）；移动/批量删除需 Contributor+。

**状态轮询策略**：文档解析状态按需轮询；Wiki 队列 5 秒轮询，且上传/重解析后退避探测（500ms/2s/5s/10s）尽快点亮"索引中"。

**分页**：滚动分页（首屏 35 条，按视口高度自适应增量）。

#### 5.4.3 知识库设置弹窗

创建/编辑共用同一弹窗（入口：列表卡片菜单、KB 详情页，支持指定分区直达）。分区：

| 分区 | 内容 |
|---|---|
| 模型配置 | 摘要模型 / Embedding 模型 / 多模态 VLM 模型；问题生成开关 |
| 解析器 | 解析引擎按文件类型的路由规则 |
| 分块 | 分块策略配置 + 分块实时调试（预览分块效果） |
| 索引策略 | 向量索引 / 关键词索引 / Wiki 生成 等开关 |
| 向量库 | 绑定向量库实例 |
| 存储 | 绑定存储后端实例 |
| 知识图谱 | 图谱抽取配置（启用后列表显示图谱徽标） |
| 共享 | 共享到组织空间（选择组织、角色、成员） |
| 活动记录 | 知识库操作动态（Owner/Admin 可见） |
| 数据源 | 外部数据源接入（见 §5.7） |
| 高级 | 其他高级参数 |

FAQ 库隐藏文档型设置项（文件夹树、文档上传等不适用项）。

#### 5.4.4 手动录入与正文渲染

- 手动知识编辑器：在线录入/编辑知识内容。
- 正文渲染：Markdown（含 Mermaid 图表、KaTeX 公式、代码高亮，DOMPurify 消毒）与 Office 文档在线预览（docx/pptx/xlsx）。

### 5.5 文档解析与分块管理

| 功能 | 说明 |
|---|---|
| 解析状态机 | 待处理 → 处理中 →（收尾中）→ 已完成 / 失败 / 已取消 / 草稿；列表实时轮询刷新 |
| 解析时间线 | Langfuse 风格 Span 树：逐阶段展示解析进度（分段、摘要、向量化等），支持解析中止 |
| 重新解析 | 单个/批量；批量重解析支持覆盖解析配置 |
| 分块编辑 | 可视化编辑检索单元（分块），保存后自动重建索引 |
| 分块版本历史 | 逐版本 diff 对比与一键回滚 |
| 分块推荐问题 | 为分块维护推荐问题（增删改、再生成） |
| 文档元数据 | 自定义元数据编辑、文档描述编辑、自动摘要再生成 |
| 分块调试 | 分块策略实时预览（选样本文档 → 预览分块效果） |

### 5.6 FAQ 管理

FAQ 类型知识库以全屏 FAQ 管理器为主界面（替代文档列表/文件夹树/上传）：

- 问答对列表：搜索、筛选（标签、状态）、分页展示。
- 条目维护：新增/编辑/删除问答对；标准问题与相似问题管理。
- 批量操作：批量删除、批量打标签。
- 导入导出：FAQ 批量导入（Excel/CSV，含导入进度追踪与导入结果展示）、导出（Excel/CSV）。
- 检索测试：FAQ 命中测试（验证查询能命中哪些问答对）。

### 5.7 数据源集成

在知识库设置中接入外部数据源，自动同步远端内容：

| 功能 | 说明 |
|---|---|
| 支持类型 | 飞书知识库、飞书云盘、Notion、语雀、GitLab、RSS 等（类型列表由后端下发，前端按类型渲染专属表单与图标） |
| 数据源管理 | 新建/编辑/删除数据源；凭据加密托管（更新/清除）；凭据连通性校验 |
| 资源浏览 | 远端资源树浏览（目录/文档下钻）、资源路径解析 |
| 同步控制 | 手动触发同步、暂停、恢复 |
| 同步日志 | 按数据源查看同步历史（状态、时间、错误信息） |
| 入库呈现 | 同步内容以文档形式进入知识库，"来源"筛选可按数据源类型过滤 |

### 5.8 Wiki

入口：知识库详情"Wiki"页签（索引策略开启 `wiki_enabled` 后可用）。Agent 从知识库原始文档自治生成相互链接的 Markdown Wiki 页面与知识图谱，并支持人工维护：

| 功能 | 说明 |
|---|---|
| Wiki 浏览器 | 目录树（文件夹/页面层级）、页面阅读与渲染（Markdown）、Wiki 内搜索 |
| 页面管理 | 页面新建/编辑/删除；文件夹新建/重命名/删除；页面在文件夹间移动 |
| 版本历史 | 页面快照列表、行级 diff 对比、一键回滚；支持浏览器内人工编辑保存 |
| 生成状态 | 队列状态展示（待处理任务数、"索引中"指示）；统计面板（页面数/问题数等） |
| 链接治理 | 链接问题清单（断链等）与状态处理；一键重建链接 |
| 知识图谱 | 图谱可视化页签（与 Wiki 页面双向跳转） |
| Agent 联动 | 对话中 Agent 的 Wiki 编辑操作以工具结果卡片回显（WikiEditResult） |

### 5.9 对话（核心）

#### 5.9.1 新建会话（`/platform/creatChat`、`/platform/knowledge-bases/:kbId/creatChat`）

- 居中输入区：多行输入、附件/图片上传（全站拖拽自动转为会话附件）、@ 提及（知识库/智能体/MCP 服务/技能）。
- 上下文选择：Agent 选择、知识库多选（KB 内发起时默认带当前知识库）；**对话模型选择仅系统管理员可见**（v2.14：普通用户不感知模型，会话由后端默认解析链自动落到 Agent 配置模型 / KB 摘要模型 / 首个可用对话模型）。
- 发送首条消息即创建会话并跳转 `/platform/chat/:chatid`（侧栏同步插入新会话，支持预填：问题文本、提及项、模型、图片、附件）。

#### 5.9.2 对话页（`/platform/chat/:chatid`）

**消息流**：用户消息气泡 / 机器人消息气泡；Markdown 流式渲染（代码高亮、Mermaid、KaTeX、表格、外链新开页）；图片大图预览。

**回答工具栏**：停止生成、重新生成、复制、一键导出 Markdown。

**引用与溯源**：引用浮层（悬浮预览）+ 引用抽屉（区分网络来源 / 知识库来源）；点击引用查看分块原文（ChunkDetail）与文档信息。

**RAG 流水线进度**：问答过程中分阶段展示检索流水线（意图理解 → 检索 → 重排 → 生成等阶段进度）。

**Agent 模式执行可视化**：
- 深度思考展示（可折叠思考内容）；
- 执行流展示（计划 PlanDisplay、逐工具调用与结果卡片）；
- 工具结果渲染器矩阵：知识检索结果、分块列表、相关分块、文档信息、网络搜索结果、网页抓取结果、Grep 结果、数据库查询、图谱查询、知识库列表、Shell 执行结果、Wiki 编辑结果等；
- **工具调用人工审批**：敏感工具调用弹出审批卡（允许/拒绝），MCP OAuth 授权卡（会话内完成远程服务授权）。

**记忆**：会话内记忆步骤展示（记忆写入/读取过程）。

**调试辅助**：请求信息按钮（查看本次请求元数据，X-Request-ID 关联）、模型调试抽屉。

**产物**：技能产物抽屉（按回复查看 Agent/技能产出的文件与结果，支持下载）。

#### 5.9.3 会话管理（侧栏）

| 功能 | 说明 |
|---|---|
| 列表组织 | 按时间分组（今天/近 7 天…）、置顶分组、搜索、来源筛选（我的对话 / API 会话 / IM / 嵌入等渠道，按管理员范围隔离）、分组方式切换 |
| 单会话操作 | 重命名（手动或 AI 生成标题）、置顶/取消、清空消息、删除 |
| 批量管理 | 批量选择删除（含清空全部） |
| 历史加载 | 消息按时间游标分页加载（上滑加载更早消息） |
| 推荐问题 | 首轮基于知识库生成推荐问题；答后生成追问建议；曝光/点击/忽略埋点上报 |

#### 5.9.4 会话临时附件

会话级临时上传图片/文档（base64 或上传后引用 attachment_id），异步解析后用于一次性问答；支持图片与附件合并限额；附件可预览、删除。

### 5.10 智能体管理

路由 `/platform/agents`（能力 `agents` 控制；缺失时入口隐藏）。**管理权限（v2.13）：创建/编辑/删除/复制收归空间 Admin+；普通成员（contributor/viewer）只读列表并使用智能体（对话中选用、@提及、共享智能体使用），不再拥有配置修改能力。**

| 功能 | 说明 |
|---|---|
| 智能体列表 | 卡片展示（头像、名称、描述）；新建入口（Admin+）+ 分步创建引导 |
| 编辑器（Admin+） | 大型编辑弹窗：基础信息（名称/头像/描述）、模型与参数（对话模型、思考模式等）、提示词与人设（含提示词模板选择）、开场白、知识库配置（知识库多选 + 选择策略 kb_selection_mode）、工具与 MCP 配置（工具开关、MCP 服务、审批策略）、网络搜索开关、发布渠道 |
| 复制（Admin+） | 一键复制现有智能体 |
| 删除（Admin+） | 删除智能体 |
| 共享到空间（Admin+） | 将智能体共享到组织空间（选择组织与成员范围）；共享后目标空间成员可在对话中选择该共享智能体 |
| 推荐问题 | 智能体级推荐问题配置读取 |
| 嵌入渠道（Admin+） | 管理该智能体的网站嵌入渠道（渠道配置 + 实时预览，见 §5.15） |
| IM 渠道（Admin+） | 管理该智能体的 IM 机器人渠道绑定与启停（渠道配置管理界面，见 §5.11） |

### 5.11 集成中心（发布集成）

入口：设置中心"发布集成"组（4 个页签）；侧边栏悬停预览同 4 项。页签可见性受部署能力开关控制；API 页签要求 Owner 角色。

| 页签 | 内容 |
|---|---|
| IM 集成 | IM 机器人渠道的接入配置与管理界面：支持渠道（企业微信/飞书/Lark/钉钉/Slack/Telegram/QQ 机器人/个人微信/云之家/Mattermost）；渠道绑定到智能体、启停开关、微信扫码绑定（二维码生成与状态轮询）；页面会拼出平台回调 URL **展示给管理员**配置到对应 IM 平台。仅覆盖**配置管理**；渠道的消息收发服务在后端运行，不属于 Web 前端范围 |
| 网页嵌入 | 嵌入渠道管理：生成/复制嵌入代码片段（挂载 Widget 的外站代码）、渠道参数配置、效果预览（见 §5.15） |
| API 集成 | 空间 API Key 管理：创建（勾选能力范围）、查看（明文仅显示一次）、吊销；KB 范围限制；**API 调试台（Playground）**：在页面内直接发起 SSE 流式调用验证 Key 权限与效果；API Principal 配置与调试令牌签发 |
| Chrome 插件 | 落地页：插件功能介绍 + 外链跳转 Chrome 应用商店安装（插件本体为独立发行物，不在本 PRD 范围） |

### 5.12 共享空间（组织）

路由 `/platform/organizations`（能力 `organizations` 控制；侧栏仅 admin+ 可见；Lite 隐藏）。

| 功能 | 说明 |
|---|---|
| 组织列表 | 我加入的组织、可发现的组织搜索、组织卡片（角色标签） |
| 创建组织 | 弹窗创建（名称、描述等） |
| 加入组织 | 邀请码加入（`/join?code=` 深链自动进入）、按 ID 直接加入、加入申请（需审批的组织）、邀请预览页 |
| 组织设置 | 成员管理（列表、角色分配 owner/admin/editor/viewer、移除）、加入申请审批、邀请成员（邮箱邀请/邀请码）、资源共享 |
| 退出/升级 | 退出组织；权限升级申请 |
| 知识库共享 | 将知识库共享到组织（指定角色）；组织维度共享列表管理；共享后目标空间以"共享给我"视图访问并按共享授权鉴权 |
| 智能体共享 | 将智能体共享到组织；组织管理员可禁用特定共享智能体（disabled 管理） |
| 空间搜索 | 组织内搜索可邀请的空间（search-tenants） |

### 5.13 设置中心

全屏弹窗（路由 `/platform/settings?section=`），左侧导航 7 组（账户 / 空间 / 模型 / 发布集成 / 数据与扩展 / 系统管理 / 平台），分区地图与角色门槛见 §4.2。

| 分区 | 功能要点 |
|---|---|
| 通用设置 | 界面语言（4 语言）、主题（浅色/深色/跟随系统）、界面字体与代码字体（实时预览）、字号；Lite 版额外提供"自动检查更新" |
| 用户信息 | 账户基础信息展示；修改密码 |
| 我的记忆 | 个人级记忆条目管理（查看/编辑/删除，跨空间跟随个人） |
| 空间信息 | 空间名称/描述编辑、存储配额与用量展示 |
| 成员管理 | 成员名册（搜索、分页、角色标签、状态）+ 邀请 + 空间审计日志三合一面板：按邮箱添加成员（未注册提示先注册）、修改角色（保护最后一个活跃 Owner）、移除成员、退出空间；邮箱邀请与多用途邀请链接（可复制、显示已加入人数、可撤销）、邀请状态机（待处理/已接受/已拒绝/已撤销/已过期）；个人邀请收件箱与头像铃铛（未决数轮询）；空间审计日志（游标分页，按动作/结果/操作人过滤，Admin+） |
| 模型管理 | 按类型（对话/Embedding/Rerank/多模态/ASR）分区配置模型：添加/编辑/删除模型、凭据加密托管、厂商选择（含内置厂商列表与 Logo）；viewer 可浏览，增删改需 admin |
| Ollama | 本地 Ollama 服务状态、服务地址、按名称下载模型（进度条）、已下载模型管理 |
| 消息管理 | 空间级消息索引开关与保留配置 |
| 长期记忆 | 空间级长期记忆总开关与整理配置（记忆条目/主题/文档管理、手动整合、导出） |
| 网络搜索 | 搜索供应商选择与自定义供应商管理（类型、凭据托管、连通性测试）、空间级启停配置 |
| 向量数据库 | 向量库实例管理（类型、连接参数、测试连通性、默认实例） |
| 解析引擎 | 解析引擎配置与状态检测、docreader 服务重连 |
| 存储引擎 | 多实例存储后端管理（类型、连接参数、测试、默认实例） |
| 技能沙箱 | 沙箱配置管理（运行窗口/工作区策略）、技能包管理（上传/安装/启停、安装时间线）、技能文件浏览、连通性检查 |
| MCP 服务 | MCP 服务列表管理：新建/编辑/删除、连接测试、工具与资源发现、工具调用审批策略配置、凭据托管、OAuth 授权（授权地址生成/状态查询/解除授权） |
| 版本信息 | 系统版本/构建信息只读展示 |

（发布集成 4 页签见 §5.11；系统管理 4 分区见 §5.14。）

### 5.14 系统管理（系统管理员专区）

`SYSTEM_ADMIN_SETTINGS_SECTIONS` 五个分区，仅系统管理员可见（UI 门控 + 后端中间件兜底）：

| 分区 | 功能要点 |
|---|---|
| 全局设置（system-global） | 平台级全局配置；系统管理员账户管理（列表、授予/撤销系统管理员、创建用户、重置密码）；平台设置键值管理；默认存储配额应用 |
| 账号管理（account-management） | 平台级账号治理（v2.19–v2.21）：全部账号目录（搜索/状态过滤/分页）、新建账号（自动生成密码一次性展示）、编辑资料（用户名/邮箱，唯一性校验）、启用/禁用（禁用即拒绝登录并撤销全部在线令牌）、跨空间成员角色调整与移出（最后 Owner 保护沿用）、查看任意账号的多端登录会话并逐条踢除或全部强制下线；自我保护——不能对自身账号执行任何写操作（后端 400 + 前端隐藏双保险）；全部写操作落平台审计日志 |
| 任务队列（runtime-queues） | 运行时任务队列监控：队列列表、任务明细（分阶段独立池、失败任务排查与重试）、分页浏览 |
| 平台 API Key（platform-api-keys) | 平台级 API Key 签发与管理：能力范围 = 系统能力组（7 项）+ 全部租户能力组（18 项） |
| 审计日志（system-audit-log） | 平台全局审计日志（游标分页、action/outcome/actor 过滤、详情抽屉） |

### 5.15 网站嵌入 Widget（Embed）

**管理端**（集成中心·网页嵌入页签 / Agent 嵌入渠道面板）：创建/编辑/删除嵌入渠道；安全模式 Token 交换；轮换 Embed Token；渠道参数配置（品牌/主题/欢迎语等）；嵌入代码片段生成与复制；渠道效果实时预览（EmbedChannelPreview）。

**前台 Widget**（独立入口 `embed.html`，路由 `/embed/:channelId`，非主 SPA）：

- 面向外部网站访客的**免登录**对话界面：消息气泡、流式回复、引用来源展示、多轮会话。
- 鉴权：`Authorization: Embed <token>` + `X-Embed-Session`（会话签名）+ `X-Embed-Visitor`（访客标识）；不走 JWT 刷新流程。
- 会话控制：访客可停止生成；管理配置驱动品牌与主题样式。
- 源站限制：按渠道配置的允许来源（origins）校验嵌入站点。

**接口**：§8.12、§8.14。

---

## 6. 关键用户旅程（端到端流程）

### 6.1 首次部署 → 首个问答

1. 管理员访问站点 → 登录页（或 Lite 模式 auto-setup 透明进入）→ 无空间时进入工作区初始化页创建空间。
2. 首次引导：配置对话/Embedding 模型（远端连通性测试，或 Ollama 本地模型下载）→ 系统初始化向导生成默认知识库。
3. 上传首个文档（上传确认对话框确认分块参数）→ 解析状态轮询 → 完成。
4. 侧栏"新对话" → 选择知识库 → 输入问题 → SSE 流式回答（RAG 流水线进度、引用浮层）。
5. 验收点：引用可点击查看分块原文；推荐问题/追问正常出现。

### 6.2 上传文档 → 分块维护 → 检索命中

1. 知识库详情 → 上传（本地/文件夹/URL/手动录入）→ 上传确认对话框（分块与处理参数）。
2. 文档列表实时轮询解析状态；解析时间线展示逐阶段进度（可中止）。
3. 完成后进入正文页：查看分块 → 编辑分块（乐观锁保存）→ 版本历史 diff → 需要时回滚。
4. 为分块维护推荐问题；打标签、移动文件夹。
5. 对话中提问验证命中；或全局 ⌘K 语义搜索验证。

### 6.3 Agent 模式多步推理与工具审批

1. 新建会话选择 Agent（或进入智能体页创建：模型/人设/知识库/工具与 MCP）。
2. 提问 → Agent 执行流展示：计划 → 逐工具调用卡片（知识检索/网络搜索/MCP 工具/Shell 等）。
3. 敏感工具调用弹出审批卡 → 用户允许/拒绝 → Agent 继续或终止。
4. MCP 远程服务首次使用时会话内完成 OAuth 授权卡流程。
5. 最终答案流式渲染 + 引用抽屉；产物抽屉查看技能产物并可下载。

### 6.4 生成 Wiki 并人工修订

1. 知识库设置开启 Wiki 生成（索引策略 `wiki_enabled`）。
2. 上传/重解析文档后，后端队列自动生成 Wiki 页面；面包屑显示"索引中"与队列数。
3. Wiki 页签浏览目录树 → 阅读/搜索页面 → 人工编辑保存 → 版本历史（行级 diff、一键回滚）。
4. 图谱页签查看知识图谱，点击节点跳回对应 Wiki 页面（slug 深链）。
5. 链接问题清单处理断链；必要时一键重建链接。

### 6.5 把智能体发布到外部网站

1. 智能体编辑器/列表 → 嵌入渠道面板 → 创建渠道 → 配置品牌与参数 → 预览确认。
2. 复制嵌入代码片段 → 粘贴到外部网站。
3. 访客打开外部网站 → Widget（`/embed/:channelId`）免登录对话：安全模式下发布 token 换短时会话 token，流式回答 + 引用。
4. 渠道统计查看会话量；必要时轮换 Embed Token（旧 token 立即失效）。

### 6.6 邀请协作与共享治理

1. 空间 Owner：设置 → 成员管理 → 邮箱邀请或生成多用途邀请链接（显示已加入人数，可撤销）。
2. 被邀请人：头像铃铛收到邀请（2 分钟内轮询可达）→ 接受/拒绝；或邀请链接直接注册加入。
3. 知识库/智能体共享到组织 → 目标空间以"共享给我"视图访问，按共享授权（owner/admin/editor/viewer）鉴权（本地 admin 身份不放大权限）。
4. 审计：成员页审计日志记录成员变动（rbac.* 动作）；KB 活动记录供 Owner/Admin 追溯。

---

## 7. 非功能需求

| 类别 | 需求 |
|---|---|
| 流式体验 | 对话首字时间（TTFB）前端埋点（X-Request-ID 关联后端日志）；流式渲染按缓冲间隔批量上屏；支持随时停止（AbortController）；断线/刷新后对未完成回复自动续流（continue-stream）；前端无 WebSocket，长任务进度一律 HTTP 轮询（解析 spans、导入进度、下载进度等） |
| 会话恢复 | 页面刷新后从 localStorage 恢复会话（token/refresh token/激活空间/语言/Lite 标记）；Lite 模式恢复最后访问路径 |
| 安全 | Token 存 localStorage 并随请求头传递；401 自动刷新并队列化重放；Embed 渠道独立鉴权体系（Embed token → 安全模式换取短时会话 token + 会话签名 + 访客标识）；Markdown 渲染经 DOMPurify 消毒；受保护文件经鉴权代理访问（非直链）；API Key/数据源/MCP 凭据加密托管、凭据字段不回显明文 |
| 多语言 | zh-CN / en-US / ru-RU / ko-KR 四语言包；语言偏好随请求头（Accept-Language）传递并持久化 |
| 大数据量 | 会话消息 before_time 游标分页；审计日志游标分页；文档列表滚动分页（首屏 35 条自适应增量）；组织列表缓存协调（并发去重 + 失效递增，防旧响应覆盖新状态） |
| 长任务交互 | 上传进度（XHR 事件）、解析进度（spans 轮询）、FAQ 导入进度、Ollama 下载进度、跨库移动进度、技能安装进度（SSE）均有对应的可视化反馈 |
| 超时策略 | 默认 30s；例外：数据源资源树/祖先解析 120s、图谱文本抽取 60s、模型调试 300s、技能包上传 5min、技能源码安装 2min |
| 部署形态 | 单机 Docker Compose 一键启动（start_zhishu.bat / stop_zhishu.bat）；Lite 模式（单用户、无共享空间、SQLite）与完整模式（多空间 RBAC）同一前端按能力探测自适应；桌面端（Wails）与 LocalHub 反代（/app/weknora 基路径）复用同一 SPA |

---

## 8. 附录：前后端接口清单

> 仅收录 Web 前端（含 Embed Widget）实际调用的接口，按业务域分组。`{id}` 类为路径参数。流式/SSE 接口单独标注。

### 8.1 认证与账户

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | /api/v1/auth/login | 账号密码登录 |
| POST | /api/v1/auth/register | 注册（invite_only 部署受限） |
| POST | /api/v1/auth/auto-setup | Lite 模式自动建号登录 |
| POST | /api/v1/auth/refresh | 刷新 token |
| POST | /api/v1/auth/logout | 退出登录 |
| GET | /api/v1/auth/me | 当前用户/空间/成员关系/能力 |
| PUT | /api/v1/auth/me/preferences | 更新个人偏好 |
| POST | /api/v1/auth/change-password | 修改密码 |
| GET | /api/v1/auth/config | 登录页配置（注册开关等） |
| GET | /api/v1/auth/oidc/url | 获取 OIDC 跳转地址 |
| GET | /api/v1/auth/oidc/config | OIDC 配置查询 |
| POST | /api/v1/auth/invitations/lookup | 邀请令牌查验（匿名） |
| POST | /api/v1/auth/register-by-invite | 凭邀请注册 |

### 8.2 初始化向导（Workspace Onboarding）

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | /api/v1/initialization/config/{kbId} | 读取初始化配置（调用待确认） |
| PUT | /api/v1/initialization/config/{kbId} | 保存初始化配置 |
| POST | /api/v1/initialization/multimodal/test | 多模态（VLM+存储）测试 |
| POST | /api/v1/initialization/remote/check | 远端模型连通性校验 |
| POST | /api/v1/initialization/embedding/test | Embedding 模型测试 |
| POST | /api/v1/initialization/rerank/check | Rerank 模型测试 |
| POST | /api/v1/initialization/asr/check | ASR 模型测试 |
| POST | /api/v1/initialization/extract/text-relation | 文本关系抽取测试 |
| POST | /api/v1/initialization/extract/fabri-text | 文本加工测试 |
| POST | /api/v1/initialization/extract/fabri-tag | 标签加工测试 |
| GET | /api/v1/initialization/ollama/status | Ollama 连接状态 |
| GET | /api/v1/initialization/ollama/models | Ollama 模型列表 |
| POST | /api/v1/initialization/ollama/models/check | 模型存在性检查 |
| POST | /api/v1/initialization/ollama/models/download | 下载 Ollama 模型 |
| GET | /api/v1/initialization/ollama/download/progress/{taskId} | 下载进度轮询 |
| GET | /api/v1/initialization/ollama/download/tasks | 下载任务列表 |

### 8.3 知识库与文档

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | /api/v1/knowledge-bases | 创建知识库 |
| GET | /api/v1/knowledge-bases/{id}/activity | 知识库活动审计 |
| PUT | /api/v1/knowledge-bases/{id} | 更新知识库 |
| DELETE | /api/v1/knowledge-bases/{id} | 删除知识库 |
| POST | /api/v1/knowledge-bases/{id}/rebuild-index | 重建索引（⚠️ 后端未注册该路由，见 §3.5） |
| POST | /api/v1/knowledge-bases/{id}/duplicate | 知识库复制（库内复制） |
| GET | /api/v1/knowledge-bases/{sourceKbId}/move-targets | 可移动目标查询 |
| POST | /api/v1/knowledge/move；GET /api/v1/knowledge/move/progress/{taskId} | 知识迁移与进度 |
| PUT | /api/v1/knowledge-bases/{id}/pin | 知识库置顶 |
| POST | /api/v1/knowledge-bases/{kbId}/knowledge/file | 文件上传（multipart，带进度） |
| POST | /api/v1/knowledge-bases/{kbId}/knowledge/url | 网页 URL 入库 |
| POST | /api/v1/knowledge-bases/{kbId}/knowledge/manual | 手工文本入库 |
| GET | /api/v1/knowledge-bases/{kbId}/knowledge | 文档列表（分页/筛选） |
| GET | /api/v1/knowledge/batch | 批量查询知识 |
| PUT | /api/v1/knowledge/manual/{id} | 编辑手工知识 |
| POST | /api/v1/knowledge/{id}/reparse | 重新解析 |
| POST | /api/v1/knowledge/{id}/cancel-parse | 中止解析 |
| POST | /api/v1/knowledge/batch-reparse | 批量重解析 |
| GET | /api/v1/knowledge/{id}/spans | 解析追踪时间线 |
| DELETE | /api/v1/knowledge/{id}；POST /api/v1/knowledge/batch-delete | 删除/批量删除 |
| GET | /api/v1/knowledge/{id}/download、/preview | 原文件下载 / 预览（Blob） |
| GET/PUT | /api/v1/chunks/{knowledgeId}/{chunkId} | 分块列表读取 / 分块编辑 |
| GET/POST | /api/v1/chunks/{knowledgeId}/{chunkId}/revisions、/revert | 分块版本历史与回滚 |
| PUT | /api/v1/knowledge/{knowledgeId}（custom_metadata / description） | 文档自定义元数据/描述 |
| POST | /api/v1/knowledge/{knowledgeId}/regenerate-summary | 重新生成摘要 |
| GET | /api/v1/chunks/by-id/{chunkId} | 引用定位单分块 |
| PUT/DELETE/POST | /api/v1/chunks/by-id/{chunkId}/questions… | 分块推荐问题管理/再生成 |
| GET/POST/PUT/DELETE | /api/v1/knowledge-bases/{kbId}/tags… | 标签管理 |
| PUT | /api/v1/knowledge/tags；/api/v1/knowledge-bases/{kbId}/faq/entries/tags | 批量打标签（文档/FAQ） |
| GET | /api/v1/knowledge/search | 知识关键字搜索（@提及联想） |
| POST | /api/v1/knowledge-search | 跨库语义搜索（⌘K 命令面板） |
| GET/POST/PUT | /api/v1/knowledge-bases/{kbId}/knowledge/folders…；POST /api/v1/knowledge/folder | 文件夹树管理 |
| POST | /api/v1/chunker/preview | 分块策略实时预览（调试） |

### 8.4 FAQ 管理

| 方法 | 路径 | 用途 |
|---|---|---|
| GET/POST | /api/v1/knowledge-bases/{kbId}/faq/entries、/faq/entry | FAQ 列表/新建（含批量导入） |
| PUT | /api/v1/knowledge-bases/{kbId}/faq/entries/{entryId}、/faq/entries/fields | 编辑/批量修改字段 |
| DELETE | /api/v1/knowledge-bases/{kbId}/faq/entries | 批量删除 |
| POST | /api/v1/knowledge-bases/{kbId}/faq/search | FAQ 检索测试 |
| GET | /api/v1/knowledge-bases/{kbId}/faq/entries/export | FAQ 导出（Blob） |
| GET | /api/v1/faq/import/progress/{taskId} | 导入进度轮询 |
| PUT | /api/v1/knowledge-bases/{id}/faq/import/last-result/display | 导入结果展示标记 |

### 8.5 数据源集成

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | /api/v1/datasource/types | 支持的数据源类型 |
| GET/POST/PUT/DELETE | /api/v1/datasource… | 数据源 CRUD |
| POST | /api/v1/datasource/validate-credentials、/{id}/validate | 凭据校验 |
| GET | /api/v1/datasource/{id}/resources | 远端资源树浏览 |
| POST | /api/v1/datasource/{id}/resource-ancestors | 资源路径解析 |
| POST | /api/v1/datasource/{id}/sync、/pause、/resume | 同步控制 |
| GET | /api/v1/datasource/{id}/logs | 同步日志 |
| PUT/DELETE | /api/v1/datasource/{id}/credentials… | 凭据托管更新/删除 |

### 8.6 Wiki

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | /api/v1/knowledgebase/{kbId}/wiki/pages、/folders | 页面/文件夹列表 |
| POST/PUT/DELETE | /api/v1/knowledgebase/{kbId}/wiki/folders… | Wiki 文件夹管理 |
| PUT | /api/v1/knowledgebase/{kbId}/wiki/move-page | 页面移动 |
| POST/GET/PUT/DELETE | /api/v1/knowledgebase/{kbId}/wiki/pages… | 页面创建/读取/编辑/删除 |
| GET | /api/v1/knowledgebase/{kbId}/wiki/revisions/{slug}（?version=） | 版本列表/单版本 |
| POST | /api/v1/knowledgebase/{kbId}/wiki/revert | 一键回滚 |
| GET | /api/v1/knowledgebase/{kbId}/wiki/index、/graph、/stats | 索引/图谱/统计 |
| GET | /api/v1/knowledgebase/{kbId}/wiki/search | Wiki 搜索 |
| GET/PUT | /api/v1/knowledgebase/{kbId}/wiki/issues… | 链接问题清单与状态 |
| POST | /api/v1/knowledgebase/{kbId}/wiki/rebuild-links | 重建链接 |

### 8.7 对话与会话

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | /api/v1/sessions | 创建会话 |
| GET | /api/v1/sessions | 会话列表（搜索/置顶/来源筛选） |
| GET | /api/v1/sessions/{id} | 会话详情 |
| PUT | /api/v1/sessions/{id} | 更新会话（重命名等） |
| DELETE | /api/v1/sessions/{id}；POST /api/v1/sessions/batch | 删除/批量删除（含 delete_all） |
| POST | /api/v1/sessions/{id}/pin（DELETE 同路径） | 置顶/取消置顶 |
| POST | /api/v1/sessions/{id}/generate_title | 生成会话标题 |
| POST | /api/v1/knowledge-chat/{session_id} | **RAG 快速问答（SSE 流式）** |
| POST | /api/v1/agent-chat/{session_id} | **Agent 模式问答（SSE 流式）** |
| GET | /api/v1/sessions/continue-stream/{session_id}?message_id= | **断线/刷新后对未完成回复续流（SSE）** |
| GET | /api/v1/messages/{session_id}/load（before_time 游标） | 历史消息分页加载 |
| POST | /api/v1/sessions/{id}/stop | 停止生成 |
| DELETE | /api/v1/sessions/{id}/messages | 清空会话消息 |
| POST | /api/v1/sessions/{sid}/attachments | 会话临时附件上传（multipart，带进度；上传后轮询解析状态） |
| GET | /api/v1/sessions/{sid}/messages/{mid}/artifacts（+/download） | 单条回复的技能产物列表与下载 |
| GET | /api/v1/messages/chat-history-stats；POST /api/v1/messages/search | 消息统计/跨会话消息搜索 |
| POST/GET | /api/v1/sessions/{sid}/messages/{mid}/suggestions | 追问/推荐问题生成与读取 |
| POST | /api/v1/sessions/{sid}/suggestion-events | 推荐问题曝光/点击/忽略埋点 |
| GET/DELETE | /api/v1/sessions/{sid}/attachments/{aid}… | 会话临时附件（状态轮询/预览/删除） |
| GET/PUT | /api/v1/tenants/kv/chat-history-config | 消息索引配置 |

### 8.8 智能体 / MCP / 技能

| 方法 | 路径 | 用途 |
|---|---|---|
| GET/POST | /api/v1/agents… | 智能体列表/创建；另含 PUT/DELETE 更新与删除、POST /{id}/copy 复制 |
| GET | /api/v1/agents/placeholders、/agents/type-presets | 预设占位符与类型模板 |
| GET | /api/v1/agents/{id}/suggested-questions | 智能体级推荐问题配置 |
| GET/POST/PUT/DELETE | /api/v1/agents/{agentId}/shares…；GET /api/v1/shared-agents | 智能体共享到组织 |
| GET/POST/PUT/DELETE | /api/v1/mcp-services… | MCP 服务 CRUD/测试/工具与资源发现 |
| GET/PUT/DELETE | /api/v1/mcp-services/{id}/tool-approvals…、/credentials… | 工具审批策略/凭据托管 |
| POST/GET/DELETE | /api/v1/mcp-services/{id}/oauth… | MCP OAuth 授权链路 |
| POST | /api/v1/agent/tool-approvals/{pendingId} | 会话内工具调用审批（同意/拒绝） |
| POST | /api/v1/agent/mcp-oauth-resolutions/{pendingId}… | 会话内 MCP OAuth 授权处理 |
| GET | /api/v1/skills | 技能清单（@Skill 提及） |
| GET/POST/PUT/DELETE | /api/v1/sandbox-configs… | 沙箱配置与技能文件管理（见 §5.14） |

### 8.9 模型管理

| 方法 | 路径 | 用途 |
|---|---|---|
| GET/POST/PUT/DELETE | /api/v1/models… | 模型 CRUD（按类型分区） |
| GET | /api/v1/models/providers?model_type= | 模型厂商列表 |
| PUT/DELETE | /api/v1/models/{id}/credentials… | 模型凭据托管 |
| POST | /api/v1/models/{id}/debug | 模型调试（multipart，长超时） |

### 8.10 空间（租户）与成员

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | /api/v1/tenants/search | 空间搜索（切换器联想） |
| POST/PUT/DELETE | /api/v1/tenants… | 创建/更新/删除空间 |
| GET/POST/PUT/DELETE | /api/v1/tenants/{id}/members… | 成员列表/添加/改角色/移除 |
| POST | /api/v1/tenants/{id}/leave | 退出空间 |
| GET/POST/DELETE | /api/v1/tenants/{id}/invitations… | 邮箱邀请管理（列表/撤销） |
| POST | /api/v1/tenants/{id}/invite-links | 生成多用途邀请链接 |
| GET | /api/v1/me/invitations（/pending-count） | 我的邀请收件箱与角标数（轮询） |
| POST | /api/v1/me/invitations/{id}/accept、/decline、/accept-by-token | 接受/拒绝/凭链接 token 加入 |
| GET/POST/PUT/DELETE | /api/v1/tenants/{id}/api-keys… | 空间 API Key 管理 |
| GET/PUT | /api/v1/tenants/{id}/api-principal-config；POST /api/v1/tenants/{id}/api-principal-test-token | API Principal 配置与调试令牌 |
| GET | /api/v1/tenants/{id}/audit-log | 空间审计日志（Admin+，游标分页） |
| GET/PUT | /api/v1/tenants/kv/* | 空间级 KV 配置（检索/解析引擎/存储引擎/记忆/消息索引/提示词模板等） |

### 8.11 共享空间（组织）

| 方法 | 路径 | 用途 |
|---|---|---|
| GET/POST/PUT/DELETE | /api/v1/organizations… | 组织 CRUD/搜索 |
| POST | /api/v1/organizations/join、/join-request、/join-by-id、/preview/{code} | 加入组织系列 |
| POST | /api/v1/organizations/{id}/leave、/invite、/invite-code、/request-upgrade | 退出/邀请/邀请码/升级申请 |
| GET/PUT/DELETE | /api/v1/organizations/{id}/members… | 组织成员管理 |
| GET/PUT | /api/v1/organizations/{id}/join-requests… | 加入审批 |
| GET/POST/PUT/DELETE | /api/v1/knowledge-bases/{kbId}/shares…；GET /api/v1/shared-knowledge-bases | 知识库共享与共享列表 |
| GET/POST/PUT/DELETE | /api/v1/organizations/{id}/shares…、/shared-agents…、/agent-shares | 组织共享总览与智能体共享治理 |

### 8.12 发布集成（Embed / IM 渠道管理）

| 方法 | 路径 | 用途 |
|---|---|---|
| GET/POST/PUT/DELETE | /api/v1/agents/{agentId}/embed-channels、/api/v1/embed-channels… | Embed 渠道 CRUD |
| POST | /api/v1/embed-channels/{id}/rotate-token | 轮换 Embed Token |
| GET | /api/v1/embed-channels/{id}/stats | 渠道会话统计 |
| POST | /api/v1/embed-channels/{id}/preview-session | 管理端预览用短时会话 |
| POST | /api/v1/embed/{channelId}/sessions/{sessionId}/stop | Embed 会话停止生成 |
| GET/POST/PUT/DELETE | /api/v1/agents/{agentId}/im-channels、/api/v1/im-channels…、/{id}/toggle | IM 渠道绑定与开关（仅配置管理） |
| GET | /api/v1/im-channels | 全空间 IM 渠道总览（侧栏/菜单） |

### 8.13 系统管理与平台治理（System Admin）

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | /api/v1/system/capabilities | 部署能力探测 |
| GET | /api/v1/system/info | 版本/构建信息 |
| GET | /api/v1/system/admin/list | 系统管理员/用户列表 |
| POST | /api/v1/system/admin/promote、/revoke | 授予/撤销系统管理员 |
| POST | /api/v1/system/admin/users/create、/users/reset-password | 创建用户/重置密码 |
| GET/POST/DELETE | /api/v1/system/admin/api-keys… | 平台 API Key 管理 |
| GET/PUT/DELETE | /api/v1/system/admin/settings… | 平台全局设置读写 |
| POST | /api/v1/system/admin/tenants/apply-default-storage-quota | 应用默认存储配额 |
| GET | /api/v1/system/admin/runtime/queues… | 运行时队列与任务监控 |
| GET | /api/v1/system/admin/audit-log | 系统审计日志 |
| GET/POST | /api/v1/system/parser-engines…、/docreader/reconnect | 解析引擎配置与重连 |
| GET/POST | /api/v1/system/storage-engine-status、/storage-engine-check | 存储引擎状态/连通性 |
| GET/POST/PUT/DELETE | /api/v1/storage-backends…、/test | 多实例存储后端管理 |
| GET/POST/PUT/DELETE | /api/v1/vector-stores…、/test | 向量库实例管理 |
| GET/POST | /api/v1/web-search/providers、/web-search-providers… | 网络搜索供应商与凭据 |
| GET/POST/PUT/DELETE | /api/v1/sandbox-configs…（含 /skills、/files） | 沙箱配置/技能包/技能文件 |
| POST | /api/v1/system/sandbox-check | 沙箱连通性检查 |
| GET/POST/PUT/DELETE | /api/v1/memory/* | 记忆条目/主题/文档/整合/导出 |
| GET/POST/PUT/DELETE | /api/v1/user/favorites… | 收藏 |

### 8.14 Embed Widget 前台（独立鉴权域）

Embed Widget 使用独立入口（`embed.html` + `/embed/:channelId` 路由），鉴权方式为 `Authorization: Embed <token>`（安全模式下先经 `exchange` 用发布 token 换取短时会话 token `ems_*`），辅以 `X-Embed-Session`（会话签名）与 `X-Embed-Visitor`（访客标识）请求头；所有 `/api/v1/embed/` 路径的 401 不触发 JWT 刷新流程。

| 方法 | 路径（前缀 /api/v1/embed/{channelId}） | 用途 |
|---|---|---|
| GET | /config | 渠道公开配置（品牌/主题/欢迎语等） |
| POST | /exchange | 发布 token 换短时会话 token（安全模式） |
| POST | /sessions | 创建访客会话（返回会话 id + 签名） |
| GET | /messages/{sid}/load | 历史消息分页 |
| POST | /agent-chat/{sid}、/knowledge-chat/{sid} | **访客流式问答（SSE，channel="embed"）** |
| POST | /sessions/{sid}/stop | 停止生成 |
| GET | /chunks/{chunkId} | 引用分块内容 |
| GET | /suggested-questions | 开场推荐问题 |
| GET/POST | /sessions/{sid}/messages/{mid}/suggestions | 追问建议 |
| POST | /sessions/{sid}/suggestion-events | 建议埋点 |
| POST | /sessions/{sid}/events | 外部事件中继（best-effort） |
| POST | /sessions/{sid}/tool-approvals/{pendingId} | 工具调用审批 |
| POST/GET | /sessions/{sid}/mcp-oauth-resolutions…、/mcp-services/{svc}/oauth… | 访客侧 MCP OAuth 授权 |
| GET | /files?file_path= | 受保护图片/文件代理 |

### 8.15 受保护文件代理

文档内图片等受保护资源不走直链，前端用原生 fetch 拉取为 Blob URL 渲染（登录态带 Bearer + X-Tenant-ID；Embed 态带 Embed token）：

| 路径 | 场景 |
|---|---|
| /files?file_path={provider://…} | 本站登录态通用文件代理 |
| /api/v1/knowledge-bases/{kbId}/files?file_path=… | 跨空间共享知识库资源 |
| /api/v1/sessions/{sid}/messages/{mid}/files?file_path=… | 共享智能体回复中的源空间资源 |
| /api/v1/embed/{ch}/files?file_path=… | Embed 访客资源 |

### 8.16 前端已不使用 / 遗留的后端接口（不计入产品功能）

> 以下接口在前端代码中存在封装或由上游遗留，但**没有现役调用方**（或已被替代实现取代），不代表任何前端功能，供后端能力盘点参考：

| 接口 | 状态 |
|---|---|
| POST /api/v1/initialization/initialize/{kbId} | 旧版一键初始化，无调用方（前端走分步配置 + PUT config） |
| POST /api/v1/knowledge-chat/{sid}、/agent-chat/{sid}（postChat 封装版本） | 遗留封装无调用方；同路径端点本身经 SSE 封装（streame.ts）在用 |
| GET /api/v1/sessions/{sid}/artifacts | 会话级产物列表，无调用方（实际用消息级 artifacts） |
| POST /api/v1/knowledge-bases/copy | 跨库复制封装无调用方（实际用 /{id}/duplicate） |
| GET /api/v1/auth/tenant、GET /api/v1/auth/validate | API 层存在，现役调用方未确认 |
| GET /api/v1/tenants/all | 已标记 deprecated |
| GET/PUT /api/v1/tenants/kv/web-search-config、GET /api/v1/web-search/providers | web-search.ts 标记 deprecated（由 /web-search-providers 体系取代） |
| POST /api/v1/wechat/qrcode、/wechat/qrcode/status | 微信扫码相关，现役调用方未确认 |
| settings 组件 StorageEngineSettings.vue | 前端死代码：Settings.vue 的 import 实际指向 StorageBackendSettings.vue |

前端请求封装层另有一个无调用方的 `postChat` 导出函数（Content-Type: text/event-stream 的普通 POST），真实流式一律走 fetch-event-source 封装。

---

## 9. 修订记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-10-08 | 首版发布。基于代码走查（前端 28 个 API 模块、全部视图路由、后端路由注册表）产出：产品概述与边界（§1–§3）、角色权限（§4）、15 个功能模块（§5）、6 条端到端旅程（§6）、非功能需求（§7）、接口清单（§8.1–§8.16，含前端已不使用的遗留接口盘点） |
| v1.1 | 2026-10-09 | 权限体系更新（配合 v2.8–v2.10 三次系统改造）：§3 产品边界补「发布集成」整组前端隐藏；§4.2 设置分区表新增「写权限」列——模型/Ollama/网络搜索/向量库/解析引擎/存储引擎/沙箱/MCP/检索配置九域写权限收归系统管理员（平台统一配置），空间侧保留只读与 KB 级选用；§4.4 能力开关与 §5.13 设置中心描述同步 |
| v1.2 | 2026-10-10 | 可见性收口（配合 v2.12）：§4.2 平台统一配置八分区与 ⌘K 检索配置改为**仅系统管理员可见**（原「只读可见」方案按用户反馈升级为整体隐藏，产品语言「看得见 = 管得了」）；空间侧选用能力说明不变 |
| v1.3 | 2026-10-10 | 业务资产与对话体验收口（配合 v2.13–v2.14）：§5.10 智能体管理权（创建/编辑/删除/复制/共享/渠道）收归空间 Admin+，普通成员只读+使用；§5.9.1 对话模型选择仅系统管理员可见，普通用户由后端默认解析链（Agent 配置 → KB 摘要模型 → 首个可用对话模型）自动命中管理员配置的模型 |
| v1.4 | 2026-10-10 | 登录设备管理（配合 v2.15–v2.17 多端会话管理）：§5.1 新增 5.1.4 小节——设置中心「账户」组新增登录设备分区，多端并存前提下会话可见可控（查看活跃会话/踢下线/本机保护）；API Key 与 Embed 鉴权域不受影响 |
| v1.5 | 2026-10-10 | 账号管理（配合 v2.19–v2.21 平台级账号治理）：§5.14 系统管理组新增「账号管理」分区（四分区→五分区）——全部账号目录/新建/资料编辑/启用禁用（禁用即拒绝登录+撤销全部在线令牌）/跨空间角色调整/多端会话查看与强制下线；删除账号明确后置（决策：先禁用后删除）；自我保护（不能对自身写操作）与平台审计留痕 |

### 已知技术债提示（非功能承诺，供迭代参考）

1. `POST /knowledge-bases/:id/rebuild-index` 前端调用但后端未注册——"重建索引"按钮当前不可用（§3.5）。
2. 前端存在少量无调用方的 API 封装与 1 个死代码设置组件（§8.16），建议随版本清理。
3. WeKnora Cloud 入口为"硬隐藏"而非能力开关控制；若长期舍弃建议后端路由一并下线，若保留建议纳入 capabilities 机制。
4. 前端菜单/角色均以 localStorage 值做 UI 门控，服务端 RBAC 为真实边界——对外发布与二次开发时应保持该原则。
