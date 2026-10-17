package media

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/media/internal/gen"
)

const maxChunkBytes = 8 << 20

type BinaryChunk struct {
	Offset int       `json:"offset"`
	Length int       `json:"length"`
	SHA256 Hash      `json:"sha256"`
	Reader io.Reader `json:"-"`
}
type PutUploadChunkInput struct {
	UploadId         Identifier
	Chunk            BinaryChunk
	ExpectedRevision gen.Revision
}
type PutUploadChunkResult = gen.PutUploadChunkResult

type chunkOperation struct {
	OperationID      Identifier
	UploadID         Identifier
	SiteID           Identifier
	ActorID          Identifier
	DataEpoch        Identifier
	RequestHash      Hash
	ExpectedRevision int
	Offset           int
	Length           int
	SHA256           Hash
	StageID          Identifier
	State            string
	Result           *gen.UploadSession
}

func (s *localBlobStore) mediaWriteContext(ctx context.Context, op OperationContext) (context.Context, context.CancelFunc, error) {
	if s.window.Runtime == nil || s.window.WriteWindow == nil {
		return nil, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体写窗口未装配")
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), s.window)
	if err != nil {
		return nil, nil, err
	}
	if view.DataEpoch != op.DataEpoch || view.DataEpoch != s.openedEpoch {
		cancel()
		return nil, nil, platform.NewError(platform.CodeVersionMismatch, false, "媒体写入不属于已打开数据代次")
	}
	if view.Maintenance || view.Phase != platform.PhaseRunning {
		cancel()
		return nil, nil, platform.NewError(platform.CodeLocked, true, "维护期间暂停媒体写入")
	}
	return work, cancel, nil
}

func hashChunkRequest(op OperationContext, input PutUploadChunkInput) (Hash, error) {
	raw, err := platform.CanonicalJSON(struct {
		Kind             string     `json:"kind"`
		ActorID          Identifier `json:"actorId"`
		SiteID           Identifier `json:"siteId"`
		DataEpoch        Identifier `json:"dataEpoch"`
		UploadID         Identifier `json:"uploadId"`
		Offset           int        `json:"offset"`
		Length           int        `json:"length"`
		SHA256           Hash       `json:"sha256"`
		ExpectedRevision int        `json:"expectedRevision"`
	}{"upload_chunk", op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, input.UploadId, input.Chunk.Offset, input.Chunk.Length, input.Chunk.SHA256, input.ExpectedRevision})
	if err != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "分块业务摘要无法编码")
	}
	return platform.HashBytes(raw), nil
}

func (s *localBlobStore) readChunkOperation(ctx context.Context, q sqlExecer, id Identifier) (chunkOperation, bool, error) {
	value := chunkOperation{OperationID: id}
	var raw, resultHash sql.NullString
	err := q.QueryRowContext(ctx, `SELECT upload_id,site_id,actor_id,data_epoch,request_hash,expected_revision,byte_offset,byte_length,sha256,stage_id,state,CASE WHEN result_json IS NULL OR length(CAST(result_json AS BLOB))<=16777216 THEN result_json ELSE NULL END,result_hash FROM upload_chunks WHERE operation_id=?`, id).Scan(&value.UploadID, &value.SiteID, &value.ActorID, &value.DataEpoch, &value.RequestHash, &value.ExpectedRevision, &value.Offset, &value.Length, &value.SHA256, &value.StageID, &value.State, &raw, &resultHash)
	if errors.Is(err, sql.ErrNoRows) {
		return value, false, nil
	}
	if err != nil {
		return value, false, mapMediaDBError(err, "读取分块回执失败")
	}
	if !platform.ValidIdentifier(value.UploadID) || !platform.ValidIdentifier(value.SiteID) || !platform.ValidIdentifier(value.ActorID) || !platform.ValidIdentifier(value.DataEpoch) || !platform.ValidHash(value.RequestHash) || !platform.ValidHash(value.SHA256) || !platform.ValidIdentifier(value.StageID) || value.ExpectedRevision < 0 || value.Offset < 0 || value.Length < 1 || value.Length > maxChunkBytes || value.Offset > int(maxUploadBytes)-value.Length {
		return value, true, platform.NewError(platform.CodeIntegrityError, false, "分块持久材料损坏")
	}
	if value.State == "COMMITTED" {
		if !raw.Valid || !resultHash.Valid || platform.HashBytes([]byte(raw.String)) != resultHash.String || s.validateUpload(json.RawMessage(raw.String)) != nil {
			return value, true, platform.NewError(platform.CodeIntegrityError, false, "分块原始结果回执损坏")
		}
		var result gen.UploadSession
		if json.Unmarshal([]byte(raw.String), &result) != nil || result.UploadId != value.UploadID || result.SiteId != value.SiteID || result.ReceivedBytes != value.Offset+value.Length || result.Revision != value.ExpectedRevision+1 || result.State != gen.Receiving {
			return value, true, platform.NewError(platform.CodeIntegrityError, false, "分块原始结果与来源不符")
		}
		value.Result = &result
	} else if value.State != "PREPARED" || raw.Valid || resultHash.Valid {
		return value, true, platform.NewError(platform.CodeIntegrityError, false, "分块持久阶段损坏")
	}
	return value, true, nil
}

type chunkContextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r chunkContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(buffer)
}

func chunkStageName(id Identifier) string { return "staging/chunk-" + id + ".part" }

// writeChunkStage 只写新私有文件，长度或SHA不符不留下PREPARED或offset。
func writeChunkStage(ctx context.Context, root *os.Root, input BinaryChunk) (Identifier, error) {
	id := platform.NewIdentifier()
	name := chunkStageName(id)
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "创建私有分块暂存失败")
	}
	ok := false
	defer func() {
		file.Close()
		if !ok {
			_ = root.Remove(name)
		}
	}()
	hasher := sha256.New()
	count, err := io.CopyBuffer(io.MultiWriter(file, hasher), io.LimitReader(chunkContextReader{ctx, input.Reader}, int64(input.Length)+1), make([]byte, hashBufferSize))
	if err != nil {
		return "", mapMediaDBError(err, "读取上传分块失败")
	}
	if count != int64(input.Length) {
		return "", platform.NewError(platform.CodeValidation, false, "分块实际长度与Content-Length不一致")
	}
	if Hash(hex.EncodeToString(hasher.Sum(nil))) != input.SHA256 {
		return "", platform.NewError(platform.CodeIntegrityError, false, "上传分块SHA不一致")
	}
	if err := file.Sync(); err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "持久化分块暂存失败")
	}
	if err := file.Close(); err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "关闭分块暂存失败")
	}
	directory, err := root.Open("staging")
	if err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "打开分块暂存目录失败")
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "持久化分块目录失败")
	}
	ok = true
	return id, nil
}

func (s *localBlobStore) reconcileUploadFile(ctx context.Context, op OperationContext, root *os.Root, record storedUpload, verify bool) (*os.File, error) {
	file, err := openMediaRegular(root, s.root, "uploads/"+record.session.UploadId+".part", os.O_RDWR)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || info.Size() < int64(record.session.ReceivedBytes) {
		file.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "已确认上传字节缺失")
	}
	if info.Size() > int64(record.session.ReceivedBytes) {
		work, cancel, err := s.mediaWriteContext(ctx, op)
		if err != nil {
			file.Close()
			return nil, err
		}
		defer cancel()
		if err := work.Err(); err != nil {
			file.Close()
			return nil, contextError(err)
		}
		if err := file.Truncate(int64(record.session.ReceivedBytes)); err != nil {
			file.Close()
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "截去未提交上传尾失败")
		}
		if err := file.Sync(); err != nil {
			file.Close()
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化上传截尾失败")
		}
	}
	if verify {
		if err := s.verifyUploadChunks(ctx, file, record); err != nil {
			file.Close()
			return nil, err
		}
	}
	return file, nil
}

