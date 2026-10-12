package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/models/embedding"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ============================================================================
// v2.23 P1-1 修复单测：KB 开着 vector/keyword 索引但嵌入模型缺失
// （embedding_model_id 为空 → GetEmbeddingModel 报 "model ID cannot be
// empty"）时，processChunks 必须把文档显式置为 failed，而不是只打日志
// 留在 processing 永久卡死。
//
// processChunks 前段依赖极少：isKnowledgeAborted（repo.GetKnowledgeByID）
// → GetEmbeddingModel（modelService）→（修复路径）repo.UpdateKnowledge。
// 用最小桩直接驱动真实 processChunks，锁定行为。
// ============================================================================

// p11KnowledgeRepoStub 提供 GetKnowledgeByID（未取消）与 UpdateKnowledge 捕获。
type p11KnowledgeRepoStub struct {
	interfaces.KnowledgeRepository
	knowledge *types.Knowledge
	updates   []*types.Knowledge
}

func (r *p11KnowledgeRepoStub) GetKnowledgeByID(
	_ context.Context, _ uint64, id string,
) (*types.Knowledge, error) {
	if r.knowledge != nil && r.knowledge.ID == id {
		cp := *r.knowledge
		return &cp, nil
	}
	return nil, nil
}

func (r *p11KnowledgeRepoStub) UpdateKnowledge(_ context.Context, k *types.Knowledge) error {
	cp := *k
	r.updates = append(r.updates, &cp)
	return nil
}

// p11ModelSvcStub 使 GetEmbeddingModel 按嵌入模型 ID 是否为空决定成败，
// 与真实 modelService 的行为一致（空 ID → "model ID cannot be empty"）。
type p11ModelSvcStub struct {
	interfaces.ModelService
}

func (s *p11ModelSvcStub) GetEmbeddingModel(_ context.Context, modelID string) (embedding.Embedder, error) {
	if modelID == "" {
		return nil, errors.New("model ID cannot be empty")
	}
	return nil, errors.New("stub: no real embedder")
}

// p11TenantInfoCtx 注入 TenantInfo（processChunks 后段会取用；修复路径在
// 取用之前 return，但保持上下文形态与生产一致以防前移）。
func p11TenantInfoCtx() context.Context {
	return context.WithValue(context.Background(), types.TenantInfoContextKey, &types.Tenant{ID: 10000})
}

func TestProcessChunks_EmbeddingModelMissingMarksFailed(t *testing.T) {
	kb := &types.KnowledgeBase{
		ID:               "kb-p11",
		EmbeddingModelID: "", // 缺失：开着索引却没绑嵌入模型
	}
	kb.IndexingStrategy = types.IndexingStrategy{VectorEnabled: true, KeywordEnabled: true}
	if !kb.NeedsEmbeddingModel() {
		t.Fatal("precondition: KB should need an embedding model")
	}

	knowledge := &types.Knowledge{
		ID:              "doc-p11",
		TenantID:        10000,
		KnowledgeBaseID: kb.ID,
		ParseStatus:     types.ParseStatusProcessing,
	}
	repo := &p11KnowledgeRepoStub{knowledge: knowledge}

	svc := &knowledgeService{
		repo:         repo,
		modelService: &p11ModelSvcStub{},
	}

	// 触发：真实 processChunks，走到嵌入模型解析失败分支即 return。
	svc.processChunks(p11TenantInfoCtx(), kb, knowledge, nil)

	if len(repo.updates) == 0 {
		t.Fatal("expected UpdateKnowledge to persist the failed status, got none (doc stuck in processing)")
	}
	last := repo.updates[len(repo.updates)-1]
	if last.ParseStatus != types.ParseStatusFailed {
		t.Fatalf("parse_status = %q, want %q", last.ParseStatus, types.ParseStatusFailed)
	}
	if last.ErrorMessage == "" {
		t.Fatal("error_message must explain the failure (embedding model unavailable)")
	}
}

func TestProcessChunks_EmbeddingDisabledKBNoFailedWrite(t *testing.T) {
	// 关闭 vector/keyword 的 KB 合法地不需要嵌入模型：
	// 嵌入模型解析必须被跳过，不得触发 failed 回写（正常链路零改动）。
	kb := &types.KnowledgeBase{ID: "kb-noemb", EmbeddingModelID: ""}
	kb.IndexingStrategy = types.IndexingStrategy{VectorEnabled: false, KeywordEnabled: false}
	if kb.NeedsEmbeddingModel() {
		t.Fatal("precondition: KB should not need an embedding model")
	}

	knowledge := &types.Knowledge{
		ID:              "doc-noemb",
		TenantID:        10000,
		KnowledgeBaseID: kb.ID,
		ParseStatus:     types.ParseStatusProcessing,
	}
	repo := &p11KnowledgeRepoStub{knowledge: knowledge}

	// 空租户上下文形态下禁用-KB 分支不会触碰后段重依赖（修复路径断言仅
	// 针对 failed 回写；后段若因桩边界 panic，recover 记录即可——它不属
	// 于本修复的目标路径）。
	svc := &knowledgeService{
		repo:         repo,
		modelService: &p11ModelSvcStub{},
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("processChunks continued past the embedding stage (stub boundary): %v", r)
			}
		}()
		svc.processChunks(p11TenantInfoCtx(), kb, knowledge, nil)
	}()

	for _, u := range repo.updates {
		if u.ParseStatus == types.ParseStatusFailed {
			t.Fatalf("disabled-KB path must not write failed status, got error_message=%q", u.ErrorMessage)
		}
	}
}
