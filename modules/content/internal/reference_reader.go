package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/content/internal/gen"
)

const referenceScanBudget = 20000

type ReferencePageRequest struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}
type ListResourceReferencesInput struct {
	DocumentId *Identifier          `json:"documentId"`
	AssetId    *Identifier          `json:"assetId"`
	Page       ReferencePageRequest `json:"page"`
}
type ListResourceReferencesResult = gen.ListResourceReferencesResult

// ReferenceRuntime只装配只读来源依赖，不要求内存编辑锁或开放业务写入。
type ReferenceRuntime struct {
	Store             *Store
	Identity          *gen.IdentityClient
	SigningKey        platform.SigningKey
	validateReference func(json.RawMessage) error
	validateMetadata  func(json.RawMessage) error
}
type referenceRuntimeKey struct{}

func NewReferenceRuntime(store *Store, identity *gen.IdentityClient, key platform.SigningKey) (ReferenceRuntime, error) {
	if store == nil {
		return ReferenceRuntime{}, platform.NewError(platform.CodeDependencyUnavailable, true, "引用来源私有存储未装配")
	}
	spec, err := gen.GetSwagger()
	if err != nil {
		return ReferenceRuntime{}, platform.NewError(platform.CodeIntegrityError, false, "引用来源规格不可读")
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return ReferenceRuntime{}, platform.NewError(platform.CodeIntegrityError, false, "引用来源规格无法编码")
	}
	validate, err := apivalidate.NewPayloadValidator(raw, "ResourceReference")
	if err != nil {
		return ReferenceRuntime{}, err
	}
	validateMetadata, err := apivalidate.NewPayloadValidator(raw, "DocumentMetadata")
	if err != nil {
		return ReferenceRuntime{}, err
	}
	return ReferenceRuntime{Store: store, Identity: identity, SigningKey: key, validateReference: validate, validateMetadata: validateMetadata}, nil
}
func WithReferenceRuntime(ctx context.Context, runtime ReferenceRuntime) context.Context {
	return context.WithValue(ctx, referenceRuntimeKey{}, runtime)
}
func referenceRuntimeFromContext(ctx context.Context) (ReferenceRuntime, error) {
	runtime, ok := ctx.Value(referenceRuntimeKey{}).(ReferenceRuntime)
	if !ok || runtime.Store == nil || runtime.validateReference == nil || runtime.validateMetadata == nil {
		return runtime, platform.NewError(platform.CodeDependencyUnavailable, true, "引用来源运行依赖未装配")
	}
	return runtime, nil
}

type referenceCursor struct {
	Watermark int64 `json:"watermark"`
	Offset    int   `json:"offset"`
}

