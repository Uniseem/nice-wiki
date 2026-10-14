package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

// CommitAssetBlobInput 是已完成校验的上传对象提交输入。
type CommitAssetBlobInput struct {
	UploadID  Identifier `json:"uploadId"`
	SHA256    Hash       `json:"sha256"`
	MediaType string     `json:"mediaType"`
}

// CommitAssetBlobResult 是不可变资源提交结果。
type CommitAssetBlobResult struct {
	Asset gen.Asset `json:"asset"`
}

// blobCommitRequest 是私有存储提交边界；它不暴露操作系统路径。
// 实现必须在同一 site 的 sha256 命名空间内核对已有对象字节，临时对象
// fsync 后执行同文件系统 rename 和目录 fsync，并在元数据事务成功前不返回
// ready 资源。具体本地目录、数据库和 journal 由后续存储装配提供。
type blobCommitRequest struct {
	SiteID    Identifier
	UploadID  Identifier
	SHA256    Hash
	MediaType string
	ByteSize  int
}

// blobStore 是 media 私有存储与本函数之间的最小边界。它不允许调用方传入
// 任意路径，也不连接外部对象存储；真实实现由本域后续存储工作包装配。
type blobStore interface {
	OpenUpload(ctx context.Context, uploadID Identifier) (ioReadCloser, error)
	CommitVerifiedBlob(ctx context.Context, request blobCommitRequest) (gen.Asset, error)
}

// ioReadCloser 只保留 HashUpload 所需的受限读取能力，避免把文件句柄或路径
// 泄露到函数合同之外。
type ioReadCloser interface {
	Read([]byte) (int, error)
	Close() error
}

// localBlobStoreState 是本域 active 区的权威元数据快照。它只保存公开
// 资源元数据和上传会话，不保存临时路径、密钥或请求正文。
type localBlobStoreState struct {
	SchemaVersion int                           `json:"schemaVersion"`
	Uploads       map[string]localUploadRecord  `json:"uploads"`
	Assets        map[string]gen.Asset          `json:"assets"`
	Receipts      map[string]localUploadReceipt `json:"receipts"`
}

type localUploadRecord struct {
	Session   gen.UploadSession `json:"session"`
	CreatedBy Identifier        `json:"createdBy"`
}

type localUploadReceipt struct {
	RequestHash Hash             `json:"requestHash"`
	UploadID    Identifier       `json:"uploadId"`
	Session     gen.UploadSession `json:"session"`
}

const localBlobStoreSchemaVersion = 1

type uploadQuota struct {
	MaxUploadBytes       int
	SiteStorageBytes     int64
	MaxUploadConcurrency int
}

const (
	blobJournalPrepared  = "PREPARED"
	blobJournalCommitted = "COMMITTED"
	blobJournalDeleting  = "DELETING"
)

type blobJournalEvent struct {
	SchemaVersion int        `json:"schemaVersion"`
	Event         string     `json:"event"`
	SiteID        Identifier `json:"siteId"`
	UploadID      Identifier `json:"uploadId"`
	SHA256        Hash       `json:"sha256"`
	AssetID       Identifier `json:"assetId,omitempty"`
	OccurredAt    time.Time  `json:"occurredAt"`
}

// localBlobStore 负责 media active 区的本地权威文件和元数据。所有路径
// 都由受约束的 ID/hash 生成，调用方不能传入任意操作系统路径。
type localBlobStore struct {
	root       string
	statePath  string
	journalPath string
	mu         sync.Mutex
	state      localBlobStoreState
}

// activeBlobStore 由本域存储装配在接受业务请求前设置；nil 时所有依赖该
// 存储的函数都返回 DEPENDENCY_UNAVAILABLE，不能伪造提交成功。
var (
	activeBlobStore   blobStore
	activeBlobStoreMu sync.RWMutex
)

