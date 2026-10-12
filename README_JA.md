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
        <img alt="version" src="https://img.shields.io/badge/version-0.7.2-2e6cc4?labelColor=d4eaf7">
    </a>
</p>

<p align="center">
| <a href="./README.md"><b>English</b></a> | <a href="./README_CN.md"><b>简体中文</b></a> | <b>日本語</b> | <a href="./README_KO.md"><b>한국어</b></a> |
</p>

# 智枢データベースナレッジ管理システム（智枢文渊 · ZhiShu）

## 📌 概要

**智枢データベースナレッジ管理システム**（智枢文渊、ZhiShu）は、オープンソースの大規模言語モデル（LLM）ナレッジ管理フレームワークを基盤として開発された、エンタープライズ向けナレッジ管理システムです。

**RAG ナレッジ Q&A**、**エージェント推論**、**ナレッジベース管理**（多形式ドキュメント取り込み・チャンク編集）、**マルチワークスペース権限管理**、**プライベートデプロイ**などの機能を提供し、分散したドキュメントを検索可能で推論可能なナレッジ資産へと変換します。

主な機能：PDF / Word / Excel / 画像など 10 種類以上のドキュメント形式に対応、飛書・Notion・語雀などのデータソース同期、16 以上の LLM プロバイダー統合、4 階層ロールマトリクスによるワークスペース RBAC、Docker / Kubernetes によるプライベートデプロイ。

## 🚀 クイックスタート

### 環境要件

- [Docker](https://www.docker.com/) & [Docker Compose](https://docs.docker.com/compose/)
- [Git](https://git-scm.com/)

### インストールと起動

```bash
git clone <リポジトリURL>
cd <リポジトリディレクトリ>
cp .env.example .env   # .env を編集：JWT_SECRET / SYSTEM_AES_KEY / DB・Redis パスワードを必ず変更
docker compose up -d --build   # 初回ビルドは約 10〜20 分（Go とフロントエンドはコンテナ内でビルド）
```

> マルチステージビルド採用：フロントエンド資産はコンテナ内でコンパイルされるため、ホストに Node.js / Go 等の開発環境は不要です。Docker のみで動作します。

Windows 環境ではリポジトリ直下の `start_zhishu.bat` でワンクリック起動、`stop_zhishu.bat` で停止できます。

起動後 **http://localhost** にアクセスし、初期設定ウィザードに従ってモデルとナレッジベースを構成してください。`.env` に `ZHISHU_BOOTSTRAP_SYSTEM_ADMIN_EMAIL=<登録済みメールアドレス>` を設定して再起動すると、そのアカウントが自動的にシステム管理者になります。

> **モデル設定（任意の外部依存）**：「設定 → モデル管理」でクラウドモデル API（対話 / Embedding / Rerank）を設定すればすぐに使えます。ローカル llama.cpp / Ollama サービス（既定 `http://host.docker.internal:8081`、`:8082`、`:11434`）にも接続可能です。モデル未設定でも対話・解析機能が使えないだけで、システム自体は正常に起動します。

> 既定ポート：Web UI `80`（`FRONTEND_PORT`）、バックエンド API `8080`（`APP_PORT`）。変更する場合は `.env` を編集して `docker compose up -d` を再実行してください。

## 📘 ドキュメント

- 製品ドキュメント：[website-docs/](./website-docs/README.md)
- よくある質問：[docs/QA.md](./docs/QA.md)
- API ドキュメント：[docs/api/README.md](./docs/api/README.md)

詳細は[簡体中文版 README](./README_CN.md) または [英語版 README](./README.md) をご参照ください。

## 🔒 セキュリティに関する注意

本番環境では、内部ネットワーク / プライベートネットワークへの配置を強く推奨します。公衆インターネットに直接公開せず、適切なファイアウォール規則とアクセス制御を設定し、最新バージョンへの更新を定期的に行ってください。

## 📄 ライセンス

本プロジェクトは [MIT](./LICENSE) ライセンスで公開しています。著作権表示を保持する限り、自由に利用・改変・配布できます。
