package content

import (
	"context"
	"database/sql"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type ValidatePublicationsInput = gen.ValidatePublicationsInput
type ValidatePublicationsResult = gen.ValidatePublicationsResult

type storedPublication struct {
	documentID      Identifier
	draftRevisionID Identifier
	contentHash     Hash
	schemaVersion   int
	revision        int
	visibility      int
	dataEpoch       Identifier
}

// ValidatePublications在一个只读快照里判断精确引用是否仍是当前公开版本。
func ValidatePublications(ctx context.Context, op OperationContext, input ValidatePublicationsInput) (ValidatePublicationsResult, error) {
	if op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService || !publicationValidationCaller(op.CallerService) {
		return ValidatePublicationsResult{}, platform.NewError(platform.CodeForbidden, false, "发布可见性仅允许已登记的纯服务读取")
	}
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return ValidatePublicationsResult{}, publicationDependency(err)
	}
	if err = runtime.Reference.Store.checkRevisionScope(ctx, op); err != nil {
		return ValidatePublicationsResult{}, publicationDependency(err)
	}
	tx, err := runtime.Reference.Store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ValidatePublicationsResult{}, contentStorageError()
	}
	defer tx.Rollback()
	ids := make([]Identifier, 0, len(input.Refs))
	for _, ref := range input.Refs {
		ids = append(ids, ref.DocumentId)
	}
	index, err := readDocuments(ctx, tx, op, runtime, ids)
	if err != nil {
		return ValidatePublicationsResult{}, publicationDependency(err)
	}
	validations := make([]gen.PublicationValidation, 0, len(input.Refs))
	for _, ref := range input.Refs {
		document, found := index[ref.DocumentId]
		item, err := validatePublicationRef(ctx, tx, op, found, document, ref)
		if err != nil {
			return ValidatePublicationsResult{}, err
		}
		validations = append(validations, item)
	}
	return ValidatePublicationsResult{Validations: validations}, nil
}

func publicationValidationCaller(service string) bool {
	switch service {
	case "public-site", "knowledge", "qa", "localization", "site-config", "memory":
		return true
	default:
		return false
	}
}

func publicationDependency(err error) error {
	var problem *platform.Error
	if errors.As(err, &problem) && problem.Code == platform.CodeForbidden {
		return err
	}
	if errors.As(err, &problem) && problem.Code == platform.CodeDependencyUnavailable {
		return err
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "发布可见性权威不可用")
}

func validatePublicationRef(ctx context.Context, tx *sql.Tx, op OperationContext, found bool, document gen.DocumentMetadata, ref gen.PublishedRevisionRef) (gen.PublicationValidation, error) {
	result := gen.PublicationValidation{Reference: ref, Visible: false, MatchesCurrent: false}
	if !found {
		return result, nil
	}
	result.VisibilityRevision = document.VisibilityRevision
	if document.ShareImage != nil {
		return result, platform.NewError(platform.CodeDependencyUnavailable, true, "分享图发布权威尚未装配")
	}
	if document.State != gen.DocumentMetadataStatePublished || document.PublishedReference == nil {
		return result, nil
	}
	current := *document.PublishedReference
	if current.DocumentId != document.DocumentId || current.DataEpoch != op.DataEpoch {
		return result, platform.NewError(platform.CodeDependencyUnavailable, true, "当前发布引用与打开代次不一致")
	}
	stored, ok, err := readPublicationSnapshot(ctx, tx, op, current.PublishedRevisionId)
	if err != nil {
		return result, err
	}
	if !ok || stored.documentID != current.DocumentId || stored.draftRevisionID == "" || stored.contentHash != current.ContentHash || stored.schemaVersion != current.SchemaVersion || stored.revision != current.Revision || stored.visibility != document.VisibilityRevision || stored.dataEpoch != current.DataEpoch {
		return result, platform.NewError(platform.CodeDependencyUnavailable, true, "当前发布快照与元数据不一致")
	}
	matches := current.PublishedRevisionId == ref.PublishedRevisionId && current.Revision == ref.Revision && current.ContentHash == ref.ContentHash && current.SchemaVersion == ref.SchemaVersion && current.DataEpoch == ref.DataEpoch && current.DocumentId == ref.DocumentId
	result.Visible = matches
	result.MatchesCurrent = matches
	result.CurrentRevision = current.Revision
	return result, nil
}

func readPublicationSnapshot(ctx context.Context, tx *sql.Tx, op OperationContext, publishedRevisionID Identifier) (storedPublication, bool, error) {
	var stored storedPublication
	err := tx.QueryRowContext(ctx, `SELECT document_id,draft_revision_id,content_hash,schema_version,revision,visibility_revision,data_epoch FROM publication_snapshots WHERE site_id=? AND published_revision_id=?`, op.Actor.SiteID, publishedRevisionID).Scan(&stored.documentID, &stored.draftRevisionID, &stored.contentHash, &stored.schemaVersion, &stored.revision, &stored.visibility, &stored.dataEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return storedPublication{}, false, nil
	}
	if err != nil {
		return storedPublication{}, false, contentStorageError()
	}
	return stored, true, nil
}
