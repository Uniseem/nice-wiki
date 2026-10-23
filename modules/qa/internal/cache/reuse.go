package cache

import (
	"context"
	"encoding/json"
	"time"

	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

// ReuseCandidate 是一条可以交给决策模型（Jev）判断能否复用的缓存问答：英文问题（Jev 只读英文）和提问时所在的文档。
type ReuseCandidate struct {
	CacheID         string
	EnglishQuestion string
	SourceDocument  string
}

// ListReuseCandidates 列出本范围内、访客语言相同、仍有效且有英文问题的缓存条目，精选和较新的在前，最多 limit 条。
func ListReuseCandidates(ctx context.Context, op OperationContext, scopeHash, language string, limit int) ([]ReuseCandidate, error) {
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return nil, err
	}
	defer tx.Close()
	policy, err := qaruntime.ReadPolicy(tx)
	if err != nil || !policy.CacheEnabled {
		return nil, err
	}
	rows, err := tx.Tx.QueryContext(tx.Context, `SELECT cache_id,english_question,source_document,cache_json FROM answer_cache WHERE site_id=? AND data_epoch=? AND scope_hash=? AND state='active' AND (curated=1 OR expires_ns>?) AND english_question<>'' ORDER BY curated DESC,expires_ns DESC LIMIT ?`, op.Actor.SiteID, op.DataEpoch, scopeHash, time.Now().UnixNano(), limit*2)
	if err != nil {
		return nil, qaruntime.StorageError(err)
	}
	defer rows.Close()
	candidates := []ReuseCandidate{}
	for rows.Next() && len(candidates) < limit {
		var candidate ReuseCandidate
		var raw []byte
		if err := rows.Scan(&candidate.CacheID, &candidate.EnglishQuestion, &candidate.SourceDocument, &raw); err != nil {
			return nil, qaruntime.StorageError(err)
		}
		var entry struct {
			Language         string `json:"language"`
			QuestionComplete bool   `json:"questionComplete"`
		}
		if json.Unmarshal(raw, &entry) == nil && entry.Language == language && entry.QuestionComplete {
			candidates = append(candidates, candidate)
		}
	}
	return candidates, qaruntime.StorageError(rows.Err())
}

// LoadReusable 读出选中的缓存条目，并按当前发布状态和回答记忆版本重新核验；不再有效时返回 nil。
func LoadReusable(ctx context.Context, op OperationContext, cacheID string) (*gen.CacheEntry, error) {
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return nil, err
	}
	record, err := ReadCache(tx, op, cacheID)
	tx.Close()
	if err != nil {
		return nil, err
	}
	validated, err := ValidateCachedAnswer(ctx, op, ValidateCachedAnswerInput{Entry: record.Entry, ContextHash: tx.Runtime.AnswerMemoryHash})
	if err != nil || !validated.Valid {
		return nil, err
	}
	return validated.Entry, nil
}

// SaveReuseInfo 记下缓存条目的英文问题和提问时所在的文档，之后的提问由 Jev 拿它们判断能否复用这条答案。
func SaveReuseInfo(ctx context.Context, op OperationContext, cacheID, question, sourceDocument string) error {
	tx, err := qaruntime.BeginWrite(ctx, op)
	if err != nil {
		return err
	}
	defer tx.Close()
	if _, err := tx.Tx.ExecContext(tx.Context, `UPDATE answer_cache SET english_question=?,source_document=? WHERE cache_id=? AND site_id=? AND data_epoch=?`, question, sourceDocument, cacheID, op.Actor.SiteID, op.DataEpoch); err != nil {
		return qaruntime.StorageError(err)
	}
	return tx.Commit()
}