// InstallLocalBlobStore 打开本域 active 私有目录并装配不可变 blob 边界。
// 该入口只接受服务配置生成的数据根，不接受请求携带的路径。
func InstallLocalBlobStore(dataRoot string) error {
	if !filepath.IsAbs(dataRoot) {
		return platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	store, err := openLocalBlobStore(filepath.Join(dataRoot, "active", "media"))
	if err != nil {
		return err
	}
	activeBlobStoreMu.Lock()
	activeBlobStore = store
	activeBlobStoreMu.Unlock()
	return nil
}

func currentBlobStore() blobStore {
	activeBlobStoreMu.RLock()
	defer activeBlobStoreMu.RUnlock()
	return activeBlobStore
}

func openLocalBlobStore(root string) (*localBlobStore, error) {
	if !filepath.IsAbs(root) {
		return nil, platform.NewError(platform.CodeValidation, false, "媒体存储根必须为绝对路径")
	}
	for _, directory := range []string{"blobs", "uploads", "staging", "isolated"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "建立媒体私有目录失败")
		}
	}
	store := &localBlobStore{
		root:        root,
		statePath:   filepath.Join(root, "metadata.json"),
		journalPath: filepath.Join(root, "blob-journal.jsonl"),
		state: localBlobStoreState{
			SchemaVersion: localBlobStoreSchemaVersion,
			Uploads:       make(map[string]localUploadRecord),
			Assets:        make(map[string]gen.Asset),
			Receipts:      make(map[string]localUploadReceipt),
		},
	}
	if raw, err := os.ReadFile(store.statePath); err == nil {
		if err := json.Unmarshal(raw, &store.state); err != nil ||
			store.state.SchemaVersion != localBlobStoreSchemaVersion ||
			store.state.Uploads == nil || store.state.Assets == nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "媒体元数据schema或内容不合法")
		}
		if store.state.Receipts == nil {
			store.state.Receipts = make(map[string]localUploadReceipt)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "读取媒体元数据失败")
	} else if err := store.persistStateLocked(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *localBlobStore) persistStateLocked() error {
	raw, err := json.Marshal(s.state)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "序列化媒体元数据失败")
	}
	temporary, err := os.CreateTemp(s.root, ".metadata-*")
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "创建媒体元数据暂存失败")
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryName)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "设置媒体元数据权限失败")
	}
	if _, err := temporary.Write(raw); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "写入媒体元数据失败")
	}
	if err := temporary.Sync(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "持久化媒体元数据失败")
	}
	if err := temporary.Close(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "关闭媒体元数据暂存失败")
	}
	if err := os.Rename(temporaryName, s.statePath); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "提交媒体元数据失败")
	}
	if err := syncDirectory(s.root); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "持久化媒体目录失败")
	}
	return nil
}

func (s *localBlobStore) appendJournalLocked(event blobJournalEvent) error {
	if event.SchemaVersion == 0 {
		event.SchemaVersion = localBlobStoreSchemaVersion
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "序列化媒体journal失败")
	}
	file, err := os.OpenFile(s.journalPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "打开媒体journal失败")
	}
	defer file.Close()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "写入媒体journal失败")
	}
	if err := file.Sync(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "持久化媒体journal失败")
	}
	return nil
}

func (s *localBlobStore) uploadPath(uploadID Identifier) string {
	return filepath.Join(s.root, "uploads", uploadID+".part")
}

func (s *localBlobStore) stagingPath(uploadID Identifier, sha256Hash Hash) string {
	return filepath.Join(s.root, "staging", uploadID+"-"+sha256Hash+".part")
}

func (s *localBlobStore) blobPath(siteID Identifier, sha256Hash Hash) string {
	return filepath.Join(s.root, "blobs", siteID, sha256Hash[:2], sha256Hash)
}

