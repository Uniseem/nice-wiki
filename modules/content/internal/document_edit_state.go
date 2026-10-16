package content

import (
	"context"
	"database/sql"
	"errors"

	"nice-wiki/libs/platform"
)

// DocumentEditState只来自私有权威行，绝不根据待保存请求猜建文档。
type DocumentEditState struct {
	SiteID           Identifier
	DocumentID       Identifier
	State            string
	Revision         int64
	DeleteFence      int64
	ReferenceVersion int64
	Quarantined      bool
}

// ReadDocumentEditState不持SQL事务跨后续IO；调用方持同doc mutex至短事务结束。
func (s *Store) ReadDocumentEditState(ctx context.Context, op OperationContext, documentID Identifier) (DocumentEditState, error) {
	if err := s.checkRevisionScope(ctx, op); err != nil {
		return DocumentEditState{}, err
	}
	state := DocumentEditState{SiteID: op.Actor.SiteID, DocumentID: documentID}
	err := s.db.QueryRowContext(ctx, `SELECT d.state,d.revision,d.delete_fence,d.reference_version,EXISTS(SELECT 1 FROM document_quarantines q WHERE q.site_id=d.site_id AND q.document_id=d.document_id) FROM documents d WHERE d.site_id=? AND d.document_id=?`, op.Actor.SiteID, documentID).Scan(&state.State, &state.Revision, &state.DeleteFence, &state.ReferenceVersion, &state.Quarantined)
	if errors.Is(err, sql.ErrNoRows) {
		return DocumentEditState{}, platform.NewError(platform.CodeNotFound, false, "文档不存在")
	}
	if err != nil {
		return DocumentEditState{}, contentStorageError()
	}
	return state, nil
}