func (s *localBlobStore) verifyUploadChunks(ctx context.Context, file *os.File, record storedUpload) error {
	rows, err := s.db.QueryContext(ctx, `SELECT operation_id FROM upload_chunks WHERE upload_id=? AND state='COMMITTED' ORDER BY byte_offset`, record.session.UploadId)
	if err != nil {
		return mapMediaDBError(err, "枚举已提交分块失败")
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return mapMediaDBError(err, "读取已提交分块失败")
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return mapMediaDBError(err, "枚举已提交分块失败")
	}
	offset := 0
	for revision, id := range ids {
		chunk, found, err := s.readChunkOperation(ctx, s.db, id)
		if err != nil {
			return err
		}
		if !found || chunk.Offset != offset || chunk.ExpectedRevision != revision || chunk.DataEpoch != record.dataEpoch || chunk.SiteID != record.session.SiteId || chunk.ActorID != record.createdBy {
			return platform.NewError(platform.CodeIntegrityError, false, "已提交上传分块水位不连续")
		}
		hasher := sha256.New()
		count, err := io.CopyBuffer(hasher, chunkContextReader{ctx, io.NewSectionReader(file, int64(chunk.Offset), int64(chunk.Length))}, make([]byte, hashBufferSize))
		if err != nil {
			return mapMediaDBError(err, "读取已确认上传字节失败")
		}
		if count != int64(chunk.Length) || Hash(hex.EncodeToString(hasher.Sum(nil))) != chunk.SHA256 {
			return platform.NewError(platform.CodeIntegrityError, false, "已确认上传分块完整性损坏")
		}
		offset += chunk.Length
	}
	if offset != record.session.ReceivedBytes || len(ids) != record.session.Revision {
		return platform.NewError(platform.CodeIntegrityError, false, "上传SQL水位与分块回执不符")
	}
	return nil
}

