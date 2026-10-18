package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/content/internal/gen"
	"sort"
	"strconv"
)

type ReadRevisionHistoryInput struct {
	DocumentId string               `json:"documentId"`
	Page       ReferencePageRequest `json:"page"`
}
type ReadRevisionHistoryResult = gen.ReadRevisionHistoryResult
type DiffRevisionsInput struct {
	DocumentId      string `json:"documentId"`
	LeftRevisionId  string `json:"leftRevisionId"`
	RightRevisionId string `json:"rightRevisionId"`
}
type DiffRevisionsResult = gen.DiffRevisionsResult
type historyCursor struct {
	Watermark int64 `json:"watermark"`
	Offset    int   `json:"offset"`
}

func ReadRevisionHistory(ctx context.Context, op OperationContext, input ReadRevisionHistoryInput) (ReadRevisionHistoryResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return ReadRevisionHistoryResult{}, err
	}
	if err = authorizeTree(ctx, op, runtime, "content.publish", "readRevisionHistory"); err != nil {
		return ReadRevisionHistoryResult{}, err
	}
	if input.Page.Limit == 0 {
		input.Page.Limit = 20
	}
	binding, _ := platform.CanonicalJSON(struct {
		Actor, Session, Site, Epoch, Document string
		SessionRevision, PermissionRevision   int64
		Limit                                 int
	}{op.Actor.SubjectID, op.Actor.SessionID, op.Actor.SiteID, op.DataEpoch, input.DocumentId, op.Actor.SessionRevision, op.Actor.PermissionRevision, input.Page.Limit})
	scope := "content.history.v1:" + platform.HashBytes(binding)
	position := historyCursor{}
	if input.Page.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(runtime.Reference.SigningKey, scope, input.Page.Cursor)
		if err != nil {
			return ReadRevisionHistoryResult{}, err
		}
		if json.Unmarshal(raw, &position) != nil || position.Offset < 1 || position.Watermark < 1 {
			return ReadRevisionHistoryResult{}, platform.NewError(platform.CodeValidation, false, "历史游标不合法")
		}
	}
	tx, err := beginTreeRead(ctx, op, runtime)
	if err != nil {
		return ReadRevisionHistoryResult{}, err
	}
	defer tx.Rollback()
	var revision int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM reference_scan_state WHERE singleton=1`).Scan(&revision); err != nil {
		return ReadRevisionHistoryResult{}, contentStorageError()
	}
	if position.Watermark != 0 && position.Watermark != revision {
		return ReadRevisionHistoryResult{}, platform.NewError(platform.CodeStaleResult, false, "发布历史已变化，请重新读取")
	}
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE site_id=? AND document_id=? AND state<>'purged'`, op.Actor.SiteID, input.DocumentId).Scan(&exists); err != nil {
		return ReadRevisionHistoryResult{}, contentStorageError()
	}
	if exists != 1 {
		return ReadRevisionHistoryResult{}, platform.NewError(platform.CodeNotFound, false, "历史所属文档不存在")
	}
	rows, err := tx.QueryContext(ctx, `SELECT history_id,reference_json,author_account_id,author_account_name,created_at,commit_mirror_state,git_commit_id,operation FROM publication_history WHERE site_id=? AND document_id=? ORDER BY created_at DESC,history_id DESC LIMIT ? OFFSET ?`, op.Actor.SiteID, input.DocumentId, input.Page.Limit+1, position.Offset)
	if err != nil {
		return ReadRevisionHistoryResult{}, contentStorageError()
	}
	result := ReadRevisionHistoryResult{History: []gen.HistoryEntry{}, PageInfo: gen.PageInfo{HasMore: false}}
	for rows.Next() {
		var entry gen.HistoryEntry
		var raw []byte
		var when, mirror, operation string
		var commit sql.NullString
		if err = rows.Scan(&entry.HistoryId, &raw, &entry.AuthorAccountId, &entry.AuthorAccountName, &when, &mirror, &commit, &operation); err != nil {
			rows.Close()
			return ReadRevisionHistoryResult{}, contentStorageError()
		}
		if json.Unmarshal(raw, &entry.Reference) != nil || entry.Reference.DocumentId != input.DocumentId || entry.Reference.DataEpoch != op.DataEpoch || entry.AuthorAccountName == "" || !platform.ValidIdentifier(entry.AuthorAccountId) || !platform.ValidIdentifier(entry.HistoryId) {
			rows.Close()
			return ReadRevisionHistoryResult{}, platform.NewError(platform.CodeIntegrityError, false, "原发布历史身份或引用损坏")
		}
		timestamp, err := platform.ParseTimestamp(when)
		if err != nil {
			rows.Close()
			return ReadRevisionHistoryResult{}, platform.NewError(platform.CodeIntegrityError, false, "原发布时间损坏")
		}
		entry.CreatedAt = timestamp
		if (mirror != "pending" && mirror != "mirrored" && mirror != "failed") || (operation != "publish" && operation != "rollback") || (mirror == "mirrored" && !commit.Valid) {
			rows.Close()
			return ReadRevisionHistoryResult{}, platform.NewError(platform.CodeIntegrityError, false, "发布历史镜像回执损坏")
		}
		entry.CommitMirrorState = gen.HistoryEntryCommitMirrorState(mirror)
		entry.Operation = gen.HistoryEntryOperation(operation)
		if commit.Valid {
			entry.GitCommitId = &commit.String
		}
		result.History = append(result.History, entry)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ReadRevisionHistoryResult{}, contentStorageError()
	}
	if len(result.History) > input.Page.Limit {
		result.History = result.History[:input.Page.Limit]
		result.PageInfo.HasMore = true
		raw, _ := platform.CanonicalJSON(historyCursor{Watermark: revision, Offset: position.Offset + len(result.History)})
		cursor, err := serviceauth.SignCursor(runtime.Reference.SigningKey, scope, raw)
		if err != nil {
			return ReadRevisionHistoryResult{}, err
		}
		result.PageInfo.NextCursor = &cursor
	}
	return result, nil
}
func readHistoricalBody(ctx context.Context, op OperationContext, runtime TreeRuntime, documentID, publishedID string) (DocumentBody, error) {
	tx, err := beginTreeRead(ctx, op, runtime)
	if err != nil {
		return DocumentBody{}, err
	}
	var draft, hash, epoch string
	err = tx.QueryRowContext(ctx, `SELECT p.draft_revision_id,p.content_hash,p.data_epoch FROM publication_snapshots p WHERE p.site_id=? AND p.document_id=? AND p.published_revision_id=? AND EXISTS(SELECT 1 FROM publication_history h WHERE h.site_id=p.site_id AND h.document_id=p.document_id AND h.published_revision_id=p.published_revision_id)`, op.Actor.SiteID, documentID, publishedID).Scan(&draft, &hash, &epoch)
	tx.Rollback()
	if errors.Is(err, sql.ErrNoRows) {
		return DocumentBody{}, platform.NewError(platform.CodeNotFound, false, "指定发布历史不存在")
	}
	if err != nil {
		return DocumentBody{}, contentStorageError()
	}
	if epoch != op.DataEpoch {
		return DocumentBody{}, platform.NewError(platform.CodeVersionMismatch, false, "历史正文属于其他代次")
	}
	files := runtime.Reference.Store.ConfigureRevisionStorage(RevisionStorageConfig{})
	result, err := ReadRevisionBody(WithRevisionFileStore(ctx, files), op, ReadRevisionBodyInput{DocumentID: documentID, RevisionID: draft, ExpectedHash: &hash})
	return result.Body, err
}

