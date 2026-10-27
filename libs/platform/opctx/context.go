// Package opctx 从入站请求构建可信OperationContext。
//
// 调用方经服务断言确定callerService，用户委托确定actor；
// 浏览器自报的身份、费用类别与网络作用域一律忽略。
package opctx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// 未携带委托证明时服务调用方的统一站点身份。
const serviceSiteID = "system"

// 缺省截止时长；入站未携带X-Deadline时使用。
const defaultDeadline = 30 * time.Second

// BuildOperationContext 从入站请求构建可信OperationContext。
// 验证服务断言确定callerService；携带X-Portal-Proof时验证委托得到actor；
// dataEpoch取runtime当前active代次，请求声明不一致返回VERSION_MISMATCH。
func BuildOperationContext(r *http.Request, anchors platform.TrustAnchors, self string, runtime platform.RuntimeView) (platform.OperationContext, error) {
	if self == "" {
		return platform.OperationContext{}, platform.NewError(platform.CodeInternal, false, "本服务身份缺失")
	}
	bodyHash, err := hashRequestBody(r)
	if err != nil {
		return platform.OperationContext{}, err
	}
	token := strings.TrimPrefix(r.Header.Get(platform.HeaderAuthorization), platform.AssertionScheme)
	claims, err := serviceauth.VerifyAssertion(anchors, token, platform.AssertionExpectation{
		Audience: self,
		Method:   r.Method,
		Path:     r.URL.Path,
		BodyHash: bodyHash,
	})
	if err != nil {
		return platform.OperationContext{}, err
	}
	op := platform.OperationContext{
		RequestID:     requestID(r),
		OperationID:   operationID(r),
		CallerService: claims.Issuer,
		DataEpoch:     runtime.DataEpoch,
	}
	op.Deadline, err = deadline(r)
	if err != nil {
		if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "入口拒绝期限 caller=%s path=%s deadline=%q err=%v\n", claims.Issuer, r.URL.Path, r.Header.Get(platform.HeaderDeadline), err)
		}
		return platform.OperationContext{}, err
	}
	op.LoginNetworkScopes = append([]platform.Hash(nil), claims.LoginNetworkScopes...)
	op.NetworkBudgetScopes = append([]platform.Hash(nil), claims.NetworkBudgetScopes...)
	if root := r.Header.Get(platform.HeaderRootOperationID); platform.ValidIdentifier(root) {
		op.RootOperationID = root
	} else {
		op.RootOperationID = op.OperationID
	}
	if class := r.Header.Get(platform.HeaderCostClass); class != "" {
		switch class {
		case platform.CostClassQA, platform.CostClassBackground, platform.CostClassAdminProbe:
			// 只接受经验证调用方传递的费用类别。
			op.CostClass = class
		default:
			return platform.OperationContext{}, platform.NewError(platform.CodeValidation, false, "费用类别不合法")
		}
	}
	if cancel := r.Header.Get(platform.HeaderCancellationID); platform.ValidIdentifier(cancel) {
		op.CancellationID = cancel
	}
	if declared := r.Header.Get(platform.HeaderDataEpoch); declared != "" && declared != runtime.DataEpoch {
		return platform.OperationContext{}, platform.NewError(platform.CodeVersionMismatch, false, "数据代次与服务端不一致")
	}
	if proof := r.Header.Get(platform.HeaderPortalProof); proof != "" {
		delegation, err := serviceauth.VerifyDelegation(anchors, proof, platform.DelegationExpectation{CallerService: claims.Issuer, BodyHash: bodyHash})
		if err != nil {
			return platform.OperationContext{}, err
		}
		op.Actor = delegation.Actor
		if claims.SiteID != "" && claims.SiteID != op.Actor.SiteID {
			return platform.OperationContext{}, platform.NewError(platform.CodeForbidden, false, "服务断言站点与用户委托不一致")
		}
	} else {
		op.Actor = serviceActor(claims.Issuer, op.Deadline)
		if claims.SiteID != "" {
			op.Actor.SiteID = claims.SiteID
		}
	}
	return op, nil
}

// hashRequestBody 读取正文计算SHA-256后原样回填，不缓存全文。
func hashRequestBody(r *http.Request) (platform.Hash, error) {
	if r.Body == nil {
		return platform.HashBytes(nil), nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			return "", platform.NewError(platform.CodeBudgetExceeded, false, "请求体超过接口上限")
		}
		return "", platform.NewError(platform.CodeInternal, false, "读取请求正文失败")
	}
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	sum := sha256.Sum256(body)
	return platform.Hash(hex.EncodeToString(sum[:])), nil
}

func requestID(r *http.Request) platform.Identifier {
	if id := r.Header.Get(platform.HeaderRequestID); platform.ValidIdentifier(id) {
		return id
	}
	return platform.NewIdentifier()
}

func operationID(r *http.Request) platform.Identifier {
	if id := r.Header.Get(platform.HeaderOperationID); platform.ValidIdentifier(id) {
		return id
	}
	return platform.NewIdentifier()
}

func deadline(r *http.Request) (platform.Timestamp, error) {
	if raw := r.Header.Get(platform.HeaderDeadline); raw != "" {
		parsed, err := platform.ParseTimestamp(platform.Timestamp(raw))
		if err != nil {
			return "", platform.NewError(platform.CodeValidation, false, "请求截止不合法")
		}
		if !parsed.After(time.Now()) {
			return "", platform.NewError(platform.CodeDeadlineExceeded, false, "请求已超过截止")
		}
		return platform.Timestamp(parsed.Format(time.RFC3339Nano)), nil
	}
	return platform.Timestamp(time.Now().Add(defaultDeadline).UTC().Format(time.RFC3339Nano)), nil
}

// serviceActor 无用户委托时的服务主体；权限由调用方业务函数核实。
func serviceActor(caller string, until platform.Timestamp) platform.AuthContext {
	return platform.AuthContext{
		SubjectID:          caller,
		Kind:               "service",
		SiteID:             serviceSiteID,
		PermissionRevision: 0,
		Grants:             []string{},
		ExpiresAt:          until,
	}
}
