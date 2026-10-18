package protection

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/data-protection/internal/gen"
	"time"
)

type ListBackupsInput struct {
	Cursor string               `json:"cursor"`
	Limit  int                  `json:"limit"`
	Actor  platform.AuthContext `json:"actor"`
}
type ListBackupsResult = gen.ListBackupsResult
type ReadBackupInput struct {
	BackupId string               `json:"backupId"`
	Actor    platform.AuthContext `json:"actor"`
}
type ReadBackupResult = gen.ReadBackupResult
type ReadMirrorStatusInput struct {
	MirrorId string               `json:"mirrorId,omitempty"`
	Actor    platform.AuthContext `json:"actor"`
}
type ReadMirrorStatusResult = gen.ReadMirrorStatusResult
type backupCursor struct {
	CreatedAt string `json:"createdAt"`
	ID        string `json:"id"`
	Revision  int64  `json:"revision"`
	ExpiresAt int64  `json:"expiresAt"`
}

func ListBackups(ctx context.Context, op OperationContext, in ListBackupsInput) (ListBackupsResult, error) {
	if err := requireAdministrator(op); err != nil {
		return ListBackupsResult{}, err
	}
	store, err := Catalog(op)
	if err != nil {
		return ListBackupsResult{}, err
	}
	limit := in.Limit
	if limit == 0 {
		limit = 20
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return ListBackupsResult{}, storageError(err)
	}
	defer tx.Rollback()
	var revision int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM catalog_meta WHERE singleton=1 AND data_epoch=?`, op.DataEpoch).Scan(&revision); err != nil {
		return ListBackupsResult{}, storageError(err)
	}
	binding, _ := platform.CanonicalJSON(struct {
		Site, Epoch, Subject, Session       string
		SessionRevision, PermissionRevision int64
		Limit                               int
	}{op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, op.Actor.SessionID, op.Actor.SessionRevision, op.Actor.PermissionRevision, limit})
	scope := "data-protection.backups.v1:" + platform.HashBytes(binding)
	cursor := backupCursor{Revision: revision, ExpiresAt: time.Now().UTC().Add(10 * time.Minute).Unix()}
	if in.Cursor != "" {
		raw, verifyErr := serviceauth.VerifyCursor(store.Key, scope, in.Cursor)
		if verifyErr != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Revision != revision || cursor.ExpiresAt <= time.Now().UTC().Unix() {
			return ListBackupsResult{}, platform.NewError(platform.CodeStaleResult, false, "备份目录游标已过期或目录水位变化")
		}
	}
	query := `SELECT backup_id,created_at FROM backup_runs`
	args := []any{}
	if in.Cursor != "" {
		query += ` WHERE created_at<? OR(created_at=? AND backup_id<?)`
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	query += ` ORDER BY created_at DESC,backup_id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return ListBackupsResult{}, storageError(err)
	}
	defer rows.Close()
	result := ListBackupsResult{BackupIds: []string{}}
	more := false
	for rows.Next() {
		var id, created string
		if err = rows.Scan(&id, &created); err != nil {
			return ListBackupsResult{}, storageError(err)
		}
		if len(result.BackupIds) == limit {
			more = true
			break
		}
		result.BackupIds = append(result.BackupIds, id)
		cursor.ID = id
		cursor.CreatedAt = created
	}
	if err = rows.Err(); err != nil {
		return ListBackupsResult{}, storageError(err)
	}
	if more {
		raw, _ := platform.CanonicalJSON(cursor)
		result.NextCursor, err = serviceauth.SignCursor(store.Key, scope, raw)
		if err != nil {
			return ListBackupsResult{}, err
		}
	}
	return result, nil
}
func ReadBackup(ctx context.Context, op OperationContext, in ReadBackupInput) (ReadBackupResult, error) {
	if err := requireAdministrator(op); err != nil {
		return ReadBackupResult{}, err
	}
	store, err := Catalog(op)
	if err != nil {
		return ReadBackupResult{}, err
	}
	result := ReadBackupResult{Issues: []gen.SafeIssue{}}
	var issues string
	err = store.DB.QueryRowContext(ctx, `SELECT manifest_json,status,issues_json FROM backup_runs WHERE backup_id=?`, in.BackupId).Scan(&result.ManifestJSON, &result.State, &issues)
	if errors.Is(err, sql.ErrNoRows) {
		return ReadBackupResult{}, platform.NewError(platform.CodeNotFound, false, "备份目录不存在")
	}
	if err != nil {
		return ReadBackupResult{}, storageError(err)
	}
	if json.Unmarshal([]byte(issues), &result.Issues) != nil || result.Issues == nil {
		return ReadBackupResult{}, platform.NewError(platform.CodeIntegrityError, false, "备份安全报告损坏")
	}
	return result, nil
}
func ReadMirrorStatus(ctx context.Context, op OperationContext, in ReadMirrorStatusInput) (ReadMirrorStatusResult, error) {
	if err := requireAdministrator(op); err != nil {
		return ReadMirrorStatusResult{}, err
	}
	store, err := Catalog(op)
	if err != nil {
		return ReadMirrorStatusResult{}, err
	}
	query := `SELECT state,local_commit,remote_ref FROM mirror_runs`
	args := []any{}
	if in.MirrorId != "" {
		query += ` WHERE mirror_id=?`
		args = append(args, in.MirrorId)
	}
	query += ` ORDER BY created_at DESC,mirror_id DESC LIMIT 1`
	var result ReadMirrorStatusResult
	err = store.DB.QueryRowContext(ctx, query, args...).Scan(&result.State, &result.CommitHash, &result.RemoteHead)
	if errors.Is(err, sql.ErrNoRows) {
		return ReadMirrorStatusResult{}, platform.NewError(platform.CodeNotFound, false, "尚无镜像记录")
	}
	if err != nil {
		return ReadMirrorStatusResult{}, storageError(err)
	}
	if result.State == "syncing" {
		result.State = "needs-reconcile"
	}
	return result, nil
}
