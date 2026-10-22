package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/content/internal/gen"
)

type TrashDocumentInput struct {
	DocumentId            Identifier  `json:"documentId"`
	IncludeSubtree        bool        `json:"includeSubtree"`
	ReplacementDocumentId *Identifier `json:"replacementDocumentId"`
	ExpectedRevision      int         `json:"expectedRevision"`
	ExpectedTreeRevision  int         `json:"expectedTreeRevision"`
}
type TrashDocumentResult = gen.TrashDocumentResult

type RestoreDocumentInput struct {
	DocumentId       Identifier  `json:"documentId"`
	ParentId         *Identifier `json:"parentId"`
	PathSegment      *string     `json:"pathSegment"`
	ExpectedRevision int         `json:"expectedRevision"`
}
type RestoreDocumentResult = gen.RestoreDocumentResult

type PurgeDocumentsInput struct {
	DocumentIds           []Identifier `json:"documentIds"`
	ExpectedTrashRevision int          `json:"expectedTrashRevision"`
	ConfirmationHash      Hash         `json:"confirmationHash"`
}
type PurgeDocumentsResult = gen.PurgeDocumentsResult

// writeTreeTx 取写窗口、站点锁并开事务，读取同快照的文档树；调用方负责 Commit。
func writeTreeTx(ctx context.Context, op OperationContext, runtime TreeRuntime) (work context.Context, tx *sql.Tx, version int64, documents []gen.DocumentMetadata, done func(), err error) {
	if runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return nil, nil, 0, nil, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "当前写窗口尚未装配")
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return nil, nil, 0, nil, nil, err
	}
	if view.DataEpoch != op.DataEpoch {
		cancel()
		return nil, nil, 0, nil, nil, platform.NewError(platform.CodeVersionMismatch, false, "写窗口代次已变化")
	}
	store := runtime.Reference.Store
	store.mu.Lock()
	tx, err = store.db.BeginTx(work, nil)
	if err != nil {
		store.mu.Unlock()
		cancel()
		return nil, nil, 0, nil, nil, contentStorageError()
	}
	done = func() { _ = tx.Rollback(); store.mu.Unlock(); cancel() }
	version, documents, err = readTreeSnapshot(work, tx, op, runtime)
	if err != nil {
		done()
		return nil, nil, 0, nil, nil, err
	}
	return work, tx, version, documents, done, nil
}

func isActive(d gen.DocumentMetadata) bool {
	return d.State == gen.DocumentMetadataStateDraft || d.State == gen.DocumentMetadataStatePublished
}

