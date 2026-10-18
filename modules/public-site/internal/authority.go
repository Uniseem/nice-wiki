package publicsite

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/public-site/internal/gen"
)

type CheckPublicationInput struct {
	DocumentID       string
	Locale           string
	ExpectedRevision *int
	DataEpoch        string
}
type CheckPublicationResult struct {
	Published          bool
	Publication        gen.PublishedRevisionRef
	CanonicalPath      string
	VisibilityRevision int
}

// CheckPublication 每次请求都询问content，不把缓存或自建token视为公开许可。
func CheckPublication(ctx context.Context, op platform.OperationContext, in CheckPublicationInput) (CheckPublicationResult, error) {
	if in.DataEpoch != string(op.DataEpoch) {
		return CheckPublicationResult{}, platform.NewError(platform.CodeVersionMismatch, false, "EPOCH_MISMATCH")
	}
	directory, err := directoryFrom(ctx)
	if err != nil {
		return CheckPublicationResult{}, err
	}
	tree, err := readTree(ctx, directory.Content)
	if err != nil {
		return CheckPublicationResult{}, err
	}
	node, found, err := findDocument(tree, in.DocumentID)
	if err != nil {
		return CheckPublicationResult{}, err
	}
	if !found {
		return CheckPublicationResult{}, platform.NewError(platform.CodeNotFound, false, "NOT_PUBLISHED")
	}
	if string(node.reference.DataEpoch) != in.DataEpoch {
		return CheckPublicationResult{}, platform.NewError(platform.CodeVersionMismatch, false, "EPOCH_MISMATCH")
	}
	if in.ExpectedRevision != nil && node.reference.Revision != *in.ExpectedRevision {
		return CheckPublicationResult{}, stale()
	}
	raw, err := directory.Content.ValidatePublications(ctx, map[string]any{"refs": []gen.PublishedRevisionRef{node.reference}})
	if err != nil {
		return CheckPublicationResult{}, err
	}
	if err = confirmValidation(raw, node, op); err == errWithdrawn {
		return CheckPublicationResult{}, platform.NewError(platform.CodeNotFound, false, "NOT_PUBLISHED")
	} else if err != nil {
		return CheckPublicationResult{}, err
	}
	return CheckPublicationResult{true, node.reference, node.canonicalPath, node.visibilityRevision}, nil
}