// ListResourceReferences每页均读取完整一致快照；水位变化必须从头，不能拼接不同快照。
func ListResourceReferences(ctx context.Context, op OperationContext, input ListResourceReferencesInput) (ListResourceReferencesResult, error) {
	runtime, err := referenceRuntimeFromContext(ctx)
	if input.Page.Limit == 0 {
		input.Page.Limit = 20
	}
	if err != nil {
		return ListResourceReferencesResult{}, err
	}
	if op.Actor.Kind == "service" {
		if op.Actor.SubjectID != op.CallerService || !slices.Contains([]string{"media", "site-config"}, op.CallerService) {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeForbidden, false, "此服务不能读取文档资源来源")
		}
	} else {
		if op.CallerService != "portal" || runtime.Identity == nil {
			return ListResourceReferencesResult{}, platform.NewError(platform.CodeForbidden, false, "文档资源来源要求当前后台授权")
		}
		resource := op.Actor.SiteID
		if input.DocumentId != nil {
			resource = *input.DocumentId
		}
		if _, err = authorizeEditLock(ctx, op, runtime.Identity, "content.read", "listResourceReferences", resource); err != nil {
			return ListResourceReferencesResult{}, err
		}
	}
	position, scope, err := readReferenceCursor(runtime, op, input)
	if err != nil {
		return ListResourceReferencesResult{}, err
	}
	if err = runtime.Store.checkRevisionScope(ctx, op); err != nil {
		return ListResourceReferencesResult{}, err
	}
	tx, err := runtime.Store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ListResourceReferencesResult{}, contentStorageError()
	}
	defer tx.Rollback()
	var watermark int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM reference_scan_state WHERE singleton=1`).Scan(&watermark); err != nil {
		return ListResourceReferencesResult{}, contentStorageError()
	}
	if input.Page.Cursor != "" && position.Watermark != watermark {
		return ListResourceReferencesResult{}, platform.NewError(platform.CodeVersionMismatch, true, "引用来源水位已变化，请从第一页重新读取")
	}
	if err = validateReferenceCoverage(ctx, tx, op, runtime); err != nil {
		return ListResourceReferencesResult{}, err
	}
	refs, err := readReferenceSources(ctx, tx, op, runtime)
	if err != nil {
		return ListResourceReferencesResult{}, err
	}
	_ = tx.Rollback() // 固定完整投影已读入内存，签名私钥IO不持SQL。
	selected := make([]gen.ResourceReference, 0)
	for _, ref := range refs {
		if input.DocumentId != nil && ref.DocumentId != *input.DocumentId {
			continue
		}
		if input.AssetId != nil && ref.Asset.AssetId != *input.AssetId {
			continue
		}
		selected = append(selected, ref)
	}
	slices.SortFunc(selected, func(a, b gen.ResourceReference) int { return strings.Compare(referenceSortKey(a), referenceSortKey(b)) })
	if position.Offset > len(selected) {
		return ListResourceReferencesResult{}, platform.NewError(platform.CodeValidation, false, "引用游标位置不可用")
	}
	end := min(position.Offset+input.Page.Limit, len(selected))
	result := ListResourceReferencesResult{References: selected[position.Offset:end], Watermark: int(watermark), PageInfo: gen.PageInfo{HasMore: end < len(selected)}}
	if result.PageInfo.HasMore {
		raw, _ := json.Marshal(referenceCursor{Watermark: watermark, Offset: end})
		cursor, err := serviceauth.SignCursor(runtime.SigningKey, scope, raw)
		if err != nil {
			return ListResourceReferencesResult{}, err
		}
		result.PageInfo.NextCursor = &cursor
	}
	return result, nil
}

func readReferenceCursor(runtime ReferenceRuntime, op OperationContext, input ListResourceReferencesInput) (referenceCursor, string, error) {
	binding, _ := platform.CanonicalJSON(struct {
		Caller, Kind, Actor, Site, Epoch, Session string
		SessionRevision, PermissionRevision       int64
		Document, Asset                           *Identifier
		Limit                                     int
	}{op.CallerService, op.Actor.Kind, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.Actor.SessionID, op.Actor.SessionRevision, op.Actor.PermissionRevision, input.DocumentId, input.AssetId, input.Page.Limit})
	scope := "content.references.v1:" + platform.HashBytes(binding)
	position := referenceCursor{}
	if input.Page.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(runtime.SigningKey, scope, input.Page.Cursor)
		if err != nil {
			return position, "", err
		}
		if decodeStorageJSON(raw, &position) != nil || position.Watermark < 1 || position.Offset < 1 || position.Offset > referenceScanBudget {
			return position, "", platform.NewError(platform.CodeValidation, false, "引用分页游标载荷不可用")
		}
	}
	return position, scope, nil
}

// validateReferenceCoverage拒绝缺索引、缺资格或尚无生产者的已发布/分享图来源，不能把未知当空。
func validateReferenceCoverage(ctx context.Context, tx *sql.Tx, op OperationContext, runtime ReferenceRuntime) error {
	var epoch string
	var site sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch,site_id FROM store_identity WHERE singleton=1`).Scan(&epoch, &site); err != nil {
		return contentStorageError()
	}
	if epoch != op.DataEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "引用来源数据代次已变化")
	}
	if site.Valid && site.String != op.Actor.SiteID {
		return platform.NewError(platform.CodeForbidden, false, "引用来源站点不匹配")
	}
	rows, err := tx.QueryContext(ctx, `SELECT CASE WHEN length(CAST(metadata_json AS BLOB))<=16777216 THEN metadata_json ELSE NULL END FROM document_metadata WHERE site_id=? LIMIT ?`, op.Actor.SiteID, referenceScanBudget+1)
	if err != nil {
		return contentStorageError()
	}
	count := 0
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return contentStorageError()
		}
		count++
		if count > referenceScanBudget {
			rows.Close()
			return platform.NewError(platform.CodeDependencyUnavailable, true, "文档资源来源完整扫描超出登记预算")
		}
		if runtime.validateMetadata(raw) != nil {
			rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "文档元数据来源不符合唯一schema")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return contentStorageError()
	}
	var unknown int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents d LEFT JOIN document_metadata m ON m.site_id=d.site_id AND m.document_id=d.document_id LEFT JOIN draft_heads h ON h.site_id=d.site_id AND h.document_id=d.document_id LEFT JOIN draft_resource_references f ON f.site_id=h.site_id AND f.document_id=h.document_id AND f.revision_id=h.revision_id AND f.kind='current_draft' WHERE d.site_id=? AND (m.document_id IS NULL OR h.document_id IS NULL OR f.document_id IS NULL OR d.reference_version<1 OR f.source_version<>d.reference_version OR json_extract(m.metadata_json,'$.siteId') IS NOT d.site_id OR json_extract(m.metadata_json,'$.documentId') IS NOT d.document_id OR json_extract(m.metadata_json,'$.draftRevisionId') IS NOT h.revision_id OR json_extract(m.metadata_json,'$.state') IS NOT d.state OR json_extract(m.metadata_json,'$.shareImage') IS NOT NULL OR d.state NOT IN ('draft','trash','published','purged') OR (d.state<>'published' AND json_extract(m.metadata_json,'$.publishedReference') IS NOT NULL) OR (json_extract(m.metadata_json,'$.lastPublishedAt') IS NOT NULL AND NOT EXISTS (SELECT 1 FROM publication_history y WHERE y.site_id=d.site_id AND y.document_id=d.document_id)) OR (d.state='published' AND (json_extract(m.metadata_json,'$.publishedReference.documentId') IS NOT d.document_id OR NOT EXISTS (SELECT 1 FROM publication_snapshots s WHERE s.site_id=d.site_id AND s.published_revision_id=json_extract(m.metadata_json,'$.publishedReference.publishedRevisionId') AND s.document_id=d.document_id AND s.content_hash=json_extract(m.metadata_json,'$.publishedReference.contentHash') AND s.schema_version=json_extract(m.metadata_json,'$.publishedReference.schemaVersion') AND s.revision=json_extract(m.metadata_json,'$.publishedReference.revision') AND s.visibility_revision=json_extract(m.metadata_json,'$.visibilityRevision') AND s.data_epoch=json_extract(m.metadata_json,'$.publishedReference.dataEpoch') AND EXISTS (SELECT 1 FROM committed_revisions c WHERE c.site_id=s.site_id AND c.document_id=s.document_id AND c.revision_id=s.draft_revision_id AND c.state='committed' AND c.data_epoch=s.data_epoch)) OR NOT EXISTS (SELECT 1 FROM publication_history y WHERE y.site_id=d.site_id AND y.published_revision_id=json_extract(m.metadata_json,'$.publishedReference.publishedRevisionId')) OR NOT EXISTS (SELECT 1 FROM publication_resource_references p WHERE p.site_id=d.site_id AND p.document_id=d.document_id AND p.published_revision_id=json_extract(m.metadata_json,'$.publishedReference.publishedRevisionId') AND p.kind='current_published'))))`, op.Actor.SiteID).Scan(&unknown)
	if err != nil {
		return contentStorageError()
	}
	if unknown != 0 {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "引用来源存在缺失或尚未支持的发布及元数据资格")
	}
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM publication_resource_references p LEFT JOIN document_metadata m ON m.site_id=p.site_id AND m.document_id=p.document_id WHERE p.site_id=? AND (json_extract(m.metadata_json,'$.documentId') IS NOT p.document_id OR NOT EXISTS (SELECT 1 FROM publication_snapshots s WHERE s.site_id=p.site_id AND s.published_revision_id=p.published_revision_id AND s.document_id=p.document_id AND s.draft_revision_id=p.revision_id) OR (p.kind='current_published' AND (json_extract(m.metadata_json,'$.state') IS NOT 'published' OR json_extract(m.metadata_json,'$.publishedReference.publishedRevisionId') IS NOT p.published_revision_id)))`, op.Actor.SiteID).Scan(&unknown)
	if err != nil {
		return contentStorageError()
	}
	if unknown != 0 {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "已发布资源引用与当前快照不一致")
	}
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM committed_revisions c LEFT JOIN revision_preparations p ON p.commit_id=c.commit_id LEFT JOIN draft_commit_receipts r ON r.commit_id=c.commit_id LEFT JOIN draft_operation_intents i ON i.site_id=p.site_id AND i.data_epoch=p.data_epoch AND i.operation_id=p.operation_id LEFT JOIN draft_resource_references f ON f.site_id=c.site_id AND f.document_id=c.document_id AND f.revision_id=c.revision_id WHERE c.site_id=? AND (c.data_epoch<>? OR p.commit_id IS NULL OR r.commit_id IS NULL OR i.operation_id IS NULL OR f.document_id IS NULL)`, op.Actor.SiteID, op.DataEpoch).Scan(&unknown)
	if err != nil {
		return contentStorageError()
	}
	if unknown != 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "已提交正文缺少完整引用来源资格")
	}
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM draft_resource_references f LEFT JOIN documents d ON d.site_id=f.site_id AND d.document_id=f.document_id LEFT JOIN committed_revisions c ON c.site_id=f.site_id AND c.document_id=f.document_id AND c.revision_id=f.revision_id LEFT JOIN draft_heads h ON h.site_id=f.site_id AND h.document_id=f.document_id WHERE f.site_id=? AND (d.document_id IS NULL OR c.commit_id IS NULL OR c.data_epoch<>? OR (f.kind='current_draft' AND (f.revision_id<>h.revision_id OR f.source_version<>d.reference_version)) OR (f.kind='history' AND f.revision_id=h.revision_id))`, op.Actor.SiteID, op.DataEpoch).Scan(&unknown)
	if err != nil {
		return contentStorageError()
	}
	if unknown != 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "资源引用索引缺正文资格或当前历史分类矛盾")
	}
	return nil
}

