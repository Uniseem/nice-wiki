package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// visitorSubject 与 traffic-guard 的匿名主体推导一致：没有访客证明时按网络预算作用域区分。
func visitorSubject(key platform.SigningKey, site platform.Identifier, network platform.Hash) (platform.Identifier, error) {
	payload, err := platform.CanonicalJSON(map[string]string{"siteId": site, "sessionId": "anonymous:" + network})
	if err != nil {
		return "", err
	}
	return serviceauth.PrivateOperationDigest(key, "guard.subject.v1", payload)
}

// prepareVisitorQA 为公开问答路由建立伪名访客身份：需要准入的写操作先向 traffic-guard 申请许可并写进请求体，
// 其余读写按同一主体推导。返回新的请求体（可能改写）、委托证明与已带网络预算作用域的操作上下文。
func prepareVisitorQA(ctx context.Context, op OperationContext, runtime *ProxyRuntime, route ResolvedRoute) ([]byte, InternalProof, OperationContext, error) {
	if runtime.SiteID == "" || len(runtime.NetworkScopeKeys) == 0 {
		return nil, InternalProof{}, op, platform.NewError(platform.CodeDependencyUnavailable, true, "问答访客身份的预算密钥尚未装配")
	}
	scopes := []platform.Hash{}
	for _, key := range runtime.NetworkScopeKeys {
		values, err := serviceauth.PrivateBudgetNetworkScopes(key, runtime.Request.RemoteAddr, strings.Join(runtime.Request.Header.Values("X-Forwarded-For"), ","), runtime.TrustedProxyCIDRs, runtime.SiteID, time.Now())
		if err != nil {
			return nil, InternalProof{}, op, err
		}
		scopes = append(scopes, values...)
	}
	op.Actor.SiteID = runtime.SiteID
	op.NetworkBudgetScopes = scopes
	body := runtime.Body
	// 完成人机验证后，浏览器在 X-Guard-Session 里带回 traffic-guard 签的通过证明；它决定访客主体。
	token := runtime.Request.Header.Get("X-Guard-Session")
	subject, err := visitorSubject(runtime.NetworkScopeKeys[0], runtime.SiteID, scopes[0])
	if err != nil {
		return nil, InternalProof{}, op, err
	}
	if token != "" && len(token) <= 8192 {
		if claims, verr := serviceauth.VerifyGuardSession(runtime.Anchors, token); verr == nil && claims.SiteID == runtime.SiteID {
			subject = claims.SubjectID
		} else {
			token = ""
		}
	} else {
		token = ""
	}
	if action := route.Definition.Admission; action != "" {
		permit, err := createAdmission(ctx, op, runtime, action, token)
		if err != nil {
			return nil, InternalProof{}, op, err
		}
		subject = permit.SubjectID
		var fields map[string]any
		if json.Unmarshal(body, &fields) != nil || fields == nil {
			return nil, InternalProof{}, op, platform.NewError(platform.CodeValidation, false, "问答请求正文不可读")
		}
		fields["admission"] = permit
		if body, err = json.Marshal(fields); err != nil {
			return nil, InternalProof{}, op, platform.NewError(platform.CodeInternal, false, "准入许可无法写入请求")
		}
	}
	expires := time.Now().UTC().Add(9 * time.Second)
	if deadline, err := platform.ParseTimestamp(op.Deadline); err == nil && deadline.Before(expires) {
		expires = deadline
	}
	auth := platform.AuthContext{Kind: "visitor", SubjectID: subject, SiteID: runtime.SiteID, ExpiresAt: platform.Timestamp(expires.Format(time.RFC3339Nano)), Grants: []string{}}
	op.Actor = auth
	proof, err := BuildInternalProof(WithPortalSigner(ctx, runtime.DelegationKey), op, BuildInternalProofInput{
		Auth: auth, Route: route, RequestDigest: platform.HashBytes(body), ExpiresAt: auth.ExpiresAt,
	})
	if err != nil {
		return nil, InternalProof{}, op, err
	}
	return body, proof.Proof, op, nil
}

// createAdmission 以入口身份向 traffic-guard 申请本次提问/建会话的准入许可。
func createAdmission(ctx context.Context, op OperationContext, runtime *ProxyRuntime, action, visitorToken string) (platform.AdmissionPermit, error) {
	base := strings.TrimRight(runtime.Dependencies["traffic-guard"], "/")
	if base == "" {
		return platform.AdmissionPermit{}, platform.NewError(platform.CodeDependencyUnavailable, true, "风控服务地址未配置")
	}
	raw, _ := json.Marshal(map[string]string{"requestId": op.RootOperationID, "action": action, "visitorToken": visitorToken})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/admissions", bytes.NewReader(raw))
	if err != nil {
		return platform.AdmissionPermit{}, platform.NewError(platform.CodeInternal, false, "准入请求构造失败")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Idempotency-Key", op.RootOperationID)
	guardOp := op
	guardOp.Actor = platform.AuthContext{Kind: "service", SubjectID: "portal", SiteID: runtime.SiteID, ExpiresAt: op.Deadline, Grants: []string{}}
	request = request.WithContext(platform.WithPortalProof(platform.WithOperationContext(request.Context(), guardOp), ""))
	client := &http.Client{Transport: serviceauth.SigningTransport("portal", "traffic-guard", runtime.SigningKey, nil), Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return platform.AdmissionPermit{}, platform.NewError(platform.CodeDependencyUnavailable, true, "风控服务不可达")
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if response.StatusCode >= 300 {
		var problem struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(payload, &problem)
		if _, known := platform.ProblemHTTPStatus[problem.Code]; !known {
			problem.Code = platform.CodeDependencyUnavailable
		}
		return platform.AdmissionPermit{}, platform.NewError(problem.Code, problem.Code == platform.CodeRateLimited, "%s", problem.Detail)
	}
	var result struct {
		Permit platform.AdmissionPermit `json:"permit"`
	}
	if json.Unmarshal(payload, &result) != nil || result.Permit.AdmissionID == "" {
		return platform.AdmissionPermit{}, platform.NewError(platform.CodeIntegrityError, false, "准入许可响应不可读")
	}
	return result.Permit, nil
}
