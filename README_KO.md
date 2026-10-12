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
| <a href="./README.md"><b>English</b></a> | <a href="./README_CN.md"><b>简体中文</b></a> | <a href="./README_JA.md"><b>日本語</b></a> | <b>한국어</b> |
</p>

# 지식 데이터베이스 관리 시스템 (智枢文渊 · ZhiShu)

## 📌 개요

**智枢数据库知识管理系统**(智枢文渊, ZhiShu)는 오픈소스 대규모 언어 모델(LLM) 지식 관리 프레임워크를 기반으로 개발된 엔터프라이즈 지식 관리 시스템입니다.

**RAG 지식 Q&A**, **에이전트 추론**, **지식 베이스 관리**(다양한 형식의 문서 가져오기, 청크 편집), **멀티 워크스페이스 권한 관리**, **프라이빗 배포** 기능을 제공하여 흩어진 문서를 검색 가능하고 추론 가능한 지식 자산으로 전환합니다.

주요 기능: PDF / Word / Excel / 이미지 등 10여 가지 문서 형식 지원, Feishu·Notion·语雀 등 데이터 소스 동기화, 16개 이상의 LLM 공급자 통합, 4단계 역할 매트릭스 기반 워크스페이스 RBAC, Docker / Kubernetes 프라이빗 배포.

## 🚀 빠른 시작

### 환경 요구 사항

- [Docker](https://www.docker.com/) & [Docker Compose](https://docs.docker.com/compose/)
- [Git](https://git-scm.com/)

### 설치 및 실행

```bash
git clone <저장소 URL>
cd <저장소 디렉터리>
cp .env.example .env   # .env 편집: JWT_SECRET / SYSTEM_AES_KEY / DB·Redis 비밀번호를 반드시 변경
docker compose up -d --build   # 첫 빌드는 약 10~20분 소요 (Go와 프론트엔드 모두 컨테이너 내부에서 빌드)
```

> 멀티스테이지 빌드 채택: 프론트엔드 자산은 컨테이너 내부에서 컴파일되므로 호스트에 Node.js / Go 등의 개발 환경이 필요 없습니다. Docker만 있으면 됩니다.

Windows 환경에서는 저장소 루트의 `start_zhishu.bat`로 원클릭 실행, `stop_zhishu.bat`으로 중지할 수 있습니다.

실행 후 **http://localhost** 에 접속하여 초기 설정 마법사에 따라 모델과 지식 베이스를 구성하세요. `.env`에 `ZHISHU_BOOTSTRAP_SYSTEM_ADMIN_EMAIL=<등록된 이메일>`을 설정하고 재시작하면 해당 계정이 자동으로 시스템 관리자가 됩니다.

> **모델 설정(선택적 외부 의존성)**: 「설정 → 모델 관리」에서 클라우드 모델 API(대화 / Embedding / Rerank)를 설정하면 바로 사용할 수 있습니다. 로컬 llama.cpp / Ollama 서비스(기본값 `http://host.docker.internal:8081`, `:8082`, `:11434`)에도 연결할 수 있습니다. 모델을 설정하지 않으면 대화·분석 기능만 사용할 수 없고 시스템 자체는 정상적으로 시작됩니다.

> 기본 포트: Web UI `80`(`FRONTEND_PORT`), 백엔드 API `8080`(`APP_PORT`). 변경하려면 `.env`를 편집한 후 `docker compose up -d`를 다시 실행하세요.

## 📘 문서

- 제품 문서: [website-docs/](./website-docs/README.md)
- 자주 묻는 질문: [docs/QA.md](./docs/QA.md)
- API 문서: [docs/api/README.md](./docs/api/README.md)

자세한 내용은 [간체 중국어 README](./README_CN.md) 또는 [영어 README](./README.md)를 참조하세요.

## 🔒 보안 안내

프로덕션 환경에서는 내부 네트워크 / 프라이빗 네트워크 배치를 강력히 권장합니다. 공용 인터넷에 직접 노출하지 말고, 적절한 방화벽 규칙과 접근 제어를 설정하며, 최신 버전으로 정기적으로 업데이트하세요.

## 📄 라이선스

이 프로젝트는 [MIT](./LICENSE) 라이선스로 공개되어 있습니다. 저작권 표시를 유지하는 한 자유롭게 사용·수정·배포할 수 있습니다.