type referenceSourceRecord struct {
	operation     Identifier
	action, state string
	planRaw       []byte
}

func readReferenceSources(ctx context.Context, tx *sql.Tx, op OperationContext, runtime ReferenceRuntime) ([]gen.ResourceReference, error) {
	rows, err := tx.QueryContext(ctx, `SELECT operation_id,action,state FROM draft_operation_intents WHERE site_id=? AND data_epoch=? AND (state IN ('pending','committed') OR EXISTS (SELECT 1 FROM revision_preparations p JOIN committed_revisions c ON c.commit_id=p.commit_id WHERE p.site_id=draft_operation_intents.site_id AND p.data_epoch=draft_operation_intents.data_epoch AND p.operation_id=draft_operation_intents.operation_id)) ORDER BY operation_id LIMIT ?`, op.Actor.SiteID, op.DataEpoch, referenceScanBudget+1)
	if err != nil {
		return nil, contentStorageError()
	}
	sources := make([]referenceSourceRecord, 0)
	for rows.Next() {
		var source referenceSourceRecord
		if err = rows.Scan(&source.operation, &source.action, &source.state); err != nil {
			rows.Close()
			return nil, contentStorageError()
		}
		sources = append(sources, source)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, contentStorageError()
	}
	if len(sources) > referenceScanBudget {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "引用来源完整扫描超出登记预算")
	}
	// 彻底删除并已发出空集合释放事件的文档，它的草稿与发布引用不再占用资源；还没补发的旧文档照旧计入。
	released, err := queryIdentifiers(ctx, tx, `SELECT document_id FROM purged_reference_releases WHERE site_id=?`, op.Actor.SiteID)
	if err != nil {
		return nil, err
	}
	purged := make(map[Identifier]bool, len(released))
	for _, id := range released {
		purged[id] = true
	}
	result := make([]gen.ResourceReference, 0)
	seen := make(map[string]bool)
	for _, source := range sources {
		if err = tx.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(plan_json AS BLOB))<=16777216 THEN plan_json ELSE NULL END FROM draft_operation_intents WHERE site_id=? AND data_epoch=? AND operation_id=?`, op.Actor.SiteID, op.DataEpoch, source.operation).Scan(&source.planRaw); err != nil {
			return nil, contentStorageError()
		}
		var plan draftOperationPlan
		if decodeStorageJSON(source.planRaw, &plan) != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "引用来源固定意图损坏")
		}
		if err = validateDraftOperationPlan(source.action, plan); err != nil {
			return nil, err
		}
		for _, ref := range plan.References {
			if ref.DocumentId != plan.DocumentID || ref.SourceRevisionId != plan.RevisionID || ref.SourceVersion != plan.SourceVersion || ref.Kind != gen.CurrentDraft || !ref.Retained {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "固定意图引用身份或分类矛盾")
			}
		}
		if source.action == "save" {
			_, digest, err := draftMediaReferences(plan.References)
			if err != nil || digest != plan.ReferenceDigest {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "引用来源正文索引与预留摘要矛盾")
			}
		}
		refs, state, err := readCommittedReferenceSource(ctx, tx, op, source, plan)
		if err != nil {
			return nil, err
		}
		if state == "aborted" || state == "rejected" || purged[plan.DocumentID] {
			continue
		}
		for _, ref := range refs {
			raw, err := json.Marshal(ref)
			if err != nil || runtime.validateReference(raw) != nil {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "引用来源条目不符合唯一schema")
			}
			key := referenceSortKey(ref)
			if seen[key] {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "引用来源身份重复")
			}
			seen[key] = true
			result = append(result, ref)
			if len(result) > referenceScanBudget {
				return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "引用来源完整集合超出登记预算")
			}
		}
	}
	published, err := readPublicationReferences(ctx, tx, op, runtime)
	if err != nil {
		return nil, err
	}
	for _, ref := range published {
		if purged[ref.DocumentId] {
			continue
		}
		raw, err := json.Marshal(ref)
		if err != nil || runtime.validateReference(raw) != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "已发布引用来源条目不符合唯一schema")
		}
		key := referenceSortKey(ref)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, ref)
		if len(result) > referenceScanBudget {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "引用来源完整集合超出登记预算")
		}
	}
	return result, nil
}

func readPublicationReferences(ctx context.Context, tx *sql.Tx, op OperationContext, runtime ReferenceRuntime) ([]gen.ResourceReference, error) {
	rows, err := tx.QueryContext(ctx, `SELECT document_id,revision_id,source_version,kind,CASE WHEN length(CAST(references_json AS BLOB))<=16777216 THEN references_json ELSE NULL END FROM publication_resource_references WHERE site_id=? ORDER BY document_id,published_revision_id LIMIT ?`, op.Actor.SiteID, referenceScanBudget+1)
	if err != nil {
		return nil, contentStorageError()
	}
	defer rows.Close()
	result := make([]gen.ResourceReference, 0)
	rowsSeen := 0
	for rows.Next() {
		var documentID, revisionID Identifier
		var sourceVersion int
		var kind string
		var raw []byte
		if err = rows.Scan(&documentID, &revisionID, &sourceVersion, &kind, &raw); err != nil {
			return nil, contentStorageError()
		}
		rowsSeen++
		if rowsSeen > referenceScanBudget {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "已发布引用来源完整扫描超出登记预算")
		}
		var refs []gen.ResourceReference
		if raw == nil || decodeStorageJSON(raw, &refs) != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "已发布引用来源损坏")
		}
		for i := range refs {
			service, _ := refs[i].SourceService.(string)
			if service != "content" || refs[i].DocumentId != documentID || refs[i].SourceRevisionId != revisionID || refs[i].SourceVersion != sourceVersion || !refs[i].Retained {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "已发布引用来源身份矛盾")
			}
			refs[i].Kind = gen.ResourceReferenceKind(kind)
			result = append(result, refs[i])
		}
	}
	if err = rows.Err(); err != nil {
		return nil, contentStorageError()
	}
	return result, nil
}

func readCommittedReferenceSource(ctx context.Context, tx *sql.Tx, op OperationContext, source referenceSourceRecord, plan draftOperationPlan) ([]gen.ResourceReference, string, error) {
	var preparedRaw, receiptRaw, refsRaw []byte
	var sourceVersion int
	var kind string
	err := tx.QueryRowContext(ctx, `SELECT p.result_json,r.result_json,CASE WHEN length(CAST(f.references_json AS BLOB))<=16777216 THEN f.references_json ELSE NULL END,f.source_version,f.kind FROM committed_revisions c JOIN revision_preparations p ON p.commit_id=c.commit_id AND p.site_id=c.site_id AND p.document_id=c.document_id AND p.revision_id=c.revision_id AND p.data_epoch=c.data_epoch JOIN draft_commit_receipts r ON r.commit_id=c.commit_id AND r.site_id=c.site_id AND r.data_epoch=c.data_epoch JOIN draft_resource_references f ON f.site_id=c.site_id AND f.document_id=c.document_id AND f.revision_id=c.revision_id WHERE c.site_id=? AND c.data_epoch=? AND c.document_id=? AND c.revision_id=? AND p.operation_id=? AND r.operation_id=?`, op.Actor.SiteID, op.DataEpoch, plan.DocumentID, plan.RevisionID, source.operation, source.operation).Scan(&preparedRaw, &receiptRaw, &refsRaw, &sourceVersion, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM committed_revisions WHERE site_id=? AND document_id=? AND revision_id=?`, op.Actor.SiteID, plan.DocumentID, plan.RevisionID).Scan(&count); err != nil {
			return nil, "", contentStorageError()
		}
		if count != 0 || source.state == "committed" {
			return nil, "", platform.NewError(platform.CodeIntegrityError, false, "引用来源已提交但完整资格缺失")
		}
		if source.state == "aborted" || source.state == "rejected" {
			return nil, source.state, nil
		}
		if source.state != "pending" {
			return nil, "", platform.NewError(platform.CodeIntegrityError, false, "引用来源决定未知")
		}
		refs := slices.Clone(plan.References)
		for i := range refs {
			refs[i].Kind = gen.ResourceReferenceKind("pending_pin")
			refs[i].Retained = true
		}
		return refs, "pending", nil
	}
	if err != nil {
		return nil, "", contentStorageError()
	}
	// COMMITTED是不可撤销历史决定；正文后来隔离不撤销精确资格/原回执/refs保护。
	var prepared WriteRevisionFileResult
	var receipt CommitDraftJournalResult
	var refs []gen.ResourceReference
	expected, _ := platform.CanonicalJSON(plan.References)
	if decodeStorageJSON(preparedRaw, &prepared) != nil || decodeStorageJSON(receiptRaw, &receipt) != nil || decodeStorageJSON(refsRaw, &refs) != nil {
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "引用来源提交材料损坏")
	}
	actual, _ := platform.CanonicalJSON(refs)
	if !sameStoredDraft(prepared.Draft, receipt.Draft) || receipt.Draft.DocumentId != plan.DocumentID || receipt.Draft.RevisionId != plan.RevisionID || receipt.Draft.BodyHash != plan.BodyHash || receipt.Draft.DraftVersion != plan.ExpectedDraftVersion+1 || sourceVersion != plan.SourceVersion || string(expected) != string(actual) || (kind != "current_draft" && kind != "history") {
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "引用来源资格与正文索引矛盾")
	}
	for i := range refs {
		if refs[i].DocumentId != plan.DocumentID || refs[i].SourceRevisionId != plan.RevisionID || refs[i].SourceVersion != plan.SourceVersion || refs[i].Kind != gen.CurrentDraft || !refs[i].Retained {
			return nil, "", platform.NewError(platform.CodeIntegrityError, false, "引用条目来源身份矛盾")
		}
		refs[i].Kind = gen.ResourceReferenceKind(kind)
	}
	return refs, "committed", nil
}
func referenceSortKey(ref gen.ResourceReference) string {
	return ref.DocumentId + "\x00" + ref.SourceRevisionId + "\x00" + strconv.Itoa(ref.SourceVersion) + "\x00" + string(ref.Kind) + "\x00" + ref.BlockId + "\x00" + ref.Asset.AssetId + "\x00" + strconv.Itoa(ref.Asset.Version)
}
