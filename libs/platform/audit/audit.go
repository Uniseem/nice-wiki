// Package audit 把后台写操作记成审计事实：各服务 HTTP 边界在操作成功后，以本服务身份向 analytics 追加一条历史审计
// （原主体为后台账号）。事实先原子落盘到本服务数据目录下的待交付目录，再由后台循环送达；analytics 暂时不可用或本进程
// 重启都不会丢，按稳定的 auditId 重发也不会重复。送达或被 analytics 明确拒绝才删除。
package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// readLike 是用 POST 发出的校验/只读类接口，不算后台写操作。
var readLike = map[string]bool{
	"identity.checkAuthorization": true, "identity.verifySessionCSRF": true,
	"content.validatePreviewGrant": true, "content.validatePublications": true, "content.diffRevisions": true,
}

// 待交付事实的重试间隔：失败后从 2 秒起翻倍，最长 5 分钟。
const (
	retryFirst = 2 * time.Second
	retryCap   = 5 * time.Minute
)

type emitter struct {
	module string
	base   string
	client *http.Client
	spool  string
	wake   chan struct{}
}

// fact 是一条待交付的审计事实，原样重发同一份正文。
type fact struct {
	AuditID   string          `json:"auditId"`
	SiteID    string          `json:"siteId"`
	DataEpoch string          `json:"dataEpoch"`
	Body      json.RawMessage `json:"body"`
}

var current *emitter

// Install 在服务启动时装配并开始交付上次没送完的事实；没有 analytics 依赖或本服务就是 analytics 时不审计。
func Install(config platform.ServiceConfig, key platform.SigningKey) {
	base := strings.TrimRight(config.Dependencies["analytics"], "/")
	if base == "" || config.Module == "analytics" {
		return
	}
	spool := filepath.Join(config.DataRoot, "audit-spool")
	if err := os.MkdirAll(spool, 0o700); err != nil {
		debug("审计待交付目录不可用：%v", err)
		return
	}
	current = &emitter{module: config.Module, base: base, spool: spool, wake: make(chan struct{}, 1),
		client: &http.Client{Transport: serviceauth.SigningTransport(config.Module, "analytics", key, nil)}}
	current.wake <- struct{}{} // 先送上次进程没送完的
	go current.deliver()
}

// Record 在成功的后台写操作之后调用：仅当前服务已装配、操作者是后台账号且动作在合同枚举内才记。
func Record(op platform.OperationContext, method, operationID string, params map[string]string, status int) {
	e := current
	if e == nil || status < 200 || status >= 300 || method == http.MethodGet || method == http.MethodHead || op.Actor.Kind != "admin" {
		return
	}
	action := e.module + "." + operationID
	if !knownActions[action] || readLike[action] {
		return
	}
	resource := "site"
	for _, name := range []string{"documentId", "accountId", "assetId", "jobId", "backupId", "restoreId", "revisionId"} {
		if v := params[name]; v != "" {
			resource = v
			break
		}
	}
	id := platform.NewIdentifier()
	body, _ := json.Marshal(map[string]any{
		"auditId": id, "action": action, "resourceId": resource, "resultCode": "OK", "metadata": "{}",
		"actorKind": "admin", "actorId": op.Actor.SubjectID, "occurredAt": time.Now().UTC().Format(time.RFC3339Nano), "sourceOperationId": op.RootOperationID,
	})
	if err := e.store(fact{AuditID: id, SiteID: op.Actor.SiteID, DataEpoch: op.DataEpoch, Body: body}); err != nil {
		debug("审计事实落盘失败 %s: %v", action, err)
		return
	}
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// store 先写临时文件再改名，读到的永远是完整的一条；文件名以时间开头，按发生顺序交付。
func (e *emitter) store(item fact) error {
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(e.spool, ".pending-")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(raw); err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	name := strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + item.AuditID + ".json"
	return os.Rename(temp.Name(), filepath.Join(e.spool, name))
}

// deliver 有新事实就送；送不出去按退避间隔重试，直到目录清空。
func (e *emitter) deliver() {
	wait := time.Duration(0)
	for {
		if wait > 0 {
			select {
			case <-e.wake:
			case <-time.After(wait):
			}
		} else {
			<-e.wake
		}
		if e.flush() {
			wait = 0
		} else if wait = max(retryFirst, wait*2); wait > retryCap {
			wait = retryCap
		}
	}
}

// flush 按顺序交付目录里的事实，遇到暂时失败就停下，返回是否已全部送完。
func (e *emitter) flush() bool {
	entries, err := os.ReadDir(e.spool)
	if err != nil {
		debug("审计待交付目录不可读：%v", err)
		return false
	}
	names := []string{}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(e.spool, name)
		raw, err := os.ReadFile(path)
		var item fact
		if err != nil || json.Unmarshal(raw, &item) != nil {
			debug("审计待交付事实损坏，丢弃 %s", name)
			_ = os.Remove(path)
			continue
		}
		if !e.send(item) {
			return false
		}
		_ = os.Remove(path)
	}
	return true
}

// send 返回 false 表示暂时送不到、稍后重试；analytics 明确拒绝（如数据代次已换、动作不属于本服务）视为处理完毕。
func (e *emitter) send(item fact) bool {
	deadline := time.Now().UTC().Add(10 * time.Second)
	self := platform.OperationContext{RequestID: platform.NewIdentifier(), OperationID: item.AuditID, RootOperationID: item.AuditID, CallerService: e.module, Deadline: deadline.Format(time.RFC3339Nano), DataEpoch: item.DataEpoch,
		Actor: platform.AuthContext{Kind: "service", SubjectID: e.module, SiteID: item.SiteID, ExpiresAt: deadline.Format(time.RFC3339Nano), Grants: []string{}}}
	ctx, cancel := context.WithDeadline(platform.WithPortalProof(context.Background(), ""), deadline)
	defer cancel()
	ctx = platform.WithOperationContext(ctx, self)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.base+"/v1/audits", bytes.NewReader(item.Body))
	if err != nil {
		return true
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", item.AuditID)
	resp, err := e.client.Do(req)
	if err != nil {
		debug("审计发送失败 %s: %v", item.AuditID, err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 {
		return true
	}
	var problem platform.Problem
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&problem)
	if resp.StatusCode >= 500 || problem.Retryable || problem.Code == platform.CodeLocked || problem.Code == platform.CodeRateLimited {
		debug("审计暂未送达 %s status=%d code=%s", item.AuditID, resp.StatusCode, problem.Code)
		return false
	}
	debug("审计被拒绝，丢弃 %s status=%d code=%s", item.AuditID, resp.StatusCode, problem.Code)
	return true
}

func debug(format string, args ...any) {
	if os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