func (s *localBlobStore) isolatedPath(prefix, uploadID Identifier, sha256Hash Hash) string {
	return filepath.Join(s.root, "isolated", prefix+"-"+uploadID+"-"+sha256Hash)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func hashFile(path string) (Hash, int64, error) {
	lstat, err := os.Lstat(path)
	if err != nil || !lstat.Mode().IsRegular() {
		return "", 0, errors.New("非普通文件")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", 0, errors.New("非普通文件")
	}
	hasher := sha256.New()
	buffer := make([]byte, hashBufferSize)
	var size int64
	for {
		readSize, readErr := file.Read(buffer)
		if readSize < 0 || readSize > len(buffer) {
			return "", 0, errors.New("文件读取结果不合法")
		}
		if readSize > 0 {
			if _, err := hasher.Write(buffer[:readSize]); err != nil {
				return "", 0, err
			}
			size += int64(readSize)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return "", 0, readErr
		}
		if readSize == 0 {
			return "", 0, errors.New("文件读取无进展")
		}
	}
	return Hash(hex.EncodeToString(hasher.Sum(nil))), size, nil
}

func copyAndSync(sourcePath, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = destination.Close()
		if !ok {
			_ = os.Remove(destinationPath)
		}
	}()
	buffer := make([]byte, hashBufferSize)
	if _, err := io.CopyBuffer(destination, source, buffer); err != nil {
		return err
	}
	if err := destination.Sync(); err != nil {
		return err
	}
	if err := destination.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func (s *localBlobStore) OpenUpload(ctx context.Context, uploadID Identifier) (ioReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, contextError(err)
	}
	if !validIdentifier(uploadID) {
		return nil, platform.NewError(platform.CodeValidation, false, "上传标识不合法")
	}
	s.mu.Lock()
	_, exists := s.state.Uploads[string(uploadID)]
	s.mu.Unlock()
	if !exists {
		return nil, platform.NewError(platform.CodeNotFound, false, "上传不存在")
	}
	file, err := os.Open(s.uploadPath(uploadID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "上传临时对象缺失")
		}
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "打开上传临时对象失败")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "上传临时对象不是普通文件")
	}
	return file, nil
}

