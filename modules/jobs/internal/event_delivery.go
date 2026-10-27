package internal

import (
	"context"
	"sync/atomic"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/queue"
	"nice-wiki/modules/jobs/internal/gen"
)

type ReplayDeliveryInput struct {
	DeliveryID string `json:"deliveryId"`
}
type ReplayDeliveryResult = gen.ReplayDeliveryResult

var eventQueue atomic.Pointer[queue.Queue]

// InstallEventQueue 由装配层注入共用事件队列；死信的查看和重放、历史清理都读写它。
func InstallEventQueue(q *queue.Queue) { eventQueue.Store(q) }

func currentEventQueue() (*queue.Queue, error) {
	q := eventQueue.Load()
	if q == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "事件队列尚未装配")
	}
	return q, nil
}

// authorizeDeliveryManagement 只放行当前站点的死信，并按生产方登记的管理权限核对当前用户。
func authorizeDeliveryManagement(ctx context.Context, op OperationContext, deliveryID string) (queue.DeadLetter, error) {
	q, err := currentEventQueue()
	if err != nil {
		return queue.DeadLetter{}, err
	}
	store := currentStore()
	if store == nil {
		return queue.DeadLetter{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	store.mu.RLock()
	client := store.identity
	store.mu.RUnlock()
	if client == nil {
		return queue.DeadLetter{}, platform.NewError(platform.CodeDependencyUnavailable, true, "事件重放授权尚未装配")
	}
	letter, err := q.DeadLetterByID(ctx, deliveryID)
	if err != nil {
		return queue.DeadLetter{}, err
	}
	if letter.SiteID != op.Actor.SiteID {
		return queue.DeadLetter{}, platform.NewError(platform.CodeNotFound, false, "事件投递不存在")
	}
	definition, err := findEventDefinition(letter.EventType, letter.EventVersion)
	if err != nil || definition.Producer != letter.Producer || definition.ManagementPermission == "" {
		return queue.DeadLetter{}, platform.NewError(platform.CodeIntegrityError, false, "持久事件缺少登记生产方或管理权限")
	}
	if err := authorizePermission(ctx, op, client, definition.ManagementPermission, letter.DeliveryID); err != nil {
		return queue.DeadLetter{}, err
	}
	return letter, nil
}

// ReplayDelivery 只重排死信，不能覆盖已确认或正在处理的投递。
func ReplayDelivery(ctx context.Context, op OperationContext, in ReplayDeliveryInput) (ReplayDeliveryResult, error) {
	letter, err := authorizeDeliveryManagement(ctx, op, in.DeliveryID)
	if err != nil {
		return ReplayDeliveryResult{}, err
	}
	q, err := currentEventQueue()
	if err != nil {
		return ReplayDeliveryResult{}, err
	}
	if err := q.Replay(ctx, letter.DeliveryID); err != nil {
		return ReplayDeliveryResult{}, err
	}
	return ReplayDeliveryResult{Queued: true}, nil
}
