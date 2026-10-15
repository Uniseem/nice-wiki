package internal

import (
	"encoding/json"
	"net/http"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/problem"
)

// NewEventHandlers 仅交给统一NewServiceHandler；身份、epoch、schema与路径值已在入口建立。
// 其余业务handler未完成时，不以本组映射冒充完整Application。
func NewEventHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"publishEvent": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input PublishEventInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "事件请求绑定失败"), 0)
				return
			}
			result, err := PublishEvent(r.Context(), op, input)
			writeEventResponse(w, op.RequestID, result, err, http.StatusAccepted)
		}),
		"acknowledgeEvent": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body struct {
				ConsumerID string `json:"consumerId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "事件确认请求绑定失败"), 0)
				return
			}
			result, err := AcknowledgeEvent(r.Context(), op, AcknowledgeEventInput{EventID: r.PathValue("eventId"), ConsumerID: body.ConsumerID, DataEpoch: op.DataEpoch})
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"replayDelivery": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := ReplayDelivery(r.Context(), op, ReplayDeliveryInput{DeliveryID: r.PathValue("deliveryId")})
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"subscribeEvents": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := SubscribeEvents(r.Context(), op, SubscribeEventsInput{ConsumerID: r.URL.Query().Get("consumerId"), Mode: r.URL.Query().Get("mode"), LastEventID: r.Header.Get("Last-Event-ID"), DataEpoch: op.DataEpoch})
			if err != nil {
				writeEventResponse(w, op.RequestID, nil, err, 0)
				return
			}
			// 公共pump开始后只可结束流，不能再追加JSON Problem。
			_ = apivalidate.ServeEventStream(r.Context(), w, result.Stream)
		}),
	}
}

func writeEventResponse(w http.ResponseWriter, requestID platform.Identifier, result any, err error, status int) {
	if err != nil {
		_ = problem.WriteProblem(w, problem.FromError(err, requestID))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(result)
}
