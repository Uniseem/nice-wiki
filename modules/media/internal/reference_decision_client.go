package media

import (
	"context"
	"encoding/json"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

// ReferenceSourceDecision 仅是公共唯一schema的本域解码投影；读取前严格验证原JSON。
type ReferenceSourceDecision struct {
	State             string       `json:"state"`
	SiteId            Identifier   `json:"siteId"`
	SourceOperationId Identifier   `json:"sourceOperationId"`
	SourceObjectId    Identifier   `json:"sourceObjectId"`
	SourceRevisionId  Identifier   `json:"sourceRevisionId"`
	SourceVersion     gen.Revision `json:"sourceVersion"`
	ReferenceDigest   Hash         `json:"referenceDigest"`
	DataEpoch         Identifier   `json:"dataEpoch"`
}

// InstallReferenceDecisionClients 只在构造期装配固定生成客户端。
func InstallReferenceDecisionClients(content *gen.ContentClient, config *gen.SiteConfigClient) error {
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok || content == nil || config == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "引用源决定客户端未完整装配")
	}
	s.contentDecisions = content
	s.configDecisions = config
	return nil
}

func (s *localBlobStore) readReferenceDecision(ctx context.Context, op OperationContext, material AcquireResourceReferencesInput) (ReferenceSourceDecision, error) {
	if s.contentDecisions == nil || s.configDecisions == nil || s.validateDecision == nil {
		return ReferenceSourceDecision{}, platform.NewError(platform.CodeDependencyUnavailable, true, "引用源决定读取未装配")
	}
	source := op.CallerService
	readOp := op
	readOp.CallerService = "media"
	readOp.Actor = platform.AuthContext{Kind: "service", SubjectID: "media", SiteID: material.SiteId, ExpiresAt: op.Deadline}
	readCtx := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), readOp)
	var raw json.RawMessage
	var err error
	if source == "content" {
		raw, err = s.contentDecisions.ReadReferenceDecision(readCtx, material.SourceOperationId)
	} else {
		raw, err = s.configDecisions.ReadHomeReferenceDecision(readCtx, material.SourceOperationId)
	}
	if err != nil {
		var failure *platform.Error
		if errors.As(err, &failure) && failure.Code == platform.CodeNotFound {
			return ReferenceSourceDecision{}, platform.NewError(platform.CodeDependencyUnavailable, true, "引用源决定尚不可确认")
		}
		return ReferenceSourceDecision{}, err
	}
	var decision ReferenceSourceDecision
	if s.validateDecision(raw) != nil || json.Unmarshal(raw, &decision) != nil {
		return decision, platform.NewError(platform.CodeIntegrityError, false, "引用源决定不符合公共schema")
	}
	if decision.DataEpoch != op.DataEpoch {
		return decision, platform.NewError(platform.CodeVersionMismatch, false, "引用源决定不属于当前活动代次")
	}
	if decision.SiteId != material.SiteId || decision.SourceOperationId != material.SourceOperationId || decision.SourceObjectId != material.SourceObjectId || decision.SourceRevisionId != material.SourceRevisionId || decision.SourceVersion != material.SourceVersion || decision.ReferenceDigest != material.ReferenceDigest {
		return decision, platform.NewError(platform.CodeIntegrityError, false, "引用源决定与完整预留材料不符")
	}
	return decision, nil
}
