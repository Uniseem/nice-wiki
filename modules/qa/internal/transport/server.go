// Package transport 映射已完成用例；完整服务仍须绑定全部合同路由。
package transport

import (
	"context"
	"encoding/json"
	oapiruntime "github.com/oapi-codegen/runtime"
	"io"
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/qa/internal/admin"
	"nice-wiki/modules/qa/internal/answer"
	"nice-wiki/modules/qa/internal/cache"
	"nice-wiki/modules/qa/internal/conversation"
	qadebug "nice-wiki/modules/qa/internal/debug"
	"nice-wiki/modules/qa/internal/feedback"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/stream"
	"nice-wiki/modules/qa/internal/turn"
	"strconv"
	"time"
)

// NewOperationHandlers 仅交平台NewServiceHandler装配，统一校验、鉴权及门禁在该入口完成。
func NewOperationHandlers(bind func(context.Context) context.Context) map[string]http.Handler {
	return map[string]http.Handler{
		"readCacheTranslationSource": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
			result, err := cache.ReadCacheTranslationSource(ctx, op, cache.ReadCacheTranslationSourceInput{CacheId: r.PathValue("cacheId")})
			writeResult(w, op, http.StatusOK, result, err)
		}),
		"proposeCacheEntry": jsonOperation(bind, http.StatusCreated, cache.ProposeCacheEntry, nil),
		"approveCacheEntry": jsonOperation(bind, http.StatusOK, cache.ApproveCacheEntry, func(r *http.Request, i *gen.ApproveCacheEntryInput) error {
			if i.CacheId != r.PathValue("cacheId") {
				return platform.NewError(platform.CodeValidation, false, "批准路径与正文条目不一致")
			}
			return nil
		}),
		"cancelTurn": jsonOperation(bind, http.StatusOK, turn.CancelTurn, func(r *http.Request, input *gen.CancelTurnInput) error {
			if input.TurnId != r.PathValue("turnId") {
				return platform.NewError(platform.CodeValidation, false, "路径与取消轮次不一致")
			}
			return nil
		}),
		"deleteConversation": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
			result, err := conversation.DeleteConversation(ctx, op, conversation.DeleteConversationInput{ConversationId: r.PathValue("conversationId"), IdempotencyKey: r.Header.Get("Idempotency-Key")})
			writeResult(w, op, 200, result, err)
		}),
		"createConversation": jsonOperation(bind, http.StatusCreated, conversation.CreateConversation, nil),
		"startTurn":          jsonOperation(bind, http.StatusAccepted, turn.StartTurn, nil),
		"searchDocuments":    jsonOperation(bind, http.StatusOK, answer.SearchDocuments, nil),
		"createDebugTurn":    jsonOperation(bind, http.StatusAccepted, qadebug.CreateDebugTurn, nil),
		"recordFeedback": jsonOperation(bind, http.StatusOK, feedback.RecordFeedback, func(r *http.Request, input *gen.RecordFeedbackInput) error {
			if input.TurnId != r.PathValue("turnId") {
				return platform.NewError(platform.CodeValidation, false, "路径与反馈轮次不一致")
			}
			return nil
		}),
		"disableCacheEntry": jsonOperation(bind, http.StatusOK, cache.DisableCacheEntry, func(r *http.Request, input *gen.DisableCacheEntryInput) error {
			if input.CacheId != r.PathValue("cacheId") {
				return platform.NewError(platform.CodeValidation, false, "路径与缓存对象不一致")
			}
			return nil
		}),
		"getConversation": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			result, err := conversation.GetConversation(ctx, op, conversation.GetConversationInput{ConversationID: r.PathValue("conversationId"), Cursor: r.URL.Query().Get("cursor"), Limit: limit})
			writeResult(w, op, 200, result, err)
		}),
		"putQaPolicy": jsonOperation(bind, http.StatusOK, admin.PutQaPolicy, nil),
		"readQaPolicy": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
			result, err := admin.ReadQaPolicy(ctx, op, admin.ReadQaPolicyInput{})
			writeResult(w, op, 200, result, err)
		}),
		"readPublicCapability": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
			result, err := admin.ReadPublicCapability(ctx, op, admin.ReadPublicCapabilityInput{})
			writeResult(w, op, 200, result, err)
		}),
		"listCacheEntries": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			result, err := admin.ListCacheEntries(ctx, op, admin.ListCacheEntriesInput{State: r.URL.Query().Get("state"), Cursor: r.URL.Query().Get("cursor"), Limit: limit})
			writeResult(w, op, 200, result, err)
		}),
		"readCacheCalibration": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
			var space gen.EmbeddingSpace
			if err := oapiruntime.BindQueryParameter("form", true, true, "space", r.URL.Query(), &space); err != nil {
				writeResult(w, op, 0, nil, platform.NewError(platform.CodeValidation, false, "校准空间查询不可读"))
				return
			}
			result, err := admin.ReadCacheCalibration(ctx, op, admin.ReadCacheCalibrationInput{From: r.URL.Query().Get("from"), Until: r.URL.Query().Get("until"), Space: space})
			writeResult(w, op, 200, result, err)
		}),
		"listTurnRecords": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
			var page admin.RecordPage
			if err := oapiruntime.BindQueryParameter("form", true, true, "page", r.URL.Query(), &page); err != nil {
				writeResult(w, op, 0, nil, platform.NewError(platform.CodeValidation, false, "问答记录分页不可读"))
				return
			}
			result, err := admin.ListTurnRecords(ctx, op, admin.ListTurnRecordsInput{Page: page, Filter: r.URL.Query().Get("filter"), Language: r.URL.Query().Get("language")})
			writeResult(w, op, 200, result, err)
		}),
		"readTurnRecord": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
			result, err := admin.ReadTurnRecord(ctx, op, admin.ReadTurnRecordInput{TurnID: r.PathValue("turnId")})
			writeResult(w, op, 200, result, err)
		}),
		"readDebugTurn": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
			after, _ := strconv.Atoi(r.URL.Query().Get("afterSequence"))
			result, err := qadebug.ReadDebugTurn(ctx, op, qadebug.ReadDebugTurnInput{DebugID: r.PathValue("debugId"), AfterSequence: after})
			writeResult(w, op, 200, result, err)
		}),
		"resumeAnswerStream": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveAnswerStream(w, r, bind) }),
	}
}
func jsonOperation[I, R any](bind func(context.Context) context.Context, status int, run func(context.Context, platform.OperationContext, I) (R, error), path func(*http.Request, *I) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, op, err := operationContext(r, bind)
		if err != nil {
			writeResult(w, op, 0, nil, err)
			return
		}
		var input I
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			writeResult(w, op, 0, nil, platform.NewError(platform.CodeValidation, false, "请求正文不可读"))
			return
		}
		if path != nil {
			if err := path(r, &input); err != nil {
				writeResult(w, op, 0, nil, err)
				return
			}
		}
		result, err := run(ctx, op, input)
		writeResult(w, op, status, result, err)
	})
}
func operationContext(r *http.Request, bind func(context.Context) context.Context) (context.Context, platform.OperationContext, error) {
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok || bind == nil {
		return nil, op, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信服务入口")
	}
	return bind(r.Context()), op, nil
}
func writeResult(w http.ResponseWriter, op platform.OperationContext, status int, result any, err error) {
	if err != nil {
		_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeInternal, false, "结果编码失败"), op.RequestID))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// serveAnswerStream 排他续读，心跳不持久化；过期在发送headers前返回410。
