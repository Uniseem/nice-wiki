// Package ownership 回答“这个站点的 AI 费用算在谁头上”：站点所有者、他的额度、哪些任务可用平台渠道。
// 策略由 identity 给出（站点所有者与额度权益在那里），这里只做短时缓存，不保存任何账户数据。
package ownership

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
)

// Prefix 是用户自带渠道的供应方标识前缀。
const Prefix = "byok-"

// AllowLoopback 只在本地开发（allowLocalHTTP）时置位：用户渠道可以指向本机的假供应方。
var AllowLoopback bool

// IsChannelProvider 判断一个供应方是不是用户自带的渠道。
func IsChannelProvider(providerID string) bool { return strings.HasPrefix(providerID, Prefix) }

// ModelsFor 返回用户的渠道模型（按排序）；任务所属服务在渠道的任务范围内才算。
func ModelsFor(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID, ownerService string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT model_id,tasks_json FROM user_channels WHERE user_id=? ORDER BY position, created_at`, userID)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "读取 AI 渠道失败")
	}
	defer rows.Close()
	var models []string
	for rows.Next() {
		var id, tasksJSON string
		if rows.Scan(&id, &tasksJSON) != nil {
			continue
		}
		var tasks []string
		_ = json.Unmarshal([]byte(tasksJSON), &tasks)
		if len(tasks) == 0 || slices.Contains(tasks, ownerService) {
			models = append(models, id)
		}
	}
	return models, nil
}

// Policy 是 identity 的 readSiteAiPolicy 返回的站点 AI 策略。
type Policy struct {
	SiteID          string   `json:"siteId"`
	OwnerUserID     string   `json:"ownerUserId"`
	PlatformOwner   bool     `json:"platformOwner"`
	PlatformAllowed bool     `json:"platformAllowed"`
	CapMicros       int64    `json:"capMicros"`
	PeriodStartNs   int64    `json:"periodStartNs"`
	Tasks           []string `json:"tasks"`
	SiteIDs         []string `json:"siteIds"`
	VisitorQaDaily  int      `json:"visitorQaDaily"`
}

// PlatformFor 判断这个策略下，某个任务（按所属服务）能否使用平台渠道。
func (p Policy) PlatformFor(ownerService string) bool {
	if p.PlatformOwner {
		return true
	}
	return p.PlatformAllowed && (len(p.Tasks) == 0 || slices.Contains(p.Tasks, ownerService))
}

// Fetch 向 identity 读取一个站点的策略。
type Fetch func(ctx context.Context, siteID string, dataEpoch platform.Identifier) (Policy, error)

const ttl = 20 * time.Second

var (
	mu      sync.Mutex
	fetch   Fetch
	entries = map[string]entry{}
)

type entry struct {
	policy  Policy
	expires time.Time
}

// Configure 装配读取函数（进程启动时调用一次）。
func Configure(f Fetch) {
	mu.Lock()
	fetch = f
	mu.Unlock()
}

// Lookup 取站点策略，缓存 20 秒；改了额度或兑换新码后最多这么久生效。
func Lookup(ctx context.Context, siteID string, dataEpoch platform.Identifier) (Policy, error) {
	mu.Lock()
	if cached, ok := entries[siteID]; ok && time.Now().Before(cached.expires) {
		mu.Unlock()
		return cached.policy, nil
	}
	f := fetch
	mu.Unlock()
	if f == nil {
		return Policy{}, platform.NewError(platform.CodeDependencyUnavailable, true, "站点 AI 策略读取尚未装配")
	}
	policy, err := f(ctx, siteID, dataEpoch)
	if err != nil {
		return Policy{}, err
	}
	mu.Lock()
	entries[siteID] = entry{policy, time.Now().Add(ttl)}
	mu.Unlock()
	return policy, nil
}

// Cached 只读缓存（在数据库事务里使用，不能发远程请求），不看过期时间；没有就返回 false。
func Cached(siteID string) (Policy, bool) {
	mu.Lock()
	defer mu.Unlock()
	cached, ok := entries[siteID]
	return cached.policy, ok
}

// Forget 让某个站点的缓存失效（兑换新码后立即生效用）。
func Forget(siteID string) {
	mu.Lock()
	delete(entries, siteID)
	mu.Unlock()
}