// TrashDocument 把文档（可含子树）移入回收站；已发布的先撤回公开，并按选择留下 410 或跳转。
func TrashDocument(ctx context.Context, op OperationContext, input TrashDocumentInput) (TrashDocumentResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return TrashDocumentResult{}, err
	}
	if _, err = authorizePublication(ctx, op, runtime, "trashDocument", input.DocumentId); err != nil {
		return TrashDocumentResult{}, err
	}
	work, tx, version, documents, done, err := writeTreeTx(ctx, op, runtime)
	if err != nil {
		return TrashDocumentResult{}, err
	}
	defer done()
	if version != int64(input.ExpectedTreeRevision) {
		return TrashDocumentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "文档树版本已变化")
	}
	byID := map[Identifier]gen.DocumentMetadata{}
	children := map[Identifier][]Identifier{}
	for _, d := range documents {
		byID[d.DocumentId] = d
		if d.ParentId != nil && isActive(d) {
			children[*d.ParentId] = append(children[*d.ParentId], d.DocumentId)
		}
	}
	root, found := byID[input.DocumentId]
	if !found {
		return TrashDocumentResult{}, platform.NewError(platform.CodeNotFound, false, "文档不存在")
	}
	if !isActive(root) {
		return TrashDocumentResult{}, platform.NewError(platform.CodeLocked, false, "文档已在回收站")
	}
	if int(root.Revision) != input.ExpectedRevision {
		return TrashDocumentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "文档修订已变化")
	}
	if len(children[root.DocumentId]) > 0 && !input.IncludeSubtree {
		return TrashDocumentResult{}, platform.NewError(platform.CodeConflict, false, "该文档含子文档，请选择同时删除子树")
	}
	if input.ReplacementDocumentId != nil {
		replacement, ok := byID[*input.ReplacementDocumentId]
		if !ok || !isActive(replacement) || replacement.DocumentId == root.DocumentId {
			return TrashDocumentResult{}, platform.NewError(platform.CodeValidation, false, "替代文档必须是另一篇活动文档")
		}
	}
	var order []Identifier
	var collect func(Identifier)
	collect = func(id Identifier) {
		order = append(order, id)
		for _, c := range children[id] {
			collect(c)
		}
	}
	collect(root.DocumentId)
	if input.ReplacementDocumentId != nil && slices.Contains(order, *input.ReplacementDocumentId) {
		return TrashDocumentResult{}, platform.NewError(platform.CodeValidation, false, "替代文档不能在被删除的子树内")
	}
	paths, err := canonicalDocumentPaths(documents)
	if err != nil {
		return TrashDocumentResult{}, err
	}
	pathOf := map[Identifier]string{}
	for p, id := range paths {
		pathOf[id] = p
	}
	now := platform.Now()
	next := slices.Clone(documents)
	var tombstones []gen.RouteTombstone
	bound := bindPublicationTx(work, tx)
	revision := version
	for _, id := range order {
		current := byID[id]
		metadata := current
		wasPublished := current.State == gen.DocumentMetadataStatePublished && current.PublishedReference != nil
		metadata.State = gen.DocumentMetadataStateTrash
		trashedAt, _ := time.Parse(time.RFC3339Nano, string(now))
		metadata.TrashedAt = &trashedAt
		metadata.Revision++
		if wasPublished {
			metadata.PublishedReference = nil
			metadata.VisibilityRevision++
		}
		if err = casPublicationIdentity(work, tx, op, revision, metadata, current.Revision, string(current.State)); err != nil {
			return TrashDocumentResult{}, err
		}
		revision++
		for i := range next {
			if next[i].DocumentId == id {
				next[i] = metadata
			}
		}
		if wasPublished {
			if err = archivePublishedReferences(work, tx, op, id); err != nil {
				return TrashDocumentResult{}, err
			}
			refs, sourceVersion, listErr := listPublicationReferences(work, tx, op, id)
			if listErr != nil {
				return TrashDocumentResult{}, listErr
			}
			withdrawn := publicationWithdrawnPayload{SiteID: op.Actor.SiteID, DocumentID: id, Revision: metadata.Revision, VisibilityRevision: metadata.VisibilityRevision, DataEpoch: op.DataEpoch, Reason: "trashed", OccurredAt: now}
			if err = enqueuePublication(bound, op, metadata.VisibilityRevision, nil, &withdrawn, id, sourceVersion, refs); err != nil {
				return TrashDocumentResult{}, err
			}
			if path, ok := pathOf[id]; ok {
				tomb := gen.RouteTombstone{OldPath: path, DocumentId: id, Status: gen.RouteTombstoneStatus("gone"), Revision: int(revision)}
				if input.ReplacementDocumentId != nil {
					tomb.Status = gen.RouteTombstoneStatus("redirect")
					tomb.TargetDocumentId = input.ReplacementDocumentId
				}
				tombstones = append(tombstones, tomb)
			}
		}
	}
	stored, err := commitRedirectManifest(work, tx, op, tombstones, next)
	if err != nil {
		return TrashDocumentResult{}, err
	}
	if err = enqueueTreeChanged(bound, tx, op, revision, order); err != nil {
		return TrashDocumentResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return TrashDocumentResult{}, contentStorageError()
	}
	if stored == nil {
		stored = []gen.RouteTombstone{}
	}
	return TrashDocumentResult{TrashedDocumentIds: order, Tombstones: stored}, nil
}