// PutUploadChunk 同操作先回放原 UploadSession，不消费分块流。
// 新写先取真实运行窗口；私有暂存长度与 SHA 一致并 fsync 后先落 PREPARED，
// 再按已提交 SQL 水位截尾、追加并 CAS。不发布资源或事件。
func PutUploadChunk(ctx context.Context, op OperationContext, input PutUploadChunkInput) (PutUploadChunkResult, error) {
	if op.Actor.Kind != "admin" || op.Actor.SessionID == "" || (!hasGrant(op.Actor.Grants, "media.upload") && !hasGrant(op.Actor.Grants, "superadmin")) {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeForbidden, false, "当前会话没有上传权限")
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储未装配")
	}
	if op.DataEpoch != s.openedEpoch {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeVersionMismatch, false, "上传不属于已打开数据代次")
	}
	hash, err := hashChunkRequest(op, input)
	if err != nil {
		return PutUploadChunkResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, found, err := s.readChunkOperation(ctx, s.db, op.OperationID)
	if err != nil {
		return PutUploadChunkResult{}, err
	}
	if found {
		if previous.RequestHash != hash || previous.ActorID != op.Actor.SubjectID || previous.SiteID != op.Actor.SiteID || previous.DataEpoch != op.DataEpoch || previous.UploadID != input.UploadId {
			return PutUploadChunkResult{}, platform.NewError(platform.CodeConflict, false, "operationId已绑定不同上传分块")
		}
		if previous.State == "COMMITTED" {
			return PutUploadChunkResult{Upload: *previous.Result}, nil
		}
	}
	var beginID string
	if err := s.db.QueryRowContext(ctx, `SELECT operation_id FROM upload_receipts WHERE operation_id=?`, op.OperationID).Scan(&beginID); err == nil {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeConflict, false, "operationId已绑定创建上传")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return PutUploadChunkResult{}, mapMediaDBError(err, "读取上传操作绑定失败")
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return PutUploadChunkResult{}, err
	}
	defer cancel()
	record, exists, err := readUploadRow(work, s.db, input.UploadId, s.validateUpload)
	if err != nil {
		return PutUploadChunkResult{}, err
	}
	if !exists {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeNotFound, false, "上传不存在")
	}
	if record.session.SiteId != op.Actor.SiteID || record.createdBy != op.Actor.SubjectID {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeForbidden, false, "只能写入本人同站点上传")
	}
	if record.dataEpoch == "" || record.dataEpoch != op.DataEpoch {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeVersionMismatch, false, "上传创建代次未知或已变化")
	}
	if record.session.State != gen.Receiving || !record.session.ExpiresAt.After(time.Now()) {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeConflict, false, "上传已结束或到期")
	}
	if record.session.Revision != input.ExpectedRevision || record.session.ReceivedBytes != input.Chunk.Offset {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeVersionMismatch, false, "上传offset或revision已变化")
	}
	if input.Chunk.Length < 1 || input.Chunk.Length > maxChunkBytes || input.Chunk.Offset > record.session.ExpectedBytes-input.Chunk.Length || input.Chunk.Reader == nil {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeValidation, false, "分块长度超过上传声明或流缺失")
	}
	root, err := s.openMediaRoot()
	if err != nil {
		return PutUploadChunkResult{}, err
	}
	defer root.Close()
	// 有界读 length+1；长度或 SHA 不符不留下 PREPARED，也不改上传文件。
	stageID, err := writeChunkStage(work, root, input.Chunk)
	if err != nil {
		return PutUploadChunkResult{}, err
	}
	prepareCtx, prepareCancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return PutUploadChunkResult{}, err
	}
	defer prepareCancel()
	var prepared sql.Result
	if !found {
		prepared, err = s.db.ExecContext(prepareCtx, `INSERT INTO upload_chunks(operation_id,upload_id,site_id,actor_id,data_epoch,request_hash,expected_revision,byte_offset,byte_length,sha256,stage_id,state,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,'PREPARED',?)`, op.OperationID, input.UploadId, op.Actor.SiteID, op.Actor.SubjectID, op.DataEpoch, hash, input.ExpectedRevision, input.Chunk.Offset, input.Chunk.Length, input.Chunk.SHA256, stageID, platform.Now())
	} else {
		prepared, err = s.db.ExecContext(prepareCtx, `UPDATE upload_chunks SET stage_id=? WHERE operation_id=? AND state='PREPARED' AND request_hash=?`, stageID, op.OperationID, hash)
	}
	if err != nil {
		return PutUploadChunkResult{}, mapMediaDBError(err, "持久化分块准备决定失败")
	}
	preparedRows, rowsErr := prepared.RowsAffected()
	if rowsErr != nil || preparedRows != 1 {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeConflict, false, "分块准备决定已变化")
	}
	// PREPARED 已提交后才按 SQL 水位截去未提交尾。崩溃恢复同样只认已提交水位。
	file, err := s.reconcileUploadFile(prepareCtx, op, root, record, false)
	if err != nil {
		return PutUploadChunkResult{}, err
	}
	defer file.Close()
	stage, err := openMediaRegular(root, s.root, chunkStageName(stageID), os.O_RDONLY)
	if err != nil {
		return PutUploadChunkResult{}, err
	}
	defer stage.Close()
	if _, err := file.Seek(int64(input.Chunk.Offset), io.SeekStart); err != nil {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "定位上传写入水位失败")
	}
	hasher := sha256.New()
	count, err := io.CopyBuffer(io.MultiWriter(file, hasher), io.LimitReader(chunkContextReader{prepareCtx, stage}, int64(input.Chunk.Length)+1), make([]byte, hashBufferSize))
	if err != nil {
		return PutUploadChunkResult{}, mapMediaDBError(err, "追加上传分块失败")
	}
	if count != int64(input.Chunk.Length) || Hash(hex.EncodeToString(hasher.Sum(nil))) != input.Chunk.SHA256 {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeIntegrityError, false, "分块暂存材料已变化")
	}
	if err := file.Sync(); err != nil {
		return PutUploadChunkResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化上传分块失败")
	}
	committed, err := s.commitChunk(ctx, op, input, record, hash)
	if err != nil {
		return PutUploadChunkResult{}, err
	}
	_ = root.Remove(chunkStageName(stageID))
	return PutUploadChunkResult{Upload: committed}, nil
}

