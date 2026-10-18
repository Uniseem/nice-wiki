// Package transport 映射已完成用例；完整服务仍须绑定全部合同路由。
package transport

import (
	"context"
	"encoding/json"
	"fmt"
	oapiruntime "github.com/oapi-codegen/runtime"
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/qa/internal/admin"
	"nice-wiki/modules/qa/internal/cache"
	"nice-wiki/modules/qa/internal/conversation"
	"nice-wiki/modules/qa/internal/feedback"
	"nice-wiki/modules/qa/internal/gen"
	"nice-wiki/modules/qa/internal/stream"
	"strconv"
	"time"
)

// NewOperationHandlers 仅交平台NewServiceHandler装配，统一校验、鉴权及门禁在该入口完成。
func NewOperationHandlers(bind func(context.Context) context.Context) map[string]http.Handler {
	return map[string]http.Handler{
		"proposeCacheEntry":  jsonOperation(bind, http.StatusCreated, cache.ProposeCacheEntry, nil),
		"createConversation": jsonOperation(bind, http.StatusCreated, conversation.CreateConversation, nil),
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
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	poll := time.NewTicker(250 * time.Millisecond)
	heartbeat := time.NewTicker(15 * time.Second)
	defer poll.Stop()
	defer heartbeat.Stop()
	for {
		for _, event := range batch.Events {
			raw, err := json.Marshal(event)
			if err != nil {
				return
			}
			if controller.SetWriteDeadline(time.Now().Add(5*time.Second)) != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Event, raw); err != nil {
				return
			}
			if controller.Flush() != nil {
				return
			}
			input.AfterSequence = event.Sequence
		}
		if batch.Terminal {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if controller.SetWriteDeadline(time.Now().Add(5*time.Second)) != nil {
				return
			}
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			if controller.Flush() != nil {
				return
			}
		case <-poll.C:
		}
		batch, err = stream.ResumeAnswerStream(ctx, op, input)
		if err != nil {
			return
		}
	}
}