// RestoreDocument 把回收站里的文档恢复为草稿，放到指定父节点下并重新指定路径。
func RestoreDocument(ctx context.Context, op OperationContext, input RestoreDocumentInput) (RestoreDocumentResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return RestoreDocumentResult{}, err
	}
	if _, err = authorizePublication(ctx, op, runtime, "restoreDocument", input.DocumentId); err != nil {
		return RestoreDocumentResult{}, err
	}
	work, tx, version, documents, done, err := writeTreeTx(ctx, op, runtime)
	if err != nil {
		return RestoreDocumentResult{}, err
	}
	defer done()
	var current *gen.DocumentMetadata
	for i := range documents {
		if documents[i].DocumentId == input.DocumentId {
			current = &documents[i]
		}
	}
	if current == nil {
		return RestoreDocumentResult{}, platform.NewError(platform.CodeNotFound, false, "文档不存在")
	}
	if current.State != gen.DocumentMetadataStateTrash {
		return RestoreDocumentResult{}, platform.NewError(platform.CodeLocked, false, "只能恢复回收站里的文档")
	}
	if int(current.Revision) != input.ExpectedRevision {
		return RestoreDocumentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "文档修订已变化")
	}
	if input.ParentId != nil {
		parentOK := false
		for _, d := range documents {
			if d.DocumentId == *input.ParentId && isActive(d) {
				parentOK = true
			}
		}
		if !parentOK {
			return RestoreDocumentResult{}, platform.NewError(platform.CodeLocked, false, "目标父文档不是活动文档")
		}
	}
	metadata := *current
	metadata.State = gen.DocumentMetadataStateDraft
	metadata.TrashedAt = nil
	metadata.ParentId = input.ParentId
	// 不填新路径时沿用删除前的路径；从未设过路径的草稿恢复后仍没有路径，首次发布时再定。
	if input.PathSegment != nil {
		metadata.PathSegment = input.PathSegment
	}
	metadata.Revision++
	siblings := []gen.DocumentMetadata{}
	for _, d := range documents {
		if d.DocumentId != metadata.DocumentId && isActive(d) && sameIdentifierPointer(d.ParentId, input.ParentId) {
			siblings = append(siblings, d)
		}
	}
	sort.Slice(siblings, func(i, j int) bool { return siblings[i].OrderKey < siblings[j].OrderKey })
	lower := ""
	if len(siblings) > 0 {
		lower = siblings[len(siblings)-1].OrderKey
	}
	if metadata.OrderKey, err = fractionalDocumentOrder(lower, ""); err != nil {
		return RestoreDocumentResult{}, err
	}
	candidate := slices.Clone(documents)
	for i := range candidate {
		if candidate[i].DocumentId == metadata.DocumentId {
			candidate[i] = metadata
		}
	}
	if metadata.PathSegment != nil {
		if _, err = validatePathSnapshot(ValidatePathSegmentInput{Segment: *metadata.PathSegment, ParentId: metadata.ParentId, DocumentId: metadata.DocumentId, Documents: candidate}); err != nil {
			return RestoreDocumentResult{}, err
		}
	}
	if err = casPublicationIdentity(work, tx, op, version, metadata, current.Revision, string(current.State)); err != nil {
		return RestoreDocumentResult{}, err
	}
	if err = enqueueTreeChanged(bindPublicationTx(work, tx), tx, op, version+1, []Identifier{metadata.DocumentId}); err != nil {
		return RestoreDocumentResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return RestoreDocumentResult{}, contentStorageError()
	}
	return RestoreDocumentResult{Metadata: metadata, MissingAssets: []Identifier{}}, nil
}

// PurgeDocuments 只有超管能彻底删除回收站文档：元数据留下 purged 墓碑，正文文件由后续清理回收。
func PurgeDocuments(ctx context.Context, op OperationContext, input PurgeDocumentsInput) (PurgeDocumentsResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return PurgeDocumentsResult{}, err
	}
	if op.CallerService != "portal" || op.Actor.Kind != "admin" || !slices.Contains(op.Actor.Grants, "superadmin") {
		return PurgeDocumentsResult{}, platform.NewError(platform.CodeForbidden, false, "只有超级管理员能彻底删除")
	}
	if _, err = authorizePublication(ctx, op, runtime, "purgeDocuments", ""); err != nil {
		return PurgeDocumentsResult{}, err
	}
	ids := slices.Clone(input.DocumentIds)
	sort.Strings(ids)
	sum := platform.HashBytes([]byte(strings.Join(ids, ",")))
	if input.ConfirmationHash != sum {
		return PurgeDocumentsResult{}, platform.NewError(platform.CodeValidation, false, "确认摘要与选中的文档集合不一致")
	}
	work, tx, version, documents, done, err := writeTreeTx(ctx, op, runtime)
	if err != nil {
		return PurgeDocumentsResult{}, err
	}
	defer done()
	if version != int64(input.ExpectedTrashRevision) {
		return PurgeDocumentsResult{}, platform.NewError(platform.CodeVersionMismatch, false, "回收站版本已变化")
	}
	byID := map[Identifier]gen.DocumentMetadata{}
	for _, d := range documents {
		byID[d.DocumentId] = d
	}
	revision := version
	for _, id := range ids {
		current, ok := byID[id]
		if !ok || current.State != gen.DocumentMetadataStateTrash {
			return PurgeDocumentsResult{}, platform.NewError(platform.CodeLocked, false, "只能彻底删除回收站里的文档")
		}
		metadata := current
		metadata.State = gen.DocumentMetadataStatePurged
		metadata.Revision++
		if err = casPublicationIdentity(work, tx, op, revision, metadata, current.Revision, string(current.State)); err != nil {
			return PurgeDocumentsResult{}, err
		}
		revision++
	}
	if err = releasePurgedReferences(work, tx, op, ids); err != nil {
		return PurgeDocumentsResult{}, err
	}
	if err = enqueueTreeChanged(bindPublicationTx(work, tx), tx, op, revision, ids); err != nil {
		return PurgeDocumentsResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return PurgeDocumentsResult{}, contentStorageError()
	}
	return PurgeDocumentsResult{PurgedIds: ids}, nil
}

