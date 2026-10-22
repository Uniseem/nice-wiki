package content

import (
	"context"
	"database/sql"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type ReadPublishedRevisionInput struct {
	DocumentId          Identifier  `json:"documentId"`
	PublishedRevisionId *Identifier `json:"publishedRevisionId"`
}

type ReadPublishedRevisionResult = gen.ReadPublishedRevisionResult

// ReadPublishedRevision只返回当前仍公开的精确正文，不提供已撤回历史。
func ReadPublishedRevision(ctx context.Context, op OperationContext, input ReadPublishedRevisionInput) (ReadPublishedRevisionResult, error) {
	if op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService || !publishedRevisionCaller(op.CallerService) {
		return ReadPublishedRevisionResult{}, platform.NewError(platform.CodeNotFound, false, "公开正文不存在")
	}
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return ReadPublishedRevisionResult{}, publicationReadError(err)
	}
	if err = runtime.Reference.Store.checkRevisionScope(ctx, op); err != nil {
		return ReadPublishedRevisionResult{}, publicationReadError(err)
	}
	tx, err := runtime.Reference.Store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ReadPublishedRevisionResult{}, contentStorageError()
	}
	current, stored, err := loadVisiblePublication(ctx, tx, op, runtime, input.DocumentId)
	tx.Rollback()
	if err != nil {
		return ReadPublishedRevisionResult{}, err
	}
	if input.PublishedRevisionId != nil && *input.PublishedRevisionId != current.PublishedReference.PublishedRevisionId {
		return ReadPublishedRevisionResult{}, platform.NewError(platform.CodeNotFound, false, "请求的发布修订不是当前公开版本")
	}
	checked, err := ValidatePublications(ctx, op, ValidatePublicationsInput{Refs: []gen.PublishedRevisionRef{*current.PublishedReference}})
	if err != nil {
		return ReadPublishedRevisionResult{}, publicationReadError(err)
	}
	if len(checked.Validations) != 1 || !checked.Validations[0].Visible || !checked.Validations[0].MatchesCurrent {
		return ReadPublishedRevisionResult{}, platform.NewError(platform.CodeNotFound, false, "公开正文不存在")
	}
	files := runtime.Reference.Store.ConfigureRevisionStorage(RevisionStorageConfig{})
	body, err := ReadRevisionBody(WithRevisionFileStore(ctx, files), op, ReadRevisionBodyInput{DocumentID: input.DocumentId, RevisionID: stored.draftRevisionID, ExpectedHash: &current.PublishedReference.ContentHash})
	if err != nil {
		return ReadPublishedRevisionResult{}, err
	}
	confirm, err := runtime.Reference.Store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ReadPublishedRevisionResult{}, contentStorageError()
	}
	again, againStored, err := loadVisiblePublication(ctx, confirm, op, runtime, input.DocumentId)
	confirm.Rollback()
	if err != nil {
		var problem *platform.Error
		if errors.As(err, &problem) && problem.Code == platform.CodeNotFound {
			return ReadPublishedRevisionResult{}, platform.NewError(platform.CodeStaleResult, false, "公开正文在读取期间已变化")
		}
		return ReadPublishedRevisionResult{}, err
	}
	if again.PublishedReference.PublishedRevisionId != current.PublishedReference.PublishedRevisionId || again.VisibilityRevision != current.VisibilityRevision || again.PublishedReference.ContentHash != current.PublishedReference.ContentHash || againStored.draftRevisionID != stored.draftRevisionID {
		return ReadPublishedRevisionResult{}, platform.NewError(platform.CodeStaleResult, false, "公开正文在读取期间已变化")
	}
	return ReadPublishedRevisionResult{Metadata: again, Reference: *again.PublishedReference, Body: body.Body, VisibilityRevision: again.VisibilityRevision}, nil
}

func publishedRevisionCaller(service string) bool {
	switch service {
	case "public-site", "knowledge", "qa", "localization", "memory":
		return true
	default:
		return false
	}
}

func publicationReadError(err error) error {
	var problem *platform.Error
	if errors.As(err, &problem) {
		switch problem.Code {
		case platform.CodeNotFound, platform.CodeStaleResult, platform.CodeIntegrityError, platform.CodeDependencyUnavailable:
			return err
		case platform.CodeForbidden:
			return platform.NewError(platform.CodeNotFound, false, "公开正文不存在")
		}
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "公开正文权威不可用")
}

func loadVisiblePublication(ctx context.Context, tx *sql.Tx, op OperationContext, runtime TreeRuntime, documentID Identifier) (gen.DocumentMetadata, storedPublication, error) {
	documents, err := readDocuments(ctx, tx, op, runtime, []Identifier{documentID})
	if err != nil {
		return gen.DocumentMetadata{}, storedPublication{}, publicationReadError(err)
	}
	if document, found := documents[documentID]; found {
		if document.ShareImage != nil {
			return gen.DocumentMetadata{}, storedPublication{}, platform.NewError(platform.CodeDependencyUnavailable, true, "分享图发布权威尚未装配")
		}
		if document.State != gen.DocumentMetadataStatePublished || document.PublishedReference == nil || document.PublishedReference.DocumentId != document.DocumentId {
			return gen.DocumentMetadata{}, storedPublication{}, platform.NewError(platform.CodeNotFound, false, "公开正文不存在")
		}
		stored, ok, err := readPublicationSnapshot(ctx, tx, op, document.PublishedReference.PublishedRevisionId)
		if err != nil {
			return gen.DocumentMetadata{}, storedPublication{}, err
		}
		if !ok || stored.documentID != document.DocumentId || stored.contentHash != document.PublishedReference.ContentHash || stored.schemaVersion != document.PublishedReference.SchemaVersion || stored.revision != document.PublishedReference.Revision || stored.visibility != document.VisibilityRevision || stored.dataEpoch != document.PublishedReference.DataEpoch {
			return gen.DocumentMetadata{}, storedPublication{}, platform.NewError(platform.CodeDependencyUnavailable, true, "当前发布快照与元数据不一致")
		}
		return document, stored, nil
	}
	return gen.DocumentMetadata{}, storedPublication{}, platform.NewError(platform.CodeNotFound, false, "公开正文不存在")
}