type historyBlock struct {
	Hash     string
	Position string
}

func historyBlockMap(body DocumentBody) (map[string]historyBlock, error) {
	value, err := bodyJSONValue(body)
	if err != nil {
		return nil, err
	}
	result := map[string]historyBlock{}
	var visit func(any, string, string, string) error
	visit = func(value any, parent, container, previous string) error {
		switch node := value.(type) {
		case map[string]any:
			if attrs, ok := node["attrs"].(map[string]any); ok {
				if id, ok := attrs["blockId"].(string); ok && id != "" {
					raw, _ := json.Marshal(node)
					var block gen.Block
					if json.Unmarshal(raw, &block) != nil {
						return platform.NewError(platform.CodeIntegrityError, false, "历史块无法解码")
					}
					hashed, err := HashBlock(HashBlockInput{Block: block})
					if err != nil {
						return err
					}
					result[id] = historyBlock{Hash: hashed.BlockHash, Position: parent + "/" + container + "/" + previous}
					parent = id
					container = ""
				}
			}
			if children, ok := node["content"]; ok {
				return visit(children, parent, container+"/content", "")
			}
		case []any:
			previous := ""
			for i, child := range node {
				position := container
				asMap, isMap := child.(map[string]any)
				id := ""
				if isMap {
					if attrs, ok := asMap["attrs"].(map[string]any); ok {
						id, _ = attrs["blockId"].(string)
					}
				}
				if id == "" {
					position += "/" + strconv.Itoa(i)
				}
				if err := visit(child, parent, position, previous); err != nil {
					return err
				}
				if id != "" {
					previous = id
				}
			}
		}
		return nil
	}
	if err = visit(value, "", "", ""); err != nil {
		return nil, err
	}
	return result, nil
}
func DiffRevisions(ctx context.Context, op OperationContext, input DiffRevisionsInput) (DiffRevisionsResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return DiffRevisionsResult{}, err
	}
	if err = authorizeTree(ctx, op, runtime, "content.publish", "diffRevisions"); err != nil {
		return DiffRevisionsResult{}, err
	}
	leftBody, err := readHistoricalBody(ctx, op, runtime, input.DocumentId, input.LeftRevisionId)
	if err != nil {
		return DiffRevisionsResult{}, err
	}
	rightBody, err := readHistoricalBody(ctx, op, runtime, input.DocumentId, input.RightRevisionId)
	if err != nil {
		return DiffRevisionsResult{}, err
	}
	left, err := historyBlockMap(leftBody)
	if err != nil {
		return DiffRevisionsResult{}, err
	}
	right, err := historyBlockMap(rightBody)
	if err != nil {
		return DiffRevisionsResult{}, err
	}
	result := DiffRevisionsResult{AddedBlockIds: []string{}, RemovedBlockIds: []string{}, ChangedBlockIds: []string{}, MovedBlockIds: []string{}}
	for id, before := range left {
		after, ok := right[id]
		if !ok {
			result.RemovedBlockIds = append(result.RemovedBlockIds, id)
			continue
		}
		if before.Hash != after.Hash {
			result.ChangedBlockIds = append(result.ChangedBlockIds, id)
		}
		if before.Position != after.Position {
			result.MovedBlockIds = append(result.MovedBlockIds, id)
		}
	}
	for id := range right {
		if _, ok := left[id]; !ok {
			result.AddedBlockIds = append(result.AddedBlockIds, id)
		}
	}
	sort.Strings(result.AddedBlockIds)
	sort.Strings(result.RemovedBlockIds)
	sort.Strings(result.ChangedBlockIds)
	sort.Strings(result.MovedBlockIds)
	return result, nil
}
