package protection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/data-protection/internal/gen"
)

// ExportDomainSnapshotInput保留本域types合同的单一owner与原导出请求。
type ExportDomainSnapshotInput struct {
	Owner   string                    `json:"owner"`
	Request gen.SnapshotExportRequest `json:"request"`
}

type ExportDomainSnapshotResult struct {
	Result gen.SnapshotExportResult `json:"result"`
}

// SnapshotDomainClient只暴露生产方登记的HTTP合同，不导入任何外域内部业务代码。
type SnapshotDomainClient struct {
	ExportSnapshot      func(context.Context, gen.SnapshotExportRequest, string) (gen.SnapshotExportResult, error)
	ReadSnapshotArchive func(context.Context, string, url.Values) (*http.Response, error)
}

var externalSnapshotOwners = map[string]bool{
	"analytics":     true,
	"content":       true,
	"identity":      true,
	"jobs":          true,
	"knowledge":     true,
	"localization":  true,
	"media":         true,
	"memory":        true,
	"model-gateway": true,
	"public-site":   true,
	"qa":            true,
	"site-config":   true,
	"traffic-guard": true,
}

// ExportDomainSnapshot按真实安全截点导出单个外部拥有者，并把完整归档落到本域隔离incoming目录。
func ExportDomainSnapshot(ctx context.Context, op OperationContext, in ExportDomainSnapshotInput) (ExportDomainSnapshotResult, error) {
	if op.CallerService != "data-protection" || op.CostClass != "" {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeForbidden, false, "单域快照导出只接受保护服务控制调用")
	}
	if !externalSnapshotOwners[in.Owner] || !platform.ValidIdentifier(in.Request.BackupId) || !platform.ValidIdentifier(in.Request.BarrierId) || in.Request.DataEpoch != op.DataEpoch {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeValidation, false, "单域快照导出绑定不合法")
	}
	store, err := Catalog(op)
	if err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	if err = requireAdministrator(op); err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	if op.Actor.SubjectID == "" || op.Actor.SessionID == "" || op.Actor.SiteID == "" {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeForbidden, false, "单域快照导出缺原超管委托身份")
	}
	if store.Authorize == nil {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "保护授权复核端口未装配")
	}
	if err = store.Authorize(ctx, op, "superadmin", "readSnapshotArchive"); err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	client, ok := store.SnapshotDomains[in.Owner]
	if !ok || client.ExportSnapshot == nil || client.ReadSnapshotArchive == nil {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "拥有者快照端口未装配")
	}
	if store.Runtime == nil {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "保护运行权威未装配")
	}
	view, err := store.Runtime(ctx)
	if err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	if view.Phase != platform.PhaseBackupBarrier || !view.Maintenance || view.BarrierID != in.Request.BarrierId || view.DataEpoch != in.Request.DataEpoch || view.DataEpoch != store.Epoch {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeLocked, false, "单域导出没有对应备份安全截点")
	}
	operationID := "snapshot-export-" + platform.HashBytes([]byte(in.Request.BackupId+"\x00"+in.Owner))[:32]
	exported, err := client.ExportSnapshot(ctx, in.Request, operationID)
	if err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	if err = validateSnapshotExportResult(in.Request, in.Owner, exported); err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	ref := exported.Snapshot.Object
	query := url.Values{}
	query.Set("sha256", ref.Sha256)
	query.Set("byteSize", strconv.FormatInt(int64(ref.ByteSize), 10))
	response, err := client.ReadSnapshotArchive(ctx, exported.Snapshot.SnapshotId, query)
	if err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	if response == nil || response.Body == nil {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "拥有者归档流缺失")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "拥有者归档读取失败")
	}
	mediaType, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/x-tar" || (response.ContentLength >= 0 && response.ContentLength != int64(ref.ByteSize)) {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeIntegrityError, false, "拥有者归档响应元数据不符")
	}
	base := filepath.Join(store.Root, "active", "incoming", "backup-"+in.Request.BackupId)
	if err = os.MkdirAll(base, 0o700); err != nil {
		return ExportDomainSnapshotResult{}, storageError(err)
	}
	if err = lifecycle.ValidatePrivatePath(base, "directory", false); err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	final := filepath.Join(base, in.Owner+"-"+exported.Snapshot.SnapshotId+".tar")
	cached := false
	if _, statErr := os.Lstat(final); statErr == nil {
		if _, err = VerifyObjectHash(ctx, op, VerifyObjectHashInput{ObjectPath: final, ExpectedHash: ref.Sha256, ExpectedBytes: int64(ref.ByteSize)}); err != nil {
			return ExportDomainSnapshotResult{}, err
		}
		cached = true
	} else if !os.IsNotExist(statErr) {
		return ExportDomainSnapshotResult{}, storageError(statErr)
	}
	temporary := final
	removeTemporary := false
	if !cached {
		temporary = final + ".partial-" + platform.NewIdentifier()
		if err = lifecycle.ValidatePrivatePath(temporary, "regular-file", true); err != nil {
			return ExportDomainSnapshotResult{}, err
		}
		file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return ExportDomainSnapshotResult{}, storageError(err)
		}
		removeTemporary = true
		defer func() {
			if removeTemporary {
				_ = os.Remove(temporary)
			}
		}()
		digest := sha256.New()
		count, copyErr := io.CopyBuffer(io.MultiWriter(file, digest), io.LimitReader(response.Body, int64(ref.ByteSize)+1), make([]byte, 1<<20))
		if copyErr == nil && (count != int64(ref.ByteSize) || hex.EncodeToString(digest.Sum(nil)) != ref.Sha256) {
			copyErr = platform.NewError(platform.CodeIntegrityError, false, "拥有者归档长度或hash不符")
		}
		if copyErr == nil {
			copyErr = file.Sync()
		}
		closeErr := file.Close()
		if copyErr != nil {
			return ExportDomainSnapshotResult{}, storageError(copyErr)
		}
		if closeErr != nil {
			return ExportDomainSnapshotResult{}, storageError(closeErr)
		}
	}
	if _, err = PrivateObjectPath(store, temporary); err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	validator := store.PayloadValidators["DomainSnapshotManifest"]
	if validator == nil {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "拥有者清单严格schema未装配")
	}
	verifyRoot, err := os.MkdirTemp(base, ".verify-"+exported.Snapshot.SnapshotId+"-")
	if err != nil {
		return ExportDomainSnapshotResult{}, storageError(err)
	}
	defer os.RemoveAll(verifyRoot)
	archive, err := os.Open(temporary)
	if err != nil {
		return ExportDomainSnapshotResult{}, storageError(err)
	}
	defer archive.Close()
	expected := platform.ObjectRef{
		Owner:     ref.Owner,
		ObjectID:  ref.ObjectId,
		Version:   int64(ref.Version),
		SHA256:    ref.Sha256,
		MediaType: ref.MediaType,
		ByteSize:  int64(ref.ByteSize),
	}
	verified, err := lifecycle.ExtractSnapshotArchive(ctx, archive, verifyRoot, expected, validator)
	if err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	manifest := verified.Manifest
	if manifest.Format != "nice-wiki.domain-snapshot" || manifest.Owner != in.Owner || manifest.BackupID != in.Request.BackupId || manifest.BarrierID != in.Request.BarrierId || manifest.SnapshotID != exported.Snapshot.SnapshotId || manifest.SourceEpoch != exported.Snapshot.DataEpoch || manifest.DataSchemaVersion != exported.Snapshot.DataSchemaVersion || manifest.RevisionWatermark != int64(exported.Snapshot.RevisionWatermark) || manifest.ContainsSecrets != exported.Snapshot.ContainsSecrets {
		return ExportDomainSnapshotResult{}, platform.NewError(platform.CodeIntegrityError, false, "拥有者真实归档清单与导出回执不符")
	}
	if cached {
		return ExportDomainSnapshotResult{Result: exported}, nil
	}
	if _, statErr := os.Lstat(final); statErr == nil {
		if _, err = VerifyObjectHash(ctx, op, VerifyObjectHashInput{ObjectPath: final, ExpectedHash: ref.Sha256, ExpectedBytes: int64(ref.ByteSize)}); err != nil {
			return ExportDomainSnapshotResult{}, err
		}
		return ExportDomainSnapshotResult{Result: exported}, nil
	} else if !os.IsNotExist(statErr) {
		return ExportDomainSnapshotResult{}, storageError(statErr)
	}
	if err = os.Rename(temporary, final); err != nil {
		return ExportDomainSnapshotResult{}, storageError(err)
	}
	removeTemporary = false
	if err = syncObjectDirectory(base, store.Root); err != nil {
		return ExportDomainSnapshotResult{}, err
	}
	return ExportDomainSnapshotResult{Result: exported}, nil
}

func validateSnapshotExportResult(request gen.SnapshotExportRequest, owner string, result gen.SnapshotExportResult) error {
	snapshot := result.Snapshot
	ref := snapshot.Object
	if !platform.ValidIdentifier(snapshot.SnapshotId) || snapshot.Owner != owner || snapshot.DataEpoch != request.DataEpoch || snapshot.DataSchemaVersion < 1 || snapshot.RevisionWatermark < 0 || (snapshot.ContainsSecrets && !request.IncludeSecrets) ||
		ref.Owner != owner || ref.ObjectId != snapshot.SnapshotId || ref.Version != 1 || ref.MediaType != "application/x-tar" || !platform.ValidHash(ref.Sha256) || ref.ByteSize < 1024 || int64(ref.ByteSize) > lifecycle.MaxArchiveBytes || result.Checksum != ref.Sha256 {
		return platform.NewError(platform.CodeIntegrityError, false, "拥有者快照回执与原导出范围不符")
	}
	return nil
}
