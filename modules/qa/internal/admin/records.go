package admin

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type RecordPage struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}
type ListTurnRecordsInput struct {
	Page     RecordPage `json:"page"`
	Filter   string     `json:"filter"`
	Language string     `json:"language,omitempty"`
}
type TurnRecordItem struct {
	TurnID          string `json:"turnId"`
	QuestionPreview string `json:"questionPreview"`
	AnswerKind      string `json:"answerKind"`
	Feedback        string `json:"feedback"`
	CreatedAt       string `json:"createdAt"`
}
type ListTurnRecordsResult struct {
	Items    []TurnRecordItem `json:"items"`
	PageInfo gen.PageInfo     `json:"pageInfo"`
}
type ReadTurnRecordInput struct {
	TurnID string `json:"turnId"`
}
type CitationStatus struct {
	Block gen.BlockRef `json:"block"`
	Valid bool         `json:"valid"`
}
type ReadTurnRecordResult struct {
	Sources          []gen.Citation   `json:"sources"`
	TurnID           string           `json:"turnId"`
	Question         string           `json:"question"`
	Answer           string           `json:"answer"`
	Citations        []gen.BlockRef   `json:"citations"`
	AnswerKind       string           `json:"answerKind"`
	CitationStatuses []CitationStatus `json:"citationStatuses"`
}

func ListTurnRecords(ctx context.Context, op OperationContext, input ListTurnRecordsInput) (ListTurnRecordsResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.manage"); err != nil {
		return ListTurnRecordsResult{}, err
	}
	if input.Page.Limit == 0 {
		input.Page.Limit = 20
	}
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return ListTurnRecordsResult{}, err
	}
	defer tx.Close()
	binding, _ := platform.CanonicalJSON(struct {
		Actor                   any
		Epoch, Filter, Language string
		Limit                   int
	}{qaruntime.CursorActor(op), op.DataEpoch, input.Filter, input.Language, input.Page.Limit})
	// 最新的在前：按创建时间、轮次编号倒序做键集分页，游标记录上一页最后一条的位置。
	scope := "qa.records.v2:" + platform.HashBytes(binding)
	var after struct{ CreatedAt, TurnID string }
	if input.Page.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(tx.Runtime.SigningKey, scope, input.Page.Cursor)
		if err != nil {
			return ListTurnRecordsResult{}, err
		}
		if json.Unmarshal(raw, &after) != nil || after.CreatedAt == "" || !platform.ValidIdentifier(after.TurnID) {
			return ListTurnRecordsResult{}, platform.NewError(platform.CodeValidation, false, "记录分页游标不合法")
		}
	}
	rows, err := tx.Tx.QueryContext(tx.Context, `SELECT t.turn_id,t.question,t.answer_kind,COALESCE(f.value,''),t.created_at FROM turns t LEFT JOIN feedback f ON f.turn_id=t.turn_id AND f.visitor_subject=t.visitor_subject WHERE t.site_id=? AND t.data_epoch=? AND t.redacted=0 AND (?='' OR t.created_at<? OR (t.created_at=? AND t.turn_id<?)) AND (?='' OR t.language=?) AND (?='all' OR (?='no_evidence' AND t.answer_kind='no_evidence') OR (?='unhelpful' AND f.value='unhelpful') OR (?='failed' AND t.state='failed')) ORDER BY t.created_at DESC, t.turn_id DESC LIMIT ?`, op.Actor.SiteID, op.DataEpoch, after.CreatedAt, after.CreatedAt, after.CreatedAt, after.TurnID, input.Language, input.Language, input.Filter, input.Filter, input.Filter, input.Filter, input.Page.Limit+1)
	if err != nil {
		return ListTurnRecordsResult{}, qaruntime.StorageError(err)
	}
	defer rows.Close()
	items := make([]TurnRecordItem, 0)
	for rows.Next() {
		var item TurnRecordItem
		var question string
		if err := rows.Scan(&item.TurnID, &question, &item.AnswerKind, &item.Feedback, &item.CreatedAt); err != nil {
			return ListTurnRecordsResult{}, qaruntime.StorageError(err)
		}
		runes := []rune(question)
		item.QuestionPreview = string(runes[:min(200, len(runes))])
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ListTurnRecordsResult{}, qaruntime.StorageError(err)
	}
	result := ListTurnRecordsResult{Items: items[:min(len(items), input.Page.Limit)], PageInfo: gen.PageInfo{HasMore: len(items) > input.Page.Limit}}
	if result.PageInfo.HasMore {
		last := items[input.Page.Limit-1]
		raw, _ := json.Marshal(struct{ CreatedAt, TurnID string }{last.CreatedAt, last.TurnID})
		cursor, err := serviceauth.SignCursor(tx.Runtime.SigningKey, scope, raw)
		if err != nil {
			return ListTurnRecordsResult{}, err
		}
		result.PageInfo.NextCursor = &cursor
	}
	return result, nil
}

// ReadTurnRecord 保留原引用，同时标注已知失效；权威未知不能标成有效或静默遗漏。
func ReadTurnRecord(ctx context.Context, op OperationContext, input ReadTurnRecordInput) (ReadTurnRecordResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.manage"); err != nil {
		return ReadTurnRecordResult{}, err
	}
	read, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return ReadTurnRecordResult{}, err
	}
	record, err := qaruntime.ReadTurn(read, op, input.TurnID, false)
	read.Close()
	if err != nil {
		return ReadTurnRecordResult{}, err
	}
	result := ReadTurnRecordResult{TurnID: input.TurnID, Question: record.Question, Answer: record.Answer, Citations: make([]gen.BlockRef, 0, len(record.Citations)), AnswerKind: record.AnswerKind, CitationStatuses: make([]CitationStatus, 0, len(record.Citations))}
	result.Sources = record.Citations
	for _, citation := range record.Citations {
		result.Citations = append(result.Citations, citation.Block)
	}
	// 不返回访客标识、IP、准入token或模型请求体。
	return result, nil
}
