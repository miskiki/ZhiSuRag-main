<p align="center">
  <picture>
    <img src="./docs/images/zhishu-logo.png" alt="智枢文渊 Logo" height="110"/>
  </picture>
</p>

<p align="center">
    <a href="./LICENSE">
        <img src="https://img.shields.io/badge/License-MIT-ffffff?labelColor=d4eaf7&color=2e6cc4" alt="License">
    </a>
    <a href="./CHANGELOG.md">
        <img alt="版本" src="https://img.shields.io/badge/version-0.7.2-2e6cc4?labelColor=d4eaf7">
    </a>
</p>

<p align="center">
| <a href="./README.md"><b>English</b></a> | <b>简体中文</b> |
</p>

# 智枢数据库知识管理系统（智枢文渊 · ZhiShu）

## 📌 项目介绍

**智枢数据库知识管理系统**（智枢文渊，ZhiShu）是一款基于开源大语言模型（LLM）知识管理框架二次开发的企业级知识管理系统。

系统以「让企业里的每一份文档都能被提问」为目标，围绕知识库构建了完整的文档加工与消费链路：支持 PDF、Word、Excel、图片、Markdown 等十余种格式的文档导入，文档经解析、分块与向量化后进入知识库，用户可以通过**自然语言问答**直接获取带引用来源的答案，也可以让**智能体（Agent）**自主编排知识检索、MCP 工具与网络搜索来完成复杂的多步任务。

在知识治理层面，系统提供树形文件夹组织、批量导入与打标签、分块在线编辑、多源数据同步（飞书、Notion、语雀、RSS 等）能力，并结合企业级多空间权限管控（四级角色矩阵、资源归属、审计日志与平台级账号管理），让不同团队在各自的空间内安全地沉淀与管理知识。系统同时支持网站嵌入 Widget 与 IM 渠道接入，把问答能力发布到企业已有的工作入口；兼容 OpenAI、DeepSeek、Qwen、智谱、Ollama 等主流模型厂商，大模型、向量数据库与存储后端均可灵活替换，支持本地与私有云部署，数据完全自主可控。

## 🚀 快速开始

### 🛠 环境要求

- [Docker](https://www.docker.com/) & [Docker Compose](https://docs.docker.com/compose/)
- [Git](https://git-scm.com/)

### 📦 安装与启动

```bash
git clone <本仓库地址>
cd <仓库目录>
cp .env.example .env   # 编辑 .env：务必更换 JWT_SECRET / SYSTEM_AES_KEY / DB 与 Redis 密码
docker compose up -d --build   # 首次构建约 10-20 分钟（Go 与前端均在容器内构建），后续有缓存
```

> 发布版镜像为**多阶段构建**：前端静态资源在容器内完成编译，使用者电脑**无需安装 Node.js / Go 等开发环境**，只依赖 Docker。
> 国内网络可加速构建：`docker compose build --build-arg NPM_REGISTRY=https://registry.npmmirror.com frontend`。

Windows 环境可在仓库根目录直接执行 `start_zhishu.bat` 一键启动并跟踪日志，`stop_zhishu.bat` 停止服务。

启动成功后访问 **http://localhost** 即可使用：首次进入按初始化向导配置模型与知识库，或在 `.env` 中设置 `ZHISHU_BOOTSTRAP_SYSTEM_ADMIN_EMAIL=<已注册邮箱>` 后重启，该账号将自动成为系统管理员。

> **模型配置（可选外部依赖）**：系统支持在「设置 → 模型管理」中配置云端模型 API（对话/Embedding/Rerank），开箱即用；也可对接本地 llama.cpp / Ollama 服务（默认地址 `http://host.docker.internal:8081`、`:8082` 与 `:11434`）。**不配置任何模型仅影响对话/解析功能，系统本身可正常启动与浏览**。
> 如需使用本地 Ollama 模型，请先运行 `ollama serve > /dev/null 2>&1 &`

> 默认端口：Web UI `80`（`FRONTEND_PORT`，Nginx 反代后端 API）、后端 API `8080`（`APP_PORT`）。如需修改，编辑 `.env` 后重新 `docker compose up -d` 生效。

### 🔧 可选服务（Docker Compose Profile）

按需添加 `--profile` 启动额外组件，多个 profile 可叠加使用：

| Profile | 说明 | 启动命令 |
|---------|------|----------|
| _(默认)_ | 核心服务 | `docker compose up -d --build` |
| `full` | 全部功能 | `docker compose --profile full up -d --build` |
| `neo4j` | 知识图谱 (Neo4j) | `docker compose --profile neo4j up -d --build` |
| `minio` | 对象存储 (MinIO) | `docker compose --profile minio up -d --build` |
| `langfuse` | 链路追踪 (Langfuse) | `docker compose --profile langfuse up -d --build` |

停止服务：`docker compose down`

### 🌐 服务地址

| 服务 | 地址 | 端口变量 |
|------|------|----------|
| Web UI | `http://localhost` | `FRONTEND_PORT`（默认 80） |
| 后端 API | `http://localhost:8080` | `APP_PORT`（默认 8080） |
| 链路追踪 (Langfuse) | `http://localhost:3000` | 需启用 `langfuse` profile |

## 📘 文档

**产品文档**：[`website-docs/`](./website-docs/README.md) —— 按「入门 → 架构 → 功能 → API → 客户端 → 开发」六个板块组织的完整文档。本目录同时是一个 VitePress 站点，`cd website-docs && npm install && npm run dev` 即可本地预览，也可用目录内的 `Dockerfile` 独立部署。

常见问题排查：[常见问题排查](./docs/QA.md)

详细接口说明请参考：[API 文档](./docs/api/README.md)

## 🧭 开发指南

如果你需要频繁修改代码，可以使用快速开发模式（无需每次重新构建 Docker 镜像）：

```bash
# 启动基础设施
make dev-start

# 启动后端（新终端）
make dev-app

# 启动前端（新终端）
make dev-frontend
```

**详细文档：** [开发环境快速入门](./docs/开发指南.md)

## 🤝 贡献指南

欢迎通过 [Issue](../../issues) 反馈问题或提交 Pull Request。

**流程：** Fork → 新建分支 → 提交更改 → 创建 PR

## 🔒 安全声明

**重要提示：** 生产环境部署时，我们强烈建议：

- 将本系统部署在内网/私有网络环境中，而非公网环境
- 避免将服务直接暴露在公网上，以防止重要信息泄露风险
- 为部署环境配置适当的防火墙规则和访问控制
- 定期更新到最新版本以获取安全补丁和改进

## 📄 许可证

本项目基于 [MIT](./LICENSE) 协议发布。
你可以自由使用、修改和分发本项目代码，但需保留原始版权声明。
