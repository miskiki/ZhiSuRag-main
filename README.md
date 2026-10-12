<p align="center">
  <picture>
    <img src="./docs/images/zhishu-logo.png" alt="ZhiShu Logo" height="110"/>
  </picture>
</p>

<p align="center">
    <a href="./LICENSE">
        <img src="https://img.shields.io/badge/License-MIT-ffffff?labelColor=d4eaf7&color=2e6cc4" alt="License">
    </a>
    <a href="./CHANGELOG.md">
        <img alt="version" src="https://img.shields.io/badge/version-0.7.2-2e6cc4?labelColor=d4eaf7">
    </a>
</p>

<p align="center">
| <b>English</b> | <a href="./README_CN.md"><b>简体中文</b></a> |
</p>

# ZhiShu Knowledge Management System (智枢文渊)

## 📌 Overview

**ZhiShu** (智枢文渊), fully known as the "ZhiShu Knowledge Management System" (智枢数据库知识管理系统), is an enterprise knowledge management system built on top of an open-source LLM-powered knowledge management framework.

With the goal of making every document in your organization answerable, the system builds a complete document processing and consumption pipeline around knowledge bases: it ingests 10+ document formats (PDF, Word, Excel, images, Markdown, and more), parses and vectorizes them into knowledge bases, and lets users get **cited answers through natural-language Q&A** — or delegate complex multi-step tasks to an **agent** that autonomously orchestrates knowledge retrieval, MCP tools, and web search.

For knowledge governance, the system offers tree-structured folder organization, batch import and tagging, online chunk editing, and multi-source data sync (Feishu, Notion, Yuque, RSS, etc.), combined with enterprise-grade multi-workspace access control (4-tier role matrix, resource ownership, audit logs, and platform-level account management) so different teams can securely curate knowledge in their own spaces. Q&A can also be published into existing work channels via website embed widgets and IM integrations. It works with major LLM providers including OpenAI, DeepSeek, Qwen, Zhipu, and Ollama, and the LLM, vector database, and storage backends are all swappable — supporting local and private-cloud deployment with full data sovereignty.

## 🚀 Getting Started

### 🛠 Prerequisites

- [Docker](https://www.docker.com/) & [Docker Compose](https://docs.docker.com/compose/)
- [Git](https://git-scm.com/)

### 📦 Installation & Launch

```bash
git clone <this-repository-url>
cd <repo-directory>
cp .env.example .env   # Edit .env: make sure to change JWT_SECRET / SYSTEM_AES_KEY / DB & Redis passwords
docker compose up -d --build   # First build takes ~10-20 min (Go & frontend both build inside Docker), cached afterwards
```

> Published images use **multi-stage builds**: frontend assets are compiled inside Docker, so your machine needs **no Node.js / Go toolchain** — only Docker.
> To speed up builds behind CN networks: `docker compose build --build-arg NPM_REGISTRY=https://registry.npmmirror.com frontend`.

On Windows you can run `start_zhishu.bat` from the repository root to start everything and follow the logs, and `stop_zhishu.bat` to stop.

Once started, visit **http://localhost** to get started: follow the initialization wizard to configure models and your first knowledge base, or set `ZHISHU_BOOTSTRAP_SYSTEM_ADMIN_EMAIL=<registered-email>` in `.env` and restart to auto-promote that account to system administrator.

> **Model configuration (optional external dependency)**: configure cloud model APIs (chat / embedding / rerank) under "Settings → Models" — works out of the box; or point to local llama.cpp / Ollama services (defaults `http://host.docker.internal:8081`, `:8082`, `:11434`). **Without any model, chat/parsing won't run but the system itself starts and browses normally.**
> To use a local Ollama model, run `ollama serve > /dev/null 2>&1 &` first.

> Default ports: Web UI `80` (`FRONTEND_PORT`, Nginx proxying the backend API), backend API `8080` (`APP_PORT`). To change them, edit `.env` and re-run `docker compose up -d`.

### 🔧 Optional Services (Docker Compose Profiles)

Add `--profile` flags to enable additional components. Multiple profiles can be combined:

| Profile | Description | Command |
|---------|-------------|---------|
| _(default)_ | Core services | `docker compose up -d --build` |
| `full` | All features | `docker compose --profile full up -d --build` |
| `neo4j` | Knowledge Graph (Neo4j) | `docker compose --profile neo4j up -d --build` |
| `minio` | Object Storage (MinIO) | `docker compose --profile minio up -d --build` |
| `langfuse` | Tracing (Langfuse) | `docker compose --profile langfuse up -d --build` |

Stop services: `docker compose down`

### 🌐 Service URLs

| Service | URL | Port Variable |
|---------|-----|---------------|
| Web UI | `http://localhost` | `FRONTEND_PORT` (default 80) |
| Backend API | `http://localhost:8080` | `APP_PORT` (default 8080) |
| Langfuse Tracing | `http://localhost:3000` | requires the `langfuse` profile |

## 📘 Documentation

**Product documentation**: [`website-docs/`](./website-docs/README.md) — the complete documentation set organized as Getting Started → Architecture → Features → API → Clients → Development. The directory is also a VitePress site: run `cd website-docs && npm install && npm run dev` to preview locally, or deploy it standalone with the `Dockerfile` inside.

Troubleshooting FAQ: [Troubleshooting FAQ](./docs/QA.md)

Detailed API documentation is available at: [API Docs](./docs/api/README.md)

## 🧭 Developer Guide

If you need to frequently modify code, use fast development mode (no Docker image rebuild needed each time):

```bash
# Start infrastructure
make dev-start

# Start backend (new terminal)
make dev-app

# Start frontend (new terminal)
make dev-frontend
```

**Detailed Documentation:** [Development Environment Quick Start](./docs/开发指南.md)

## 🤝 Contributing

Welcome to submit [Issues](../../issues) or Pull Requests.

**Process:** Fork → Create branch → Commit changes → Open PR

## 🔒 Security Notice

**Important:** For production deployments, we strongly recommend:

- Deploy the services in internal/private network environments rather than public internet
- Avoid exposing the service directly to public networks to prevent potential information leakage
- Configure proper firewall rules and access controls for your deployment environment
- Regularly update to the latest version for security patches and improvements

## 📄 License

This project is licensed under the [MIT License](./LICENSE).
You are free to use, modify, and distribute the code with proper attribution.