// CreateUpload 为后续上传 handler 提供私有 receiving 会话落点；会话和
// 临时文件必须同时成功，否则不留下可被恢复误认的半成品。
func (s *localBlobStore) CreateUpload(ctx context.Context, session gen.UploadSession, createdBy Identifier, quota uploadQuota, operationID Identifier, requestHash Hash) (gen.UploadSession, error) {
	if err := ctx.Err(); err != nil {
		return gen.UploadSession{}, contextError(err)
	}
	if !validIdentifier(session.UploadId) || !validIdentifier(session.SiteId) ||
		!validIdentifier(createdBy) || !validHash(session.ExpectedSha256) ||
		session.ExpectedBytes < 1 || session.ExpectedBytes > int(maxUploadBytes) ||
		session.ReceivedBytes != 0 || session.State != gen.Receiving ||
		session.OriginalName == "" || session.ExpiresAt.IsZero() ||
		quota.MaxUploadBytes < 1 || quota.SiteStorageBytes < 1 || quota.MaxUploadConcurrency < 1 ||
		session.ExpectedBytes > quota.MaxUploadBytes || !validIdentifier(operationID) || !validHash(requestHash) {
		return gen.UploadSession{}, platform.NewError(platform.CodeValidation, false, "上传会话参数不合法")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if receipt, exists := s.state.Receipts[string(operationID)]; exists {
		if receipt.RequestHash != requestHash {
			return gen.UploadSession{}, platform.NewError(platform.CodeConflict, false, "operationId已用于其他上传请求")
		}
		if stored, ok := s.state.Uploads[string(receipt.UploadID)]; ok {
			return stored.Session, nil
		}
		return gen.UploadSession{}, platform.NewError(platform.CodeIntegrityError, false, "上传回执缺少会话")
	}
	if _, exists := s.state.Uploads[string(session.UploadId)]; exists {
		return gen.UploadSession{}, platform.NewError(platform.CodeConflict, false, "上传会话已存在")
	}
	var receivingCount int
	var reservedBytes int64
	for _, existing := range s.state.Uploads {
		if existing.Session.SiteId != session.SiteId || existing.Session.State != gen.Receiving {
			continue
		}
		receivingCount++
		reservedBytes += int64(existing.Session.ExpectedBytes)
	}
	for _, asset := range s.state.Assets {
		if asset.SiteId == session.SiteId && asset.State == gen.AssetStateReady {
			reservedBytes += int64(asset.Ref.ByteSize)
		}
	}
	if receivingCount >= quota.MaxUploadConcurrency {
		return gen.UploadSession{}, platform.NewError(platform.CodeConflict, false, "上传并发配额已满")
	}
	if reservedBytes > quota.SiteStorageBytes-int64(session.ExpectedBytes) {
		return gen.UploadSession{}, platform.NewError(platform.CodeConflict, false, "站点存储配额已满")
	}
	file, err := os.OpenFile(s.uploadPath(session.UploadId), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return gen.UploadSession{}, platform.NewError(platform.CodeConflict, false, "上传临时对象已存在")
		}
		return gen.UploadSession{}, platform.NewError(platform.CodeDependencyUnavailable, true, "创建上传临时对象失败")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(s.uploadPath(session.UploadId))
		return gen.UploadSession{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化上传临时对象失败")
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(s.uploadPath(session.UploadId))
		return gen.UploadSession{}, platform.NewError(platform.CodeDependencyUnavailable, true, "关闭上传临时对象失败")
	}
	s.state.Uploads[string(session.UploadId)] = localUploadRecord{Session: session, CreatedBy: createdBy}
	s.state.Receipts[string(operationID)] = localUploadReceipt{RequestHash: requestHash, UploadID: session.UploadId, Session: session}
	if err := s.persistStateLocked(); err != nil {
		delete(s.state.Uploads, string(session.UploadId))
		delete(s.state.Receipts, string(operationID))
		_ = os.Remove(s.uploadPath(session.UploadId))
		return gen.UploadSession{}, err
	}
	return session, nil
}

func (s *localBlobStore) ReadUpload(ctx context.Context, uploadID Identifier) (gen.UploadSession, Identifier, error) {
	if err := ctx.Err(); err != nil {
		return gen.UploadSession{}, "", contextError(err)
	}
	if !validIdentifier(uploadID) {
		return gen.UploadSession{}, "", platform.NewError(platform.CodeValidation, false, "上传标识不合法")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.state.Uploads[string(uploadID)]
	if !ok {
		return gen.UploadSession{}, "", platform.NewError(platform.CodeNotFound, false, "上传不存在")
	}
	return record.Session, record.CreatedBy, nil
}

func (s *localBlobStore) AbortUpload(ctx context.Context, siteID, uploadID Identifier) error {
	if err := ctx.Err(); err != nil {
		return contextError(err)
	}
	if !validIdentifier(siteID) || !validIdentifier(uploadID) {
		return platform.NewError(platform.CodeValidation, false, "上传标识不合法")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.state.Uploads[string(uploadID)]
	if !ok {
		return platform.NewError(platform.CodeNotFound, false, "上传不存在")
	}
	if record.Session.SiteId != siteID {
		return platform.NewError(platform.CodeForbidden, false, "上传不属于当前站点")
	}
	if record.Session.State == gen.Aborted || record.Session.State == gen.Expired {
		return nil
	}
	if record.Session.State != gen.Receiving {
		return platform.NewError(platform.CodeConflict, false, "上传状态不允许取消")
	}
	record.Session.State = gen.Aborted
	record.Session.Revision++
	s.state.Uploads[string(uploadID)] = record
	if err := s.persistStateLocked(); err != nil {
		return err
	}
	_ = os.Remove(s.uploadPath(uploadID))
	return nil
}

func (s *localBlobStore) CommitVerifiedBlob(ctx context.Context, request blobCommitRequest) (gen.Asset, error) {
	if err := ctx.Err(); err != nil {
		return gen.Asset{}, contextError(err)
	}
	if !validIdentifier(request.SiteID) || !validIdentifier(request.UploadID) ||
		!validHash(request.SHA256) || request.ByteSize < 1 {
		return gen.Asset{}, platform.NewError(platform.CodeValidation, false, "不可变资源提交参数不合法")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, exists := s.state.Uploads[string(request.UploadID)]
	if !exists {
		return gen.Asset{}, platform.NewError(platform.CodeNotFound, false, "上传不存在")
	}
	if record.Session.SiteId != request.SiteID {
		return gen.Asset{}, platform.NewError(platform.CodeForbidden, false, "上传不属于当前站点")
	}
	if record.Session.ExpectedSha256 != request.SHA256 || record.Session.ExpectedBytes != request.ByteSize {
		return gen.Asset{}, platform.NewError(platform.CodeIntegrityError, false, "上传声明与临时对象不一致")
	}
	if record.Session.State == gen.Complete {
		for _, asset := range s.state.Assets {
			if asset.SiteId == request.SiteID && asset.Ref.Sha256 == request.SHA256 && asset.Ref.ByteSize == request.ByteSize {
				return asset, nil
			}
		}
		return gen.Asset{}, platform.NewError(platform.CodeIntegrityError, false, "已完成上传缺少资源元数据")
	}
	if record.Session.State != gen.Receiving {
		return gen.Asset{}, platform.NewError(platform.CodeConflict, false, "上传状态不允许提交")
	}
	temporaryPath := s.uploadPath(request.UploadID)
	actualHash, actualSize, err := hashFile(temporaryPath)
	if err != nil {
		return gen.Asset{}, platform.NewError(platform.CodeIntegrityError, false, "读取上传临时对象失败")
	}
	if actualHash != request.SHA256 || actualSize != int64(request.ByteSize) {
		return gen.Asset{}, platform.NewError(platform.CodeIntegrityError, false, "上传临时对象完整性校验失败")
	}

	finalPath := s.blobPath(request.SiteID, request.SHA256)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "建立不可变资源目录失败")
	}
	if err := s.appendJournalLocked(blobJournalEvent{
		Event:    blobJournalPrepared,
		SiteID:   request.SiteID,
		UploadID: request.UploadID,
		SHA256:   request.SHA256,
	}); err != nil {
		return gen.Asset{}, err
	}
	if _, err := os.Stat(finalPath); err == nil {
		existingHash, existingSize, hashErr := hashFile(finalPath)
		if hashErr != nil || existingHash != request.SHA256 || existingSize != int64(request.ByteSize) {
			return gen.Asset{}, platform.NewError(platform.CodeIntegrityError, false, "同站点不可变资源校验失败")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		stagingPath := s.stagingPath(request.UploadID, request.SHA256)
		if err := copyAndSync(temporaryPath, stagingPath); err != nil {
			return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "写入不可变资源暂存失败")
		}
		if err := os.Rename(stagingPath, finalPath); err != nil {
			_ = os.Remove(stagingPath)
			return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "提交不可变资源失败")
		}
		if err := syncDirectory(filepath.Dir(finalPath)); err != nil {
			return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化不可变资源目录失败")
		}
	} else {
		return gen.Asset{}, platform.NewError(platform.CodeDependencyUnavailable, true, "检查不可变资源失败")
	}
	if existing, ok := s.findReadyAssetLocked(request.SiteID, request.SHA256, request.ByteSize, request.MediaType); ok {
		oldRecord := record
		if err := s.appendJournalLocked(blobJournalEvent{
			Event:    blobJournalCommitted,
			SiteID:   request.SiteID,
			UploadID: request.UploadID,
			SHA256:   request.SHA256,
			AssetID:  existing.Ref.AssetId,
		}); err != nil {
			return gen.Asset{}, err
		}
		record.Session.State = gen.Complete
		record.Session.ReceivedBytes = record.Session.ExpectedBytes
		record.Session.Revision++
		s.state.Uploads[string(request.UploadID)] = record
		if err := s.persistStateLocked(); err != nil {
			s.state.Uploads[string(request.UploadID)] = oldRecord
			return gen.Asset{}, err
		}
		_ = os.Remove(temporaryPath)
		return existing, nil
	}

	asset := gen.Asset{
		Animated:     false,
		CreatedAt:    time.Now().UTC(),
		CreatedBy:    record.CreatedBy,
		Height:       nil,
		ImageVariants: []gen.ImageVariant{},
		Kind:         gen.AssetKind(record.Session.Kind),
		OriginalName: record.Session.OriginalName,
		Ref: gen.AssetVersionRef{
			AssetId:   platform.NewIdentifier(),
			Version:   0,
			Sha256:    request.SHA256,
			MediaType: request.MediaType,
			ByteSize:  request.ByteSize,
		},
		Revision: 0,
		SiteId:   request.SiteID,
		State:    gen.AssetStateReady,
		Width:    nil,
	}
	if err := s.appendJournalLocked(blobJournalEvent{
		Event:    blobJournalCommitted,
		SiteID:   request.SiteID,
		UploadID: request.UploadID,
		SHA256:   request.SHA256,
		AssetID:  asset.Ref.AssetId,
	}); err != nil {
		return gen.Asset{}, err
	}
	oldRecord := record
	record.Session.State = gen.Complete
	record.Session.ReceivedBytes = record.Session.ExpectedBytes
	record.Session.Revision++
	s.state.Uploads[string(request.UploadID)] = record
	s.state.Assets[string(asset.Ref.AssetId)] = asset
	if err := s.persistStateLocked(); err != nil {
		delete(s.state.Assets, string(asset.Ref.AssetId))
		s.state.Uploads[string(request.UploadID)] = oldRecord
		return gen.Asset{}, err
	}
	_ = os.Remove(temporaryPath)
	return asset, nil
}

func (s *localBlobStore) findReadyAssetLocked(siteID Identifier, sha256Hash Hash, byteSize int, mediaType string) (gen.Asset, bool) {
	for _, asset := range s.state.Assets {
		if asset.SiteId == siteID && asset.State == gen.AssetStateReady &&
			asset.Ref.Sha256 == sha256Hash && asset.Ref.ByteSize == byteSize && asset.Ref.MediaType == mediaType {
			return asset, true
		}
	}
	return gen.Asset{}, false
}

// CommitAssetBlob 将已验证上传提交到不可变内容寻址边界。哈希在提交前再次
// 从临时对象读取，防止调用方仅凭声明的 SHA 绕过校验；文件落盘、journal
// 和资源元数据事务由 blobStore 的私有实现原子完成。
func CommitAssetBlob(ctx context.Context, op OperationContext, input CommitAssetBlobInput) (CommitAssetBlobResult, error) {
	workCtx, cancel, err := uploadOperationContext(ctx, op)
	if err != nil {
		return CommitAssetBlobResult{}, err
	}
	defer cancel()

	if !validIdentifier(input.UploadID) {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeValidation, false, "上传标识不合法")
	}
	if !validHash(input.SHA256) {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeIntegrityError, false, "上传摘要不合法")
	}
	if !validMediaType(input.MediaType) {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeValidation, false, "媒体类型不合法")
	}
	if !validIdentifier(op.Actor.SiteID) {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeValidation, false, "站点标识不合法")
	}

	store := currentBlobStore()
	if store == nil {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储尚未装配")
	}

	hashResult, err := HashUpload(workCtx, op, HashUploadInput{UploadID: input.UploadID})
	if err != nil {
		return CommitAssetBlobResult{}, err
	}
	if hashResult.SHA256 != input.SHA256 {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeIntegrityError, false, "上传对象摘要与声明不一致")
	}

	select {
	case <-workCtx.Done():
		return CommitAssetBlobResult{}, contextError(workCtx.Err())
	default:
	}
	asset, err := store.CommitVerifiedBlob(workCtx, blobCommitRequest{
		SiteID:    op.Actor.SiteID,
		UploadID:  input.UploadID,
		SHA256:    input.SHA256,
		MediaType: input.MediaType,
		ByteSize:  hashResult.ByteSize,
	})
	if err != nil {
		return CommitAssetBlobResult{}, mapBlobStoreError(err, "提交不可变资源失败")
	}
	if err := validateCommittedAsset(asset, op.Actor.SiteID, input, hashResult.ByteSize); err != nil {
		return CommitAssetBlobResult{}, err
	}
	return CommitAssetBlobResult{Asset: asset}, nil
}

