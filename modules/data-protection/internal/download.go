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

type AuthorizeBackupDownloadInput struct {
	BackupId string               `json:"backupId"`
	Actor    platform.AuthContext `json:"-"`
}
type AuthorizeBackupDownloadResult = gen.AuthorizeBackupDownloadResult
type downloadClaims struct {
	Nonce        string `json:"nonce"`
	BackupID     string `json:"backupId"`
	ManifestHash string `json:"manifestHash"`
	Epoch        string `json:"epoch"`
	Binding      string `json:"binding"`
	ExpiresAt    int64  `json:"expiresAt"`
}

// AuthorizeBackupDownload 的当前superadmin权限由HTTP边界核验；token不绕过后续普通用户授权/epoch门禁。
func AuthorizeBackupDownload(ctx context.Context, op OperationContext, in AuthorizeBackupDownloadInput) (AuthorizeBackupDownloadResult, error) {
	if err := requireAdministrator(op); err != nil {
		return AuthorizeBackupDownloadResult{}, err
	}
	store, err := Catalog(op)
	if err != nil {
		return AuthorizeBackupDownloadResult{}, err
	}
	bindingJSON, _ := platform.CanonicalJSON(struct {
		Site, Subject, Session              string
		SessionRevision, PermissionRevision int64
	}{op.Actor.SiteID, op.Actor.SubjectID, op.Actor.SessionID, op.Actor.SessionRevision, op.Actor.PermissionRevision})
	binding := platform.HashBytes(bindingJSON)
	inputJSON, _ := platform.CanonicalJSON(struct{ BackupID, Epoch, Binding string }{in.BackupId, op.DataEpoch, binding})
	inputHash := platform.HashBytes(inputJSON)
	tx, err := store.Control.BeginTx(ctx, nil)
	if err != nil {
		return AuthorizeBackupDownloadResult{}, storageError(err)
	}
	defer tx.Rollback()
	var state, manifestJSON, manifestHash, sourceEpoch string
	err = tx.QueryRowContext(ctx, `SELECT status,manifest_json,manifest_hash,source_epoch FROM backup_runs WHERE backup_id=?`, in.BackupId).Scan(&state, &manifestJSON, &manifestHash, &sourceEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthorizeBackupDownloadResult{}, platform.NewError(platform.CodeNotFound, false, "备份不存在")
	}
	if err != nil {
		return AuthorizeBackupDownloadResult{}, storageError(err)
	}
	if state != "ready" {
		return AuthorizeBackupDownloadResult{}, platform.NewError(platform.CodeLocked, false, "备份尚未完整就绪")
	}
	manifest, err := decodeBackupManifest(store, manifestJSON)
	if err != nil {
		return AuthorizeBackupDownloadResult{}, err
	}
	if manifest.BackupID != in.BackupId || manifest.SourceEpoch != sourceEpoch || platform.HashBytes([]byte(manifestJSON)) != manifestHash {
		return AuthorizeBackupDownloadResult{}, platform.NewError(platform.CodeIntegrityError, false, "备份目录与真实封存清单不一致")
	}
	var previousInput, raw, tokenHash, keyID string
	var consumed, revoked sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT input_hash,claims_json,token_hash,signing_key_id,consumed_at,revoked_at FROM backup_download_grants WHERE subject_id=? AND operation_id=?`, op.Actor.SubjectID, op.OperationID).Scan(&previousInput, &raw, &tokenHash, &keyID, &consumed, &revoked)
	claims := downloadClaims{Nonce: platform.NewIdentifier(), BackupID: in.BackupId, ManifestHash: manifestHash, Epoch: op.DataEpoch, Binding: binding, ExpiresAt: time.Now().UTC().Add(120 * time.Second).Unix()}
	existing := err == nil
	if existing {
		if previousInput != inputHash {
			return AuthorizeBackupDownloadResult{}, platform.NewError(platform.CodeConflict, false, "同下载操作原输入或授权绑定不同")
		}
		if json.Unmarshal([]byte(raw), &claims) != nil || claims.BackupID != in.BackupId || claims.ManifestHash != manifestHash || claims.Epoch != op.DataEpoch || claims.Binding != binding {
			return AuthorizeBackupDownloadResult{}, platform.NewError(platform.CodeIntegrityError, false, "下载授权安全载荷损坏")
		}
		if consumed.Valid || revoked.Valid || claims.ExpiresAt <= time.Now().UTC().Unix() || keyID != store.Key.KeyID {
			return AuthorizeBackupDownloadResult{}, platform.NewError(platform.CodeLocked, false, "原下载授权已消费、撤销或过期，请明确重新下载")
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return AuthorizeBackupDownloadResult{}, storageError(err)
	}
	claimsJSON, err := platform.CanonicalJSON(claims)
	if err != nil {
		return AuthorizeBackupDownloadResult{}, err
	}
	token, err := serviceauth.SignCursor(store.Key, "data-protection.download.v1", claimsJSON)
	if err != nil {
		return AuthorizeBackupDownloadResult{}, err
	}
	hash := platform.HashBytes([]byte(token))
	if existing {
		if hash != tokenHash {
			return AuthorizeBackupDownloadResult{}, platform.NewError(platform.CodeIntegrityError, false, "下载授权签名与原hash不一致")
		}
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO backup_download_grants(grant_id,subject_id,operation_id,backup_id,input_hash,claims_json,token_hash,signing_key_id,expires_at) VALUES(?,?,?,?,?,?,?,?,?)`, claims.Nonce, op.Actor.SubjectID, op.OperationID, in.BackupId, inputHash, string(claimsJSON), hash, store.Key.KeyID, time.Unix(claims.ExpiresAt, 0).UTC().Format(time.RFC3339))
		if err != nil {
			return AuthorizeBackupDownloadResult{}, storageError(err)
		}
		if err = tx.Commit(); err != nil {
			return AuthorizeBackupDownloadResult{}, storageError(err)
		}
	}
	return AuthorizeBackupDownloadResult{DownloadToken: token, ExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC()}, nil
}