func (s *localBlobStore) commitChunk(ctx context.Context, op OperationContext, input PutUploadChunkInput, record storedUpload, hash Hash) (gen.UploadSession, error) {
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return gen.UploadSession{}, err
	}
	defer cancel()
	if !record.session.ExpiresAt.After(time.Now()) {
		return gen.UploadSession{}, platform.NewError(platform.CodeConflict, false, "上传在分块提交前已到期")
	}
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return gen.UploadSession{}, mapMediaDBError(err, "打开分块提交事务失败")
	}
	defer tx.Rollback()
	record.session.ReceivedBytes += input.Chunk.Length
	record.session.Revision++
	raw, err := platform.CanonicalJSON(record.session)
	if err != nil {
		return gen.UploadSession{}, platform.NewError(platform.CodeIntegrityError, false, "分块原结果编码失败")
	}
	result, err := tx.ExecContext(work, `UPDATE upload_sessions SET session_json=?,updated_at=? WHERE upload_id=? AND site_id=? AND created_by=? AND data_epoch=? AND state='receiving' AND json_extract(session_json,'$.revision')=? AND json_extract(session_json,'$.receivedBytes')=?`, string(raw), platform.Now(), input.UploadId, op.Actor.SiteID, op.Actor.SubjectID, op.DataEpoch, input.ExpectedRevision, input.Chunk.Offset)
	if err != nil {
		return gen.UploadSession{}, mapMediaDBError(err, "CAS上传水位失败")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return gen.UploadSession{}, platform.NewError(platform.CodeVersionMismatch, false, "上传水位CAS已变化")
	}
	result, err = tx.ExecContext(work, `UPDATE upload_chunks SET state='COMMITTED',result_json=?,result_hash=? WHERE operation_id=? AND request_hash=? AND state='PREPARED'`, string(raw), platform.HashBytes(raw), op.OperationID, hash)
	if err != nil {
		return gen.UploadSession{}, mapMediaDBError(err, "保存分块原始回执失败")
	}
	count, err = result.RowsAffected()
	if err != nil || count != 1 {
		return gen.UploadSession{}, platform.NewError(platform.CodeConflict, false, "分块准备决定已变化")
	}
	if err := tx.Commit(); err != nil {
		return gen.UploadSession{}, mapMediaDBError(err, "提交上传分块失败")
	}
	return record.session, nil
}

// RecoverUploadChunks 只按已提交SQL水位核前缀或截尾；未提交材料不自动计数。
func RecoverUploadChunks(ctx context.Context, op OperationContext) error {
	if op.CallerService != "media" || op.Actor.Kind != "service" {
		return platform.NewError(platform.CodeForbidden, false, "分块恢复仅允许本域受信启动")
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储未装配")
	}
	if op.DataEpoch != s.openedEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "分块恢复数据代次不符")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.QueryContext(ctx, `SELECT upload_id FROM upload_sessions WHERE state='receiving' AND data_epoch IS NOT NULL ORDER BY upload_id`)
	if err != nil {
		return mapMediaDBError(err, "枚举待恢复上传失败")
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return mapMediaDBError(err, "读取待恢复上传失败")
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return mapMediaDBError(err, "枚举待恢复上传失败")
	}
	root, err := s.openMediaRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	for _, id := range ids {
		record, found, err := readUploadRow(ctx, s.db, id, s.validateUpload)
		if err != nil {
			return err
		}
		if !found || record.dataEpoch != op.DataEpoch {
			return platform.NewError(platform.CodeVersionMismatch, false, "上传恢复创建代次不符")
		}
		file, err := s.reconcileUploadFile(ctx, op, root, record, true)
		if err != nil {
			return err
		}
		file.Close()
	}
	s.chunkRecovered = true
	return nil
}