func validateCommittedAsset(asset gen.Asset, siteID Identifier, input CommitAssetBlobInput, byteSize int) error {
	if asset.SiteId != siteID || asset.State != gen.AssetStateReady ||
		!validIdentifier(asset.Ref.AssetId) || asset.Ref.Sha256 != input.SHA256 ||
		asset.Ref.ByteSize != byteSize || asset.Ref.MediaType != input.MediaType {
		return platform.NewError(platform.CodeIntegrityError, false, "不可变资源提交回执不一致")
	}
	if !validIdentifier(asset.CreatedBy) || !utf8ValidAndNonEmpty(asset.OriginalName) ||
		!validAssetKind(asset.Kind) || asset.CreatedAt.IsZero() || asset.Revision < 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "资源元数据回执不完整")
	}
	return nil
}

func validAssetKind(kind gen.AssetKind) bool {
	switch kind {
	case gen.AssetKindAttachment, gen.AssetKindDrawingScene, gen.AssetKindDrawingSvg,
		gen.AssetKindImage, gen.AssetKindPlatformCover, gen.AssetKindVideo:
		return true
	default:
		return false
	}
}

func utf8ValidAndNonEmpty(value string) bool {
	return value != "" && utf8.ValidString(value)
}

func mapBlobStoreError(err error, fallback string) error {
	if err == nil {
		return nil
	}
	var platformErr *platform.Error
	if errors.As(err, &platformErr) {
		return platformErr
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, fallback)
}
