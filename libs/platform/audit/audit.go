// Package audit 把后台写操作记成审计事实：各服务 HTTP 边界在操作成功后
// 以本服务身份向 analytics 追加一条历史审计（原主体为后台账号），失败只记调试日志，不影响请求。
package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

type emitter struct {
	module string
	base   string
	client *http.Client
}

var current *emitter

// Install 在服务启动时装配；没有 analytics 依赖或本服务就是 analytics 时不审计。
func Install(config platform.ServiceConfig, key platform.SigningKey) {
	base := strings.TrimRight(config.Dependencies["analytics"], "/")
	if base == "" || config.Module == "analytics" {
		return
	}
	current = &emitter{module: config.Module, base: base, client: &http.Client{Transport: serviceauth.SigningTransport(config.Module, "analytics", key, nil)}}
}

// Record 在成功的后台写操作之后调用：仅当前服务已装配、操作者是后台账号且动作在合同枚举内才发出。
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
	go e.send(op, action, resource)
}

func (e *emitter) send(op platform.OperationContext, action, resource string) {
	deadline := time.Now().UTC().Add(10 * time.Second)
	id := platform.NewIdentifier()
	self := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: e.module, Deadline: deadline.Format(time.RFC3339Nano), DataEpoch: op.DataEpoch,
		Actor: platform.AuthContext{Kind: "service", SubjectID: e.module, SiteID: op.Actor.SiteID, ExpiresAt: deadline.Format(time.RFC3339Nano), Grants: []string{}}}
	ctx, cancel := context.WithDeadline(platform.WithPortalProof(context.Background(), ""), deadline)
	defer cancel()
	ctx = platform.WithOperationContext(ctx, self)
	body, _ := json.Marshal(map[string]any{
		"auditId": id, "action": action, "resourceId": resource, "resultCode": "OK", "metadata": "{}",
		"actorKind": "admin", "actorId": op.Actor.SubjectID, "occurredAt": time.Now().UTC().Format(time.RFC3339Nano), "sourceOperationId": op.RootOperationID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.base+"/v1/audits", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", id)
	resp, err := e.client.Do(req)
	if err != nil {
		debug("审计发送失败 %s: %v", action, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		debug("审计被拒绝 %s status=%d", action, resp.StatusCode)
	}
}

func debug(format string, args ...any) {
	if os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
