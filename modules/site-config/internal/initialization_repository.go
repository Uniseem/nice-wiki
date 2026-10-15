package siteconfig

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

// ReadTransaction 只允许读取本域active事务，不跨域暴露数据库句柄。
type ReadTransaction = queryer

type PersistInitializationDecisionInput struct {
	State            gen.InitializationState `json:"state"`
	Decision         string                  `json:"decision"`
	ExpectedRevision gen.Revision            `json:"expectedRevision"`
	RequestHash gen.Hash `json:"requestHash"`
 PreparedConfig   *gen.SiteConfig         `json:"preparedConfig"`
}

type PersistInitializationDecisionResult struct {
	State gen.InitializationState `json:"state"`
}

// PersistInitializationDecision 仅供本域已授权用例调用。决定和完整准备
// 配置在同一FULL同步事务提交，返回后仍须由恢复用例提交settings权威文件。
func PersistInitializationDecision(ctx context.Context, op platform.OperationContext, input PersistInitializationDecisionInput) (PersistInitializationDecisionResult, error) {
	if input.State.OperationId == nil || input.State.SiteId == nil {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeConflict, false, "初始化决定缺少协调标识")
	}
	if op.Actor.SiteID != "" && op.Actor.SiteID != *input.State.SiteId {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeForbidden, false, "初始化站点与委托主体不一致")
	}
	prepared, configHash, err := prepareInitializationBytes(input)
	if err != nil {
		return PersistInitializationDecisionResult{}, err
	}
	requestHash, err := hashInput(struct {
		OperationID string `json:"operationId"`
		SiteID      string `json:"siteId"`
		Decision    string `json:"decision"`
		ConfigHash  string `json:"configHash"`
	}{*input.State.OperationId, *input.State.SiteId, input.Decision, configHash})
	if err != nil {
		return PersistInitializationDecisionResult{}, err
	}
	store := currentActiveStore()
	if store == nil {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开初始化决定事务失败")
	}
	defer tx.Rollback()
	var dataEpoch string
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch FROM site_config_state WHERE singleton=1`).Scan(&dataEpoch); err != nil {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "active初始化水位不可读")
	}
	if op.DataEpoch != dataEpoch {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeVersionMismatch, false, "初始化dataEpoch已改变")
	}
	if state, found, err := readInitializationDecision(ctx, tx, *input.State.OperationId, requestHash); err != nil {
		return PersistInitializationDecisionResult{}, err
    } else if found {
        var actorID,siteID,epoch,originalHash string
        err := tx.QueryRowContext(ctx,`SELECT actor_id,site_id,data_epoch,request_hash FROM config_operations WHERE operation_id=?`,*state.OperationId).Scan(&actorID,&siteID,&epoch,&originalHash)
        if err != nil { return PersistInitializationDecisionResult{},platform.NewError(platform.CodeIntegrityError,false,"初始化决定缺少原始身份回执") }
        if actorID != op.Actor.SubjectID || siteID != *input.State.SiteId || epoch != op.DataEpoch || originalHash != input.RequestHash {
            return PersistInitializationDecisionResult{},platform.NewError(platform.CodeConflict,false,"初始化operationId被不同请求或主体复用")
        }
        return PersistInitializationDecisionResult{State: state}, nil
    }
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return PersistInitializationDecisionResult{}, err
	}
	current, initialized, err := store.ReadInitializationRecord(ctx, tx)
	if err != nil {
		return PersistInitializationDecisionResult{}, err
	}
	if current.Revision != input.ExpectedRevision || input.State.Revision != input.ExpectedRevision {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeVersionMismatch, false, "初始化revision已改变")
	}
	if initialized || current.Decision == gen.Commit {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeConflict, false, "已有不可反悔的初始化commit决定")
	}
	if current.SiteId != nil && *current.SiteId != *input.State.SiteId {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeConflict, false, "初始化站点已被保留")
	}
	if current.OperationId != nil && *current.OperationId == *input.State.OperationId {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "当前初始化状态缺少决定回执")
	}
	if input.State.Phase != gen.InitializationStatePhasePreparing || input.State.Decision != gen.None || input.State.ConfigRevision != 0 || input.State.SafeError != nil {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeConflict, false, "初始化准备阶段不完整")
	}
	next := input.State
	next.Decision = gen.InitializationStateDecision(input.Decision)
	next.Revision = current.Revision + 1
	next.UpdatedAt = time.Now().UTC()
	if input.Decision == "abort" {
		next.Phase = gen.InitializationStatePhaseAborted
	}
	stateJSON, err := json.Marshal(next)
	if err != nil {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeInternal, false, "编码初始化决定失败")
	}
	var preparedValue, hashValue any
	if prepared != nil {
		preparedValue, hashValue = string(prepared), configHash
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO initialization_decisions
		(operation_id,site_id,decision,request_hash,decision_state_json,prepared_config_json,prepared_config_hash,created_at)
		VALUES(?,?,?,?,?,?,?,?)`, *next.OperationId, *next.SiteId, input.Decision, requestHash, string(stateJSON), preparedValue, hashValue, next.UpdatedAt.Format(time.RFC3339Nano)); err != nil {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化初始化决定失败")
	}
    revisionID := initializationRevisionID(op.DataEpoch, *next.SiteId, *next.OperationId)
    operationState := "in_progress"
    if input.Decision == "abort" { operationState = "committed" }
    if _, err := tx.ExecContext(ctx, `INSERT INTO config_operations(operation_id,actor_id,site_id,data_epoch,request_hash,state,revision_id) VALUES(?,?,?,?,?,?,?)`, *next.OperationId,op.Actor.SubjectID,*next.SiteId,op.DataEpoch,input.RequestHash,operationState,revisionID); err != nil {
        return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeDependencyUnavailable,true,"持久化初始化原始请求回执失败")
    }
    if input.Decision == "abort" {
        if _,err := tx.ExecContext(ctx,`UPDATE config_operations SET result_state_json=? WHERE operation_id=?`,string(stateJSON),*next.OperationId); err != nil {
            return PersistInitializationDecisionResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"保存取消初始化回执失败")
        }
    }
	if current.Revision == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO initialization_state(singleton,operation_id,revision,state_json) VALUES(1,?,?,?)`, *next.OperationId, next.Revision, string(stateJSON))
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `UPDATE initialization_state SET operation_id=?,revision=?,state_json=? WHERE singleton=1 AND revision=?`, *next.OperationId, next.Revision, string(stateJSON), input.ExpectedRevision)
		if err == nil {
			count, countErr := result.RowsAffected()
			if countErr != nil || count != 1 {
				return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeVersionMismatch, false, "初始化revision已改变")
			}
		}
	}
	if err != nil {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化初始化状态失败")
	}
	if err := tx.Commit(); err != nil {
		return PersistInitializationDecisionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "提交初始化决定失败")
	}
	return PersistInitializationDecisionResult{State: next}, nil
}

// ReadInitializationRecord 在调用方事务内读取完整协调状态；公开字段裁剪
// 留给ReadInitializationStatus，数据库未知字段或水位矛盾一律失败关闭。
func (s *ActiveStore) ReadInitializationRecord(ctx context.Context, q ReadTransaction) (gen.InitializationState, bool, error) {
	var initialized, configRevision int64
	var updatedAt string
	if err := q.QueryRowContext(ctx, `SELECT initialized,config_revision,updated_at FROM site_config_state WHERE singleton=1`).Scan(&initialized, &configRevision, &updatedAt); err != nil {
		return gen.InitializationState{}, false, platform.NewError(platform.CodeDependencyUnavailable, true, "读取初始化水位失败")
	}
	var operationID, stateJSON, siteID, decision string
	var revision int64
	var configJSON, configHash sql.NullString
	err := q.QueryRowContext(ctx, `SELECT s.operation_id,s.revision,s.state_json,d.site_id,d.decision,d.prepared_config_json,d.prepared_config_hash FROM initialization_state s LEFT JOIN initialization_decisions d ON d.operation_id=s.operation_id WHERE s.singleton=1`).Scan(&operationID, &revision, &stateJSON, &siteID, &decision, &configJSON, &configHash)
	if errors.Is(err, sql.ErrNoRows) {
		if initialized != 0 || configRevision != 0 {
			return gen.InitializationState{}, false, platform.NewError(platform.CodeIntegrityError, false, "已初始化配置缺少协调记录")
		}
		createdAt, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return gen.InitializationState{}, false, platform.NewError(platform.CodeIntegrityError, false, "初始化时间不合法")
		}
		return gen.InitializationState{Phase: gen.InitializationStatePhaseUninitialized, Decision: gen.None, UpdatedAt: createdAt}, false, nil
	}
	if err != nil {
		return gen.InitializationState{}, false, platform.NewError(platform.CodeDependencyUnavailable, true, "读取初始化协调状态失败")
	}
	state, err := decodeInitializationState(stateJSON)
	if err != nil {
		return gen.InitializationState{}, false, err
	}
	if err := validateInitializationMaterial(state, operationID, siteID, decision, configJSON, configHash); err != nil {
		return gen.InitializationState{}, false, err
	}
	if (initialized != 0 && initialized != 1) || state.Revision != gen.Revision(revision) || (state.ConfigRevision != gen.Revision(configRevision) && (state.Phase != gen.InitializationStatePhaseCompleted || state.ConfigRevision > gen.Revision(configRevision))) || (state.Phase == gen.InitializationStatePhaseCompleted) != (initialized == 1) {
		return gen.InitializationState{}, false, platform.NewError(platform.CodeIntegrityError, false, "初始化协调状态与active水位不一致")
	}
	return state, initialized == 1, nil
}

func prepareInitializationBytes(input PersistInitializationDecisionInput) ([]byte, string, error) {
	if input.Decision == "abort" {
		if input.PreparedConfig != nil {
			return nil, "", platform.NewError(platform.CodeConflict, false, "abort不得附准备配置")
		}
		return nil, "", nil
	}
	if input.Decision != "commit" || input.PreparedConfig == nil {
		return nil, "", platform.NewError(platform.CodeConflict, false, "commit缺少完整准备配置")
	}
	config := input.PreparedConfig
	if config.SiteId != *input.State.SiteId || config.Initialized || config.Revision != 0 || config.Home.Revision != 0 {
		return nil, "", platform.NewError(platform.CodeConflict, false, "准备配置与初始化状态不一致")
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "准备配置无法序列化")
	}
	// 联合类型内部保存RawMessage；重新按map排序所有对象键，保证字段
	// 顺序变化不会改变同一准备配置的幂等摘要。
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "准备配置无法规范序列化")
	}
	raw, err = json.Marshal(value)
	if err != nil {
		return nil, "", platform.NewError(platform.CodeIntegrityError, false, "准备配置无法规范序列化")
	}
	return raw, platform.HashBytes(raw), nil
}

func readInitializationDecision(ctx context.Context, q ReadTransaction, operationID, requestHash string) (gen.InitializationState, bool, error) {
	var storedHash, stateJSON, siteID, decision string
	var configJSON, configHash sql.NullString
	err := q.QueryRowContext(ctx, `SELECT request_hash,decision_state_json,site_id,decision,prepared_config_json,prepared_config_hash FROM initialization_decisions WHERE operation_id=?`, operationID).Scan(&storedHash, &stateJSON, &siteID, &decision, &configJSON, &configHash)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.InitializationState{}, false, nil
	}
	if err != nil {
		return gen.InitializationState{}, false, platform.NewError(platform.CodeDependencyUnavailable, true, "读取初始化决定回执失败")
	}
	if storedHash != requestHash {
		return gen.InitializationState{}, false, platform.NewError(platform.CodeConflict, false, "初始化operationId被不同请求复用")
	}
	state, err := decodeInitializationState(stateJSON)
	if err != nil {
		return gen.InitializationState{}, false, err
	}
	if err := validateInitializationMaterial(state, operationID, siteID, decision, configJSON, configHash); err != nil {
		return gen.InitializationState{}, false, err
	}
	return state, true, nil
}

func validateInitializationMaterial(state gen.InitializationState, operationID, siteID, decision string, configJSON, configHash sql.NullString) error {
	if state.OperationId == nil || *state.OperationId != operationID || state.SiteId == nil || *state.SiteId != siteID || string(state.Decision) != decision || (state.Decision == gen.Commit && (!configJSON.Valid || !configHash.Valid || platform.HashBytes([]byte(configJSON.String)) != configHash.String)) || (state.Decision == gen.Abort && (configJSON.Valid || configHash.Valid)) {
		return platform.NewError(platform.CodeIntegrityError, false, "初始化决定与准备配置哈希不一致")
	}
	return nil
}

func decodeInitializationState(raw string) (gen.InitializationState, error) {
	var state gen.InitializationState
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return state, platform.NewError(platform.CodeIntegrityError, false, "初始化状态格式不合法")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return state, platform.NewError(platform.CodeIntegrityError, false, "初始化状态包含多余内容")
	}
	if state.OperationId == nil || !platform.ValidIdentifier(*state.OperationId) || state.SiteId == nil || !platform.ValidIdentifier(*state.SiteId) || state.Revision < 1 || state.ConfigRevision < 0 || state.UpdatedAt.IsZero() {
		return state, platform.NewError(platform.CodeIntegrityError, false, "初始化状态字段不合法")
	}
	switch state.Decision {
	case gen.Commit:
		if state.Phase != gen.InitializationStatePhasePreparing && state.Phase != gen.InitializationStatePhaseConfigCommitted && state.Phase != gen.InitializationStatePhaseCompleted && state.Phase != gen.InitializationStatePhaseBlocked {
			return state, platform.NewError(platform.CodeIntegrityError, false, "初始化commit阶段不合法")
		}
	case gen.Abort:
		if state.Phase != gen.InitializationStatePhaseAborting && state.Phase != gen.InitializationStatePhaseAborted {
			return state, platform.NewError(platform.CodeIntegrityError, false, "初始化abort阶段不合法")
		}
	default:
		return state, platform.NewError(platform.CodeIntegrityError, false, "初始化决定不合法")
	}
	return state, nil
}
