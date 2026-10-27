// Package sitestate 让租户组件跟上站点生命周期（identity 发的事件）：
//
//   - identity.site.changed：站点状态记进本组件的 nw_site_state，pgdb.Sites 只列正常站点，
//     后台循环（翻译、索引、概括、备份、清理）就不再处理下架和已删除的站点，恢复后接着处理。
//   - identity.site.purged：已删除站点保留期满，先清组件存在库外的数据（文件、外部索引），再删各租户表里该站点的行。
//
// 两个处理都可重复执行，不需要收件箱去重。
package sitestate

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
	"nice-wiki/libs/platform/pgdb"
)

const (
	changedEvent = "identity.site.changed"
	purgedEvent  = "identity.site.purged"
)

// Config 是一个组件跟踪站点生命周期需要的东西。
type Config struct {
	Module  string // 组件名，消费者 ID 是“组件名.sites”
	DB      *sql.DB
	Events  platform.EventConsumerClient // 队列的消费客户端（queue.Queue.Consumer()）
	Runtime func(context.Context) (platform.RuntimeView, error)
	// Purge 清掉组件存在库外的该站点数据；在删行之前调用，可以先查库找到文件。没有库外数据的组件留空。
	Purge func(ctx context.Context, site string) error
}

type payload struct {
	SiteID   string `json:"siteId"`
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}

// Task 是订阅站点生命周期事件的后台任务。
func Task(cfg Config) platform.BackgroundTask {
	return platform.BackgroundTask{Name: cfg.Module + ".site-lifecycle", Run: func(ctx context.Context) error {
		return inbox.RunConsumer(ctx, cfg.Events, platform.ConsumerConfig{
			ConsumerID: cfg.Module + ".sites", Events: []string{changedEvent + "@1", purgedEvent + "@1"}, MaxInFlight: 1,
			Runtime: cfg.Runtime, ValidatePayload: validate,
		}, platform.EventHandlerRegistry{
			changedEvent: func(ctx context.Context, delivery platform.EventDelivery) error {
				p := decode(delivery)
				return pgdb.RecordSite(ctx, cfg.DB, p.SiteID, p.Status, p.Revision)
			},
			purgedEvent: func(ctx context.Context, delivery platform.EventDelivery) error {
				p := decode(delivery)
				if cfg.Purge != nil {
					if err := cfg.Purge(ctx, p.SiteID); err != nil {
						return err
					}
				}
				if err := pgdb.PurgeSite(ctx, cfg.DB, p.SiteID); err != nil {
					return err
				}
				return pgdb.RecordSite(ctx, cfg.DB, p.SiteID, "purged", p.Revision)
			},
		})
	}}
}

// validate 是事件入口的校验：站点 ID 会拼进文件路径（清除时），不能带路径分隔符或点。
func validate(eventType string, _ int, raw json.RawMessage) error {
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil || !platform.ValidIdentifier(p.SiteID) || strings.ContainsAny(p.SiteID, `./\`) || p.Revision < 1 {
		return platform.NewError(platform.CodeValidation, false, "站点事件载荷不合法")
	}
	if eventType == changedEvent && p.Status != "active" && p.Status != "suspended" && p.Status != "deleted" {
		return platform.NewError(platform.CodeValidation, false, "站点状态不合法")
	}
	return nil
}

func decode(delivery platform.EventDelivery) payload {
	var p payload
	_ = json.Unmarshal(delivery.Envelope.Payload, &p)
	return p
}
