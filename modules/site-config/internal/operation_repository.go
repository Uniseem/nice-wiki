package siteconfig

import (
	"context"
	"database/sql"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

type ResolveConfigOperationInput struct {
	OperationId gen.Identifier `json:"operationId"`
	RequestHash gen.Hash       `json:"requestHash"`
}

type ResolveConfigOperationResult struct {
	State          string        `json:"state"`
	ResultRevision *gen.Revision `json:"resultRevision"`
}

// ResolveConfigOperation 按原始actor/site/op读取回执；重放不能换请求或主体。
func ResolveConfigOperation(ctx context.Context, op platform.OperationContext, input ResolveConfigOperationInput) (ResolveConfigOperationResult, error) {
	store := currentActiveStore()
	if store == nil {
		return ResolveConfigOperationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	var actorID, siteID, epoch, requestHash, state string
	var revision sql.NullInt64
	err := store.db.QueryRowContext(ctx, `SELECT actor_id,site_id,data_epoch,request_hash,state,result_revision FROM config_operations WHERE operation_id=?`, input.OperationId).Scan(&actorID, &siteID, &epoch, &requestHash, &state, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return ResolveConfigOperationResult{State: "absent"}, nil
	}
	if err != nil {
		return ResolveConfigOperationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取配置操作回执失败")
	}
	if actorID != op.Actor.SubjectID || siteID != op.Actor.SiteID || requestHash != input.RequestHash {
		return ResolveConfigOperationResult{}, platform.NewError(platform.CodeConflict, false, "配置operationId被不同请求或主体复用")
	}
	if epoch != op.DataEpoch {
		return ResolveConfigOperationResult{}, platform.NewError(platform.CodeVersionMismatch, false, "配置操作属于其他dataEpoch")
	}
	result := ResolveConfigOperationResult{State: state}
	if revision.Valid {
		value := gen.Revision(revision.Int64)
		result.ResultRevision = &value
	}
	return result, nil
}

// configCandidate 是未完成普通配置操作的固定恢复材料，不是正式配置。
type configCandidate struct {
	Config     gen.SiteConfig
	RevisionID gen.Identifier
}

func (s *ActiveStore) readConfigCandidate(ctx context.Context, q ReadTransaction, operationID, kind string) (configCandidate, error) {
	var storedKind, hash, revisionID string
	var raw sql.NullString
	var revision int
	err := q.QueryRowContext(ctx, `SELECT m.kind,m.config_hash,CASE WHEN length(CAST(m.config_json AS BLOB))<=? THEN m.config_json ELSE NULL END,m.config_revision,o.revision_id FROM config_operation_materials m JOIN config_operations o ON o.operation_id=m.operation_id WHERE m.operation_id=?`, maxConfigFileBytes, operationID).Scan(&storedKind, &hash, &raw, &revision, &revisionID)
	if err != nil || !raw.Valid || storedKind != kind || platform.HashBytes([]byte(raw.String)) != hash {
		return configCandidate{}, platform.NewError(platform.CodeIntegrityError, false, "配置操作候选材料缺失或hash不一致")
	}
	object := gen.ObjectRef{Owner: "site-config", ObjectId: revisionID, Version: revision, Sha256: hash, ByteSize: len(raw.String), MediaType: "application/json"}
	config, err := decodeConfigDocument([]byte(raw.String), object)
	if err != nil {
		return configCandidate{}, err
	}
	return configCandidate{Config: config, RevisionID: revisionID}, nil
}

func (s *ActiveStore) reserveConfigCandidate(ctx context.Context, op platform.OperationContext, kind, requestHash string, config gen.SiteConfig) (configCandidate, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return configCandidate{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开配置候选事务失败")
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE site_config_state SET singleton=singleton WHERE singleton=1`); err != nil {
		return configCandidate{}, platform.NewError(platform.CodeDependencyUnavailable, true, "取得配置候选写锁失败")
	}
	var actorID, siteID, epoch, hash string
	err = tx.QueryRowContext(ctx, `SELECT actor_id,site_id,data_epoch,request_hash FROM config_operations WHERE operation_id=?`, op.OperationID).Scan(&actorID, &siteID, &epoch, &hash)
	if err == nil {
		if actorID != op.Actor.SubjectID || siteID != op.Actor.SiteID || epoch != op.DataEpoch || hash != requestHash {
			return configCandidate{}, platform.NewError(platform.CodeConflict, false, "配置operationId被不同请求或主体复用")
		}
		return s.readConfigCandidate(ctx, tx, op.OperationID, kind)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return configCandidate{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取配置候选回执失败")
	}
	siteID, current, _, err := s.readConfigWatermark(ctx, op, tx)
	if err != nil {
		return configCandidate{}, err
	}
	if siteID != config.SiteId || current+1 != config.Revision {
		return configCandidate{}, platform.NewError(platform.CodeVersionMismatch, false, "配置候选CAS已变化")
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return configCandidate{}, err
	}
	raw, err := canonicalConfigJSON(configDocument{SchemaVersion: configSchemaVersion, Config: config})
	if err != nil {
		return configCandidate{}, err
	}
	if len(raw) > maxConfigFileBytes {
		return configCandidate{}, platform.NewError(platform.CodeValidation, false, "配置候选材料超过16MiB上限")
	}
	revisionID := configRevisionID(op.DataEpoch, config.SiteId, op.OperationID)
	if _, err := tx.ExecContext(ctx, `INSERT INTO config_operations(operation_id,actor_id,site_id,data_epoch,request_hash,state,revision_id) VALUES(?,?,?,?,?,'in_progress',?)`, op.OperationID, op.Actor.SubjectID, config.SiteId, op.DataEpoch, requestHash, revisionID); err != nil {
		return configCandidate{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化配置原始操作失败")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO config_operation_materials(operation_id,kind,config_json,config_hash,config_revision) VALUES(?,?,?,?,?)`, op.OperationID, kind, string(raw), platform.HashBytes(raw), config.Revision); err != nil {
		return configCandidate{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化配置候选材料失败")
	}
	if kind == "home" || kind == "branding" {
		if err := saveReferenceIntent(ctx, op, tx, kind, config, revisionID); err != nil {
			return configCandidate{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return configCandidate{}, platform.NewError(platform.CodeDependencyUnavailable, true, "提交配置候选材料失败")
	}
	return configCandidate{Config: config, RevisionID: revisionID}, nil
}
