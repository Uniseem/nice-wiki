package content

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
	"strings"
)

type ReadPublicTreeInput struct{}
type ReadPublicTreeResult = gen.ReadPublicTreeResult
type ResolvePublicPathInput struct {
	Path string `json:"path"`
}
type ResolvePublicPathResult = gen.ResolvePublicPathResult

func ReadPublicTree(ctx context.Context, op OperationContext, input ReadPublicTreeInput) (ReadPublicTreeResult, error) {
	if op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService || !publicationValidationCaller(op.CallerService) {
		return ReadPublicTreeResult{}, platform.NewError(platform.CodeForbidden, false, "公开树仅供已登记服务只读")
	}
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return ReadPublicTreeResult{}, err
	}
	tx, err := beginTreeRead(ctx, op, runtime)
	if err != nil {
		return ReadPublicTreeResult{}, err
	}
	defer tx.Rollback()
	return readPublicRouteSnapshot(ctx, tx, op, runtime)
}
func readPublicRouteSnapshot(ctx context.Context, tx *sql.Tx, op OperationContext, runtime TreeRuntime) (ReadPublicTreeResult, error) {
	revision, documents, err := readTreeSnapshot(ctx, tx, op, runtime)
	if err != nil {
		return ReadPublicTreeResult{}, err
	}
	paths, err := canonicalDocumentPaths(documents)
	if err != nil {
		return ReadPublicTreeResult{}, err
	}
	byID := map[string]string{}
	for path, id := range paths {
		byID[id] = path
	}
	visible := map[string]bool{}
	metadata := map[string]gen.DocumentMetadata{}
	for _, document := range documents {
		metadata[document.DocumentId] = document
		if document.State != "published" || document.PublishedReference == nil {
			continue
		}
		checked, err := validatePublicationRef(ctx, tx, op, true, document, *document.PublishedReference)
		if err != nil {
			return ReadPublicTreeResult{}, err
		}
		if checked.Visible && checked.MatchesCurrent {
			visible[document.DocumentId] = true
		}
	}
	nodes := make([]gen.PublicTreeNode, 0, len(visible))
	for _, document := range treePreorder(documents, false) {
		if !visible[document.DocumentId] {
			continue
		}
		path, ok := byID[document.DocumentId]
		if !ok || document.PathSegment == nil {
			return ReadPublicTreeResult{}, platform.NewError(platform.CodeIntegrityError, false, "公开文档缺少完整规范路径")
		}
		if document.LastPublishedAt == nil {
			return ReadPublicTreeResult{}, platform.NewError(platform.CodeIntegrityError, false, "公开文档缺少发布时间")
		}
		parent := document.ParentId
		for parent != nil && !visible[*parent] {
			ancestor, ok := metadata[*parent]
			if !ok {
				return ReadPublicTreeResult{}, platform.NewError(platform.CodeIntegrityError, false, "公开导航缺少祖先")
			}
			parent = ancestor.ParentId
		}
		nodes = append(nodes, gen.PublicTreeNode{DocumentId: document.DocumentId, ParentId: parent, Title: document.Title, PathSegment: *document.PathSegment, OrderKey: document.OrderKey, Reference: *document.PublishedReference, VisibilityRevision: document.VisibilityRevision, CanonicalPath: path, LastPublishedAt: *document.LastPublishedAt, SeoDescription: document.SeoDescription})
	}
	return ReadPublicTreeResult{Nodes: nodes, TreeRevision: int(revision)}, nil
}
func ResolvePublicPath(ctx context.Context, op OperationContext, input ResolvePublicPathInput) (ResolvePublicPathResult, error) {
	if op.Actor.Kind != "service" || op.Actor.SubjectID != op.CallerService || !publicationValidationCaller(op.CallerService) {
		return ResolvePublicPathResult{}, platform.NewError(platform.CodeForbidden, false, "公开路径仅供已登记服务只读")
	}
	if !strings.HasPrefix(input.Path, "/") || input.Path == "/" || len(input.Path) > 2048 {
		return ResolvePublicPathResult{}, platform.NewError(platform.CodeValidation, false, "公开路径不符合规范")
	}
	for _, segment := range strings.Split(input.Path[1:], "/") {
		if !pathSegmentPattern.MatchString(segment) {
			return ResolvePublicPathResult{}, platform.NewError(platform.CodeValidation, false, "公开路径包含非规范段")
		}
	}
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return ResolvePublicPathResult{}, err
	}
	tx, err := beginTreeRead(ctx, op, runtime)
	if err != nil {
		return ResolvePublicPathResult{}, err
	}
	defer tx.Rollback()
	tree, err := readPublicRouteSnapshot(ctx, tx, op, runtime)
	if err != nil {
		return ResolvePublicPathResult{}, err
	}
	result := ResolvePublicPathResult{State: "not_found", RouteRevision: tree.TreeRevision}
	current := map[string]gen.PublicTreeNode{}
	for _, node := range tree.Nodes {
		current[node.DocumentId] = node
		if node.CanonicalPath == input.Path {
			id, path, ref := node.DocumentId, node.CanonicalPath, node.Reference
			result.State = "found"
			result.DocumentId = &id
			result.Reference = &ref
			result.CanonicalPath = &path
			return result, nil
		}
	}
	var manifest int64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM redirect_manifest_heads WHERE site_id=?`, op.Actor.SiteID).Scan(&manifest)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, contentStorageError()
	}
	var state, documentID, epoch string
	var target sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT status,document_id,target_document_id,data_epoch FROM redirect_manifest_entries WHERE site_id=? AND manifest_revision=? AND old_path=?`, op.Actor.SiteID, manifest, input.Path).Scan(&state, &documentID, &target, &epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, contentStorageError()
	}
	if epoch != op.DataEpoch {
		return result, platform.NewError(platform.CodeVersionMismatch, false, "旧路径决定不是当前代次")
	}
	// 删除时记下的“已删除”：文档后来恢复并重新发布到了别的路径，旧地址就跳过去，不再报已删除。
	if node, ok := current[documentID]; ok && state == "gone" && node.CanonicalPath != input.Path {
		state = "redirect"
	}
	if state == "gone" {
		result.State = "gone"
		return result, nil
	}
	if state != "redirect" {
		return result, platform.NewError(platform.CodeIntegrityError, false, "旧路径决定状态损坏")
	}
	if target.Valid {
		documentID = target.String
	}
	node, ok := current[documentID]
	if !ok {
		result.State = "gone"
		return result, nil
	}
	if node.CanonicalPath == input.Path {
		return result, platform.NewError(platform.CodeIntegrityError, false, "旧路径决定存在自环")
	}
	id, path := node.DocumentId, node.CanonicalPath
	result.State = "redirect"
	result.DocumentId = &id
	result.CanonicalPath = &path
	return result, nil
}
