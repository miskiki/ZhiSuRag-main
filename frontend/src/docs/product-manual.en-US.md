# ZhiShu Product Manual

> ZhiShu Knowledge Management System — an enterprise knowledge management and intelligent Q&A platform built on LLM and RAG (Retrieval-Augmented Generation) technology.

---

## 1. Overview

ZhiShu turns scattered documents, web pages and Q&A pairs into **searchable, conversational, publishable** knowledge assets:

- **Knowledge processing**: upload documents for automatic parsing, chunking and vectorization; supports FAQ, Wiki and data-source sync;
- **Intelligent chat**: streaming answers grounded in your knowledge bases with **cited sources**, plus Agent multi-step reasoning and tool calling;
- **Agent publishing**: package knowledge bases and prompts into agents served via embedded widgets, IM channels and APIs;
- **Collaboration**: multi-workspace, multi-role collaboration with member management, knowledge sharing and full audit trails.

---

## 2. Quick Start

1. **Sign in** with the account assigned by your administrator (first-time users are encouraged to complete the onboarding tour).
2. **Make sure models are connected**: before creating a knowledge base, a system administrator must connect a chat model (KnowledgeQA) and an Embedding model in **Settings → Model Management**. If you are not a system administrator and models are not connected yet, contact your system administrator.
3. **Create a knowledge base**: sidebar → Knowledge Bases → New Knowledge Base. Choose a type (Document / FAQ), set a name and bind models.
4. **Upload documents**: in the knowledge base detail view, click Upload. Four channels are supported: local files/folders, URL capture, manual entry and data-source sync.
5. **Start chatting**: sidebar → New Chat, mention a knowledge base with `@` and ask a question — answers come with cited sources.

---

## 3. Core Features

### 3.1 Knowledge Base Management

| Feature | Description |
| --- | --- |
| Two KB types | Document bases (PDF/Word/Markdown etc.) and FAQ bases; the type is fixed after creation |
| Ingestion | Local files/folders, URL capture, manual entry, data-source sync (Feishu, Notion, Yuque, etc.) |
| Parsing & chunking | Visualized parsing timeline, online chunk editing, version history and rollback, chunk-level suggested questions |
| Tags & folders | Batch tagging, folder trees, multi-condition filtering |
| Wiki generation | Queue-driven batch Wiki generation with TOC tree, version diff and rollback |

### 3.2 Intelligent Chat

- **RAG Q&A**: streaming answers with pipeline progress (retrieve → rerank → generate);
- **Source citations**: every answer marks its knowledge sources; click to inspect the original text — knowledge base and web sources are distinguished;
- **Agent reasoning**: visualized ReAct multi-step loop; sensitive tool calls require human approval, MCP OAuth can be authorized in-session;
- **Session management**: groups, pinning, search, batch operations, AI-generated titles;
- **Memory**: two layers — personal memory (follows you across workspaces) and workspace memory;
- **Temporary attachments**: upload images/documents within a chat; parsed asynchronously and used in the conversation.

### 3.3 Agents & Publishing

- **Agent modeling**: one-stop editor for models, persona, prompt templates, openers, KB selection strategy, tools & MCP, approval policy;
- **Embed channels**: generate iframe snippets to mount on external websites — visitors chat without signing in;
- **IM channels**: bind WeCom, DingTalk, Feishu and other IM platforms for in-chat Q&A;
- **API Keys**: issue keys scoped by capability groups (knowledge / automation / collaboration) with an SSE debug console.

### 3.4 Collaboration

- **Shared workspaces (organizations)**: create organizations, invite members via code/email, optional join approval;
- **Knowledge sharing**: share knowledge bases and agents with organizations; target workspaces get a "Shared with me" view; sharing never amplifies local permissions;
- **Audit logs**: workspace admins can inspect `rbac.*` action audits filtered by action/result/actor.

### 3.5 Personal Preferences

The settings center covers language, theme, font size and password change.

---

## 4. Roles & Permissions

The system uses a two-layer model: workspace roles plus a platform-wide system administrator.

### 4.1 Roles

| Role | Scope |
| --- | --- |
| System administrator (SystemAdmin) | Platform-level: model access, engine & infrastructure configuration, task queues, platform audit |
| Owner (workspace owner) | Full workspace control: members, API key issuance, knowledge sharing |
| Admin (workspace admin) | Day-to-day management: members, sharing, message management, prompt templates; read-only on configuration |
| Contributor | Upload documents, edit chunks, maintain FAQ and Wiki |
| Viewer | Search, chat and read Wiki |

### 4.2 Permission Matrix

| Capability | Viewer | Contributor | Admin / Owner | SystemAdmin |
| --- | :---: | :---: | :---: | :---: |
| Search / chat / read Wiki | ✅ | ✅ | ✅ | ✅ |
| Upload documents / edit chunks / FAQ | — | ✅ | ✅ | ✅ |
| Members, invitations, sharing | — | — | ✅ | ✅ |
| API key issuance | — | — | Owner | ✅ |
| Message management / memory / prompt templates | — | — | Admin+ | ✅ |
| Model management (add/modify models) | Read-only | Read-only | Read-only | ✅ Write |
| Vector stores / storage / parser engines / sandbox / web search / MCP | Read-only | Read-only | Read-only | ✅ Write |
| Platform parameters (retrieval config etc.) | Read-only | Read-only | Read-only | ✅ Write |
| KB-level selection (choose model / vector store / skills) | Read-only | — | ✅ (unchanged) | ✅ |
| Platform ops (queues / platform keys / platform audit) | — | — | — | ✅ |

> **Permission consolidation note**: since v2.9, write access to models, vector stores, storage backends, parser engines, the skill sandbox, web search, MCP and retrieval configuration is reserved for system administrators; workspace roles keep read-only access and KB-level selection. If you cannot see the "Add model" button or get a 403 when saving settings, the configuration is managed by your system administrator — please contact them.

---

## 5. FAQ

**Q: Creating a knowledge base says models are required?**
A document KB needs at least one chat model (KnowledgeQA) and one Embedding model; an agent only needs the chat model. Model access is managed by system administrators — ask them to connect models in Settings → Model Management.

**Q: A document is stuck in "parsing"?**
Parsing runs asynchronously through the task queue; check the parsing timeline in the KB detail view. If it stalls, ask a system administrator to inspect and retry the task under Platform → Task Queues.

**Q: How do I serve external website visitors?**
A workspace admin can create an agent and enable an embed channel, then mount the generated iframe snippet on the external site — no sign-in required for visitors.

**Q: How do I replay the onboarding tour?**
Click the **help button** next to your name at the top of the menu.

**Q: More deployment and developer docs?**
See the project repository [docs directory](https://github.com/Tencent/WeKnora/tree/main/docs).
