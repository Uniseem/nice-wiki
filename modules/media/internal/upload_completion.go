package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

type CompleteUploadInput struct {
	UploadId         Identifier   `json:"uploadId"`
	ExpectedRevision gen.Revision `json:"expectedRevision"`
}
type CompleteUploadResult = gen.CompleteUploadResult

type completionProblem struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}
type completionReceipt struct {
	Version int                `json:"version"`
	Kind    string             `json:"kind"`
	Asset   *gen.Asset         `json:"asset"`
	Problem *completionProblem `json:"problem"`
}

func hashCompleteRequest(op OperationContext, input CompleteUploadInput) (Hash, error) {
	raw, err := platform.CanonicalJSON(struct {
		Kind      string              `json:"kind"`
		Caller    string              `json:"caller"`
		Actor     Identifier          `json:"actor"`
		SiteID    Identifier          `json:"siteId"`
		DataEpoch Identifier          `json:"dataEpoch"`
		Input     CompleteUploadInput `json:"input"`
	}{"complete_upload", op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, input})
	if err != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "完成上传摘要无法编码")
	}
	return platform.HashBytes(raw), nil
}

// replayCompletion 只读取首次快照；不从当前资源或已变化的会话重建结果。
func (s *localBlobStore) replayCompletion(ctx context.Context, op OperationContext, input CompleteUploadInput, hash Hash) (CompleteUploadResult, bool, error) {
	var storedHash, id string
	var raw, rawHash sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT request_hash,upload_id,CASE WHEN length(CAST(result_json AS BLOB))<=16777216 THEN result_json ELSE NULL END,result_hash FROM upload_receipts WHERE operation_id=?`, op.OperationID).Scan(&storedHash, &id, &raw, &rawHash)
	if errors.Is(err, sql.ErrNoRows) {
		return CompleteUploadResult{}, false, nil
	}
	if err != nil {
		return CompleteUploadResult{}, false, mapMediaDBError(err, "读取完成上传原回执失败")
	}
	if storedHash != hash || id != input.UploadId {
		return CompleteUploadResult{}, true, platform.NewError(platform.CodeConflict, false, "operationId已绑定不同上传材料")
	}
	record, found, err := readUploadRow(ctx, s.db, id, s.validateUpload)
	if err != nil {
		return CompleteUploadResult{}, true, err
	}
	if !found {
		return CompleteUploadResult{}, true, platform.NewError(platform.CodeIntegrityError, false, "完成原回执缺少来源上传")
	}
	if record.createdBy != op.Actor.SubjectID || record.session.SiteId != op.Actor.SiteID {
		return CompleteUploadResult{}, true, platform.NewError(platform.CodeForbidden, false, "完成原回执不属于当前主体或站点")
	}
	if record.dataEpoch == "" || record.dataEpoch != op.DataEpoch {
		return CompleteUploadResult{}, true, platform.NewError(platform.CodeVersionMismatch, false, "上传创建代次未知或已变化")
	}
	if !raw.Valid || !rawHash.Valid || platform.HashBytes([]byte(raw.String)) != rawHash.String {
		return CompleteUploadResult{}, true, platform.NewError(platform.CodeIntegrityError, false, "完成上传原结果损坏")
	}
	var receipt completionReceipt
	decoder := json.NewDecoder(strings.NewReader(raw.String))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&receipt) != nil || receipt.Version != 1 || receipt.Kind != "complete_upload" || (receipt.Asset == nil) == (receipt.Problem == nil) {
		return CompleteUploadResult{}, true, platform.NewError(platform.CodeIntegrityError, false, "完成上传原结果封套不合法")
	}
	if receipt.Problem != nil {
		p := receipt.Problem
		if !completionRejectionCode(p.Code) || p.Retryable || p.Message == "" || len(p.Message) > 1024 {
			return CompleteUploadResult{}, true, platform.NewError(platform.CodeIntegrityError, false, "完成上传原错误不合法")
		}
		return CompleteUploadResult{}, true, platform.NewError(p.Code, p.Retryable, "%s", p.Message)
	}
	asset := *receipt.Asset
	assetRaw, err := json.Marshal(asset)
	if err != nil || s.validateAsset(assetRaw) != nil || asset.SiteId != op.Actor.SiteID || asset.Ref.Sha256 != record.session.ExpectedSha256 || asset.Ref.ByteSize != record.session.ExpectedBytes || asset.Kind != gen.AssetKind(record.session.Kind) || asset.State != gen.AssetStateReady {
		return CompleteUploadResult{}, true, platform.NewError(platform.CodeIntegrityError, false, "完成上传原资源与来源不符")
	}
	return CompleteUploadResult{Asset: asset}, true, nil
}

func completionRejectionCode(code string) bool {
	switch code {
	case platform.CodeValidation, platform.CodeUnsupportedSchema, platform.CodeUnsupportedCapability, platform.CodeIntegrityError:
		return true
	}
	return false
}

// CompleteUpload 在同一上传互斥边界内完成校验、不可变文件及首次结果提交。
func CompleteUpload(ctx context.Context, op OperationContext, input CompleteUploadInput) (CompleteUploadResult, error) {
	if err := validateUploadActor(op, "media.upload"); err != nil {
		return CompleteUploadResult{}, err
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return CompleteUploadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体上传存储未装配")
	}
	if op.DataEpoch != s.openedEpoch {
		return CompleteUploadResult{}, platform.NewError(platform.CodeVersionMismatch, false, "完成上传不属于已打开活动代次")
	}
	hash, err := hashCompleteRequest(op, input)
	if err != nil {
		return CompleteUploadResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	original, found, err := s.replayCompletion(ctx, op, input, hash)
	if err != nil || found {
		return original, err
	}
	var chunk string
	if err := s.db.QueryRowContext(ctx, `SELECT operation_id FROM upload_chunks WHERE operation_id=?`, op.OperationID).Scan(&chunk); err == nil {
		return CompleteUploadResult{}, platform.NewError(platform.CodeConflict, false, "operationId已绑定上传分块")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return CompleteUploadResult{}, mapMediaDBError(err, "读取上传操作绑定失败")
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return CompleteUploadResult{}, err
	}
	defer cancel()
	record, found, err := readUploadRow(work, s.db, input.UploadId, s.validateUpload)
	if err != nil {
		return CompleteUploadResult{}, err
	}
	if !found {
		return CompleteUploadResult{}, platform.NewError(platform.CodeNotFound, false, "上传不存在")
	}
	if record.createdBy != op.Actor.SubjectID || record.session.SiteId != op.Actor.SiteID {
		return CompleteUploadResult{}, platform.NewError(platform.CodeForbidden, false, "只能完成本人同站点上传")
	}
	if record.dataEpoch == "" || record.dataEpoch != op.DataEpoch {
		return CompleteUploadResult{}, platform.NewError(platform.CodeVersionMismatch, false, "上传创建代次未知或已变化")
	}
	if record.session.State != gen.Receiving || !record.session.ExpiresAt.After(time.Now()) {
		return CompleteUploadResult{}, platform.NewError(platform.CodeConflict, false, "上传已结束或到期")
	}
	if record.session.Revision != input.ExpectedRevision {
		return CompleteUploadResult{}, platform.NewError(platform.CodeVersionMismatch, false, "上传revision已变化")
	}
	if record.session.ReceivedBytes != record.session.ExpectedBytes {
		return CompleteUploadResult{}, platform.NewError(platform.CodeConflict, false, "上传字节尚未收齐")
	}
	root, err := s.openMediaRoot()
	if err != nil {
		return CompleteUploadResult{}, err
	}
	defer root.Close()
	file, err := s.reconcileUploadFile(work, op, root, record, true)
	if err != nil {
		return CompleteUploadResult{}, err
	}
	file.Close()
	digest, err := HashUpload(work, op, HashUploadInput{UploadID: input.UploadId})
	if err != nil {
		return CompleteUploadResult{}, err
	}
	if digest.SHA256 != record.session.ExpectedSha256 || digest.ByteSize != record.session.ExpectedBytes {
		return CompleteUploadResult{}, s.rejectCompletion(ctx, op, input, record, hash, platform.NewError(platform.CodeIntegrityError, false, "上传完整SHA或长度与声明不一致"))
	}
	metadata, err := SniffAssetType(work, op, SniffAssetTypeInput{UploadID: input.UploadId})
	if err == nil {
		_, err = ValidateAssetPayload(work, op, ValidateAssetPayloadInput{UploadID: input.UploadId, Kind: metadata.Kind, MediaType: metadata.MediaType})
	}
	if err != nil {
		var problem *platform.Error
		if errors.As(err, &problem) && completionRejectionCode(problem.Code) {
			return CompleteUploadResult{}, s.rejectCompletion(ctx, op, input, record, hash, problem)
		}
		return CompleteUploadResult{}, err
	}
	result, err := CommitAssetBlob(work, op, CommitAssetBlobInput{UploadID: input.UploadId, SHA256: digest.SHA256, MediaType: metadata.MediaType, ExpectedRevision: input.ExpectedRevision, RequestHash: hash, Animated: metadata.Animated, Width: metadata.Width, Height: metadata.Height})
	return CompleteUploadResult{Asset: result.Asset}, err
}

func (s *localBlobStore) rejectCompletion(ctx context.Context, op OperationContext, input CompleteUploadInput, record storedUpload, hash Hash, problem *platform.Error) error {
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return mapMediaDBError(err, "开始上传拒绝事务失败")
	}
	defer tx.Rollback()
	record.session.State = gen.UploadSessionState("blocked")
	record.session.Revision++
	if err := s.casCompleteSession(work, tx, op, input, record); err != nil {
		return err
	}
	receipt := completionReceipt{Version: 1, Kind: "complete_upload", Problem: &completionProblem{Code: problem.Code, Message: problem.Message, Retryable: false}}
	if err := writeCompletionReceipt(work, tx, op, input.UploadId, hash, receipt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return mapMediaDBError(err, "提交上传拒绝结果失败")
	}
	return problem
}

func (s *localBlobStore) casCompleteSession(ctx context.Context, tx *sql.Tx, op OperationContext, input CompleteUploadInput, record storedUpload) error {
	if !record.session.ExpiresAt.After(time.Now()) {
		return platform.NewError(platform.CodeConflict, false, "上传在完成提交前已到期")
	}
	raw, err := platform.CanonicalJSON(record.session)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "完成上传状态无法编码")
	}
	result, err := tx.ExecContext(ctx, `UPDATE upload_sessions SET state=?,session_json=?,updated_at=? WHERE upload_id=? AND site_id=? AND created_by=? AND data_epoch=? AND state='receiving' AND json_extract(session_json,'$.revision')=? AND json_extract(session_json,'$.receivedBytes')=expected_bytes`, record.session.State, string(raw), platform.Now(), input.UploadId, op.Actor.SiteID, op.Actor.SubjectID, op.DataEpoch, input.ExpectedRevision)
	if err != nil {
		return mapMediaDBError(err, "CAS完成上传失败")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return platform.NewError(platform.CodeVersionMismatch, false, "上传状态或revision已变化")
	}
	return nil
}

func writeCompletionReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, uploadID Identifier, hash Hash, receipt completionReceipt) error {
	raw, err := platform.CanonicalJSON(receipt)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "完成上传原结果无法编码")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO upload_receipts(operation_id,request_hash,upload_id,created_at,result_json,result_hash) VALUES(?,?,?,?,?,?)`, op.OperationID, hash, uploadID, platform.Now(), string(raw), platform.HashBytes(raw)); err != nil {
		return mapMediaDBError(err, "写入完成上传原回执失败")
	}
	return nil
}
