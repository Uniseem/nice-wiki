package index

import (
 "context"
 "encoding/json"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/gen"
)

func ValidatePublication(ctx context.Context, op OperationContext, client *gen.ContentClient, validate func(json.RawMessage) error, ref gen.PublishedRevisionRef) (time.Time, int, error) {
	raw, err := client.ValidatePublications(platform.WithOperationContext(ctx, op), struct {
		Refs []gen.PublishedRevisionRef `json:"refs"`
	}{[]gen.PublishedRevisionRef{ref}})
	if err != nil {
		return time.Time{}, 0, err
	}
	observed := time.Now()
	var result struct {
		Validations []struct {
			Reference          gen.PublishedRevisionRef `json:"reference"`
			Visible            bool                     `json:"visible"`
			MatchesCurrent     bool                     `json:"matchesCurrent"`
			CurrentRevision    int                      `json:"currentRevision"`
			VisibilityRevision int                      `json:"visibilityRevision"`
		} `json:"validations"`
	}
	if validate(raw) != nil || json.Unmarshal(raw, &result) != nil || len(result.Validations) != 1 {
		return time.Time{}, 0, platform.NewError(platform.CodeDependencyUnavailable, false, "发布校验回执不完整")
	}
	proof := result.Validations[0]
	if proof.Reference != ref || proof.VisibilityRevision < 0 {
		return time.Time{}, 0, platform.NewError(platform.CodeDependencyUnavailable, false, "发布校验回执身份不一致")
	}
	if !proof.Visible || !proof.MatchesCurrent || proof.CurrentRevision != ref.Revision || ref.DataEpoch != op.DataEpoch {
		return time.Time{}, 0, platform.NewError(platform.CodeStaleResult, false, "发布修订已撤回或替代")
	}
	return observed, proof.VisibilityRevision, nil
}