func serveAnswerStream(w http.ResponseWriter, r *http.Request, bind func(context.Context) context.Context) {
	ctx, op, err := operationContext(r, bind)
	if err != nil {
		writeResult(w, op, 0, nil, err)
		return
	}
	after, _ := strconv.Atoi(r.URL.Query().Get("afterSequence"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if last := r.Header.Get("Last-Event-ID"); last != "" {
		value, err := strconv.Atoi(last)
		if err != nil || value < 0 {
			writeResult(w, op, 0, nil, platform.NewError(platform.CodeValidation, false, "流续读游标不合法"))
			return
		}
		after = max(after, value)
	}
	input := stream.ResumeAnswerStreamInput{TurnID: r.PathValue("turnId"), AfterSequence: after, Limit: limit}
	batch, err := stream.ResumeAnswerStream(ctx, op, input)
	if err != nil {
		writeResult(w, op, 0, nil, err)
		return
	}
	work, cancel := context.WithTimeout(ctx, 5*time.Minute)
	reader := &answerEventStream{op: op, input: input, batch: batch, cancel: cancel}
	_ = apivalidate.ServeJSONEventStream(work, w, reader)
}

// answerEventStream只负责本域持久事件的排他读取；通用心跳和背压由平台提供。
type answerEventStream struct {
	op     platform.OperationContext
	input  stream.ResumeAnswerStreamInput
	batch  stream.ResumeAnswerStreamResult
	cancel context.CancelFunc
}

func (reader *answerEventStream) Next(ctx context.Context) (platform.SSEFrame, error) {
	for {
		if len(reader.batch.Events) > 0 {
			event := reader.batch.Events[0]
			reader.batch.Events = reader.batch.Events[1:]
			reader.input.AfterSequence = event.Sequence
			raw, err := json.Marshal(event)
			if err != nil {
				return platform.SSEFrame{}, platform.NewError(platform.CodeIntegrityError, false, "流事件无法编码")
			}
			return platform.SSEFrame{ID: strconv.Itoa(event.Sequence), Event: string(event.Event), Data: raw}, nil
		}
		if reader.batch.Terminal {
			return platform.SSEFrame{}, io.EOF
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return platform.SSEFrame{}, ctx.Err()
		case <-timer.C:
		}
		batch, err := stream.ResumeAnswerStream(ctx, reader.op, reader.input)
		if err != nil {
			return platform.SSEFrame{}, err
		}
		reader.batch = batch
	}
}
func (reader *answerEventStream) Close() error { reader.cancel(); return nil }
