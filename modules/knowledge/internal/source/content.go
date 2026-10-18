// Package source 只读外域权威，保存绑定构建来源的私有快照。
package source

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type OperationContext = platform.OperationContext
type FetchPublishedSourceInput struct {
	BuildID string `json:"buildId"`
	LeaseID string `json:"leaseId"`
}
type FetchPublishedSourceResult struct {
	Document    gen.PublishedRevisionRef `json:"document"`
	Source      string                   `json:"source"`
	ContentHash string                   `json:"contentHash"`
}

// FetchPublishedSource 返回16MiB内canonical DocumentBody JSON；不把扁平文本伪装成源正文。
func FetchPublishedSource(ctx context.Context, op OperationContext, input FetchPublishedSourceInput) (FetchPublishedSourceResult, error) {
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return FetchPublishedSourceResult{}, err
	}
	if runtime.Content == nil || runtime.Validators["ContentReadPublishedRevisionResult"] == nil {
		return FetchPublishedSourceResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "正文权威未装配")
	}
	_, read, record, closeRead, err := indexer.BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return FetchPublishedSourceResult{}, err
	}
	_ = read.Rollback()
	closeRead()
	raw, err := runtime.Content.ReadPublishedRevision(platform.WithOperationContext(ctx, op), record.Build.Document.DocumentId, url.Values{"publishedRevisionId": {record.Build.Document.PublishedRevisionId}})
	if err != nil {
		return FetchPublishedSourceResult{}, err
	}
	if err = runtime.Validators["ContentReadPublishedRevisionResult"](raw); err != nil {
		return FetchPublishedSourceResult{}, err
	}
	var result struct {
		Reference gen.PublishedRevisionRef `json:"reference"`
		Body      json.RawMessage          `json:"body"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Reference != record.Build.Document {
		return FetchPublishedSourceResult{}, platform.NewError(platform.CodeIntegrityError, false, "正文权威引用与构建不一致")
	}
	canonical, err := platform.CanonicalJSON(result.Body)
	if err != nil {
		return FetchPublishedSourceResult{}, err
	}
	if len(canonical) > 16*1024*1024 || platform.HashBytes(canonical) != record.Build.Document.ContentHash {
		return FetchPublishedSourceResult{}, platform.NewError(platform.CodeIntegrityError, false, "正文规范字节长度或摘要不匹配")
	}
	work, tx, current, cancel, err := indexer.BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return FetchPublishedSourceResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	if current.Build.Document != record.Build.Document {
		return FetchPublishedSourceResult{}, platform.NewError(platform.CodeStaleResult, false, "构建来源已变化")
	}
	documentRaw, _ := platform.CanonicalJSON(result.Reference)
	var saved []byte
	err = tx.QueryRowContext(work, `SELECT body_json FROM build_sources WHERE build_id=?`, input.BuildID).Scan(&saved)
	if err == nil {
		if string(saved) != string(canonical) {
			return FetchPublishedSourceResult{}, platform.NewError(platform.CodeIntegrityError, false, "同源构建快照内容冲突")
		}
	} else if err == sql.ErrNoRows {
		_, err = tx.ExecContext(work, `INSERT INTO build_sources(build_id,document_json,content_hash,body_json,created_at) VALUES(?,?,?,?,?)`, input.BuildID, documentRaw, result.Reference.ContentHash, canonical, platform.Now())
		if err != nil {
			return FetchPublishedSourceResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "私有源快照保存失败")
		}
	} else {
		return FetchPublishedSourceResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "私有源快照读取失败")
	}
	if err = tx.Commit(); err != nil {
		return FetchPublishedSourceResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "源快照提交失败")
	}
	return FetchPublishedSourceResult{Document: result.Reference, Source: string(canonical), ContentHash: result.Reference.ContentHash}, nil
}