// releasePurgedReferences 与彻底删除同事务：彻底删除的文档不再有任何版本可恢复，按保存草稿同样的引用事件
// 以空集合推进资源服务里该文档的引用版本，资源服务据此把旧引用转为历史，资源可以进入清理计划。
// 来源操作只由文档派生，补发同一文档时资源服务按同一来源去重。
func releasePurgedReferences(ctx context.Context, tx *sql.Tx, op OperationContext, ids []Identifier) error {
	empty, digest, err := draftMediaReferences(nil)
	if err != nil {
		return err
	}
	for _, id := range ids {
		var version int
		var revision Identifier
		if err = tx.QueryRowContext(ctx, `SELECT d.reference_version,h.revision_id FROM documents d JOIN draft_heads h ON h.site_id=d.site_id AND h.document_id=d.document_id WHERE d.site_id=? AND d.document_id=?`, op.Actor.SiteID, id).Scan(&version, &revision); err != nil {
			return contentStorageError()
		}
		payload, err := json.Marshal(struct {
			SourceService     string                `json:"sourceService"`
			SourceOperationID Identifier            `json:"sourceOperationId"`
			SiteID            Identifier            `json:"siteId"`
			SourceObjectID    Identifier            `json:"sourceObjectId"`
			SourceRevisionID  Identifier            `json:"sourceRevisionId"`
			SourceVersion     int                   `json:"sourceVersion"`
			ReferenceDigest   Hash                  `json:"referenceDigest"`
			References        []draftMediaReference `json:"references"`
		}{"content", platform.HashBytes([]byte("purge:" + id)), op.Actor.SiteID, id, revision, version + 1, digest, empty})
		if err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "彻底删除的资源事件载荷无法编码")
		}
		if err = outbox.EnqueueEvent(tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: "content.resources.changed", EventVersion: 2, Producer: "content", SubjectID: id, Revision: int64(version + 1), OccurredAt: platform.Now(), DataEpoch: op.DataEpoch, Payload: payload}); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO purged_reference_releases(site_id,document_id) VALUES(?,?)`, op.Actor.SiteID, id); err != nil {
			return contentStorageError()
		}
	}
	// 引用列表随之变化，推进扫描水位，进行中的分页读取会从头再读。
	if _, err = tx.ExecContext(ctx, `UPDATE reference_scan_state SET revision=revision+1 WHERE singleton=1`); err != nil {
		return contentStorageError()
	}
	return nil
}

// releasePurgedBacklog 为本修复之前彻底删除、还没发过释放事件的文档补发；没有遗留时只是一次空查询。
func releasePurgedBacklog(ctx context.Context, op OperationContext, runtime RelayRuntime) error {
	const backlog = `SELECT d.document_id FROM documents d WHERE d.site_id=? AND d.state='purged' AND NOT EXISTS (SELECT 1 FROM purged_reference_releases r WHERE r.site_id=d.site_id AND r.document_id=d.document_id) ORDER BY d.document_id LIMIT 100`
	ids, err := queryIdentifiers(ctx, runtime.Store.db, backlog, op.Actor.SiteID)
	if err != nil || len(ids) == 0 {
		return err
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return err
	}
	defer cancel()
	if view.DataEpoch != op.DataEpoch {
		return nil
	}
	tx, err := runtime.Store.db.BeginTx(work, nil)
	if err != nil {
		return contentStorageError()
	}
	defer tx.Rollback()
	if ids, err = queryIdentifiers(work, tx, backlog, op.Actor.SiteID); err != nil || len(ids) == 0 {
		return err
	}
	if err = releasePurgedReferences(work, tx, op, ids); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return contentStorageError()
	}
	return nil
}

func queryIdentifiers(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, query string, args ...any) ([]Identifier, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, contentStorageError()
	}
	defer rows.Close()
	ids := []Identifier{}
	for rows.Next() {
		var id Identifier
		if err = rows.Scan(&id); err != nil {
			return nil, contentStorageError()
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, contentStorageError()
	}
	return ids, nil
}
