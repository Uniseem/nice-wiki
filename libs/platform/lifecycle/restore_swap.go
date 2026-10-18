package lifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"nice-wiki/libs/platform"
	"os"
	"path/filepath"
	"sync"
)

const (
	controlDir     = "control"
	activeDir      = "active"
	restoreDir     = "restore"
	restoreLogName = "restore-log.jsonl"
	stagedDirName  = "staged"
	beforeDirName  = "before"
	switchedMarker = "switched"
	rolledbackMark = "rolled-back"
)

// 日志每步先后都有完整绑定，旧缺证明日志只保留，不当作可换入依据。
type restoreLogEntry struct {
	RestoreID        platform.Identifier `json:"restoreId"`
	Action           string              `json:"action"`
	At               platform.Timestamp  `json:"at"`
	StageID          string              `json:"stageId,omitempty"`
	ManifestHash     string              `json:"manifestHash,omitempty"`
	HostManifestHash string              `json:"hostManifestHash,omitempty"`
	InputHash        string              `json:"inputHash,omitempty"`
	HostEpoch        string              `json:"hostEpoch,omitempty"`
	TargetEpoch      string              `json:"targetEpoch,omitempty"`
	BarrierID        string              `json:"barrierId,omitempty"`
	RuntimeRevision  int64               `json:"runtimeRevision,omitempty"`
}

var swapMu sync.Mutex

func integrity(message string) error {
	return platform.NewError(platform.CodeIntegrityError, false, "%s", message)
}

// readSnapshotStageRecord核完整stage回执与原输入、源清单、目标树摘要；不以目录名证明staged。
func readSnapshotStageRecord(dataRoot, restoreID string) (snapshotStageRecord, error) {
	var record snapshotStageRecord
	if !platform.ValidIdentifier(restoreID) {
		return record, integrity("恢复ID不合法")
	}
	if err := readSnapshotJSON(filepath.Join(dataRoot, controlDir, "snapshot-stages", restoreID+".json"), 48<<20, &record); err != nil {
		return record, err
	}
	if record.Manifest == nil || record.TargetTree == nil {
		return record, integrity("stage源清单或目标树证明缺失")
	}
	input, receipt := record.Input, record.Receipt
	raw, _ := platform.CanonicalJSON(input)
	manifest, _ := platform.CanonicalJSON(record.Manifest)
	tree, _ := platform.CanonicalJSON(record.TargetTree)
	if input.RestoreID != restoreID || receipt.RestoreID != restoreID || receipt.StageID != snapshotStageID(restoreID, input.Snapshot.SnapshotID) || receipt.SnapshotID != input.Snapshot.SnapshotID || receipt.InputHash != platform.HashBytes(raw) || record.SourceManifestHash != platform.HashBytes(manifest) || record.TreeFingerprint != platform.HashBytes(tree) || receipt.ManifestHash != record.TreeFingerprint {
		return record, integrity("stage原输入/源清单/完整目标树摘要矛盾")
	}
	if record.Manifest.Format != "nice-wiki.domain-snapshot" || record.Manifest.SchemaVersion != 1 || record.Manifest.Files == nil || record.Manifest.Objects == nil || record.Manifest.Publications == nil || record.Manifest.References == nil || !platform.ValidIdentifier(record.HostEpoch) || !platform.ValidIdentifier(input.TargetDataEpoch) || record.HostEpoch == input.TargetDataEpoch || receipt.TargetEpoch != input.TargetDataEpoch || receipt.SourceEpoch != input.Snapshot.DataEpoch || record.Manifest.SourceEpoch != input.Snapshot.DataEpoch || record.Manifest.Owner != input.Snapshot.Owner || record.Manifest.SnapshotID != input.Snapshot.SnapshotID || record.Manifest.DataSchemaVersion != input.Snapshot.DataSchemaVersion || record.Manifest.RevisionWatermark != input.Snapshot.RevisionWatermark || record.Manifest.ContainsSecrets != input.Snapshot.ContainsSecrets || input.Snapshot.Object.Owner != input.Snapshot.Owner || input.Snapshot.Object.ObjectID != input.Snapshot.SnapshotID || input.Snapshot.Object.Version != 1 || input.Snapshot.Object.MediaType != "application/x-tar" || input.Checksum != input.Snapshot.Object.SHA256 || !platform.ValidHash(input.Checksum) || record.TargetTree.Files == nil || record.TargetTree.Directories == nil {
		return record, integrity("stage代际/源归档引用/完整目标树证明不合法")
	}
	switch receipt.State {
	case "staged", "discarding-staged", "discarding-before", "discarded-staged", "discarded-before":
	default:
		return record, integrity("stage尚未完整持久完成")
	}
	return record, nil
}

// ApplyRestoreSwap只在入口开放前执行；prepared日志先于rename，双方父目录fsync先于下一条日志。
func ApplyRestoreSwap(dataRoot string, runtime platform.RuntimeView) (platform.SwapResult, error) {
	swapMu.Lock()
	defer swapMu.Unlock()
	if err := ValidatePrivatePath(dataRoot, "directory", false); err != nil {
		return platform.SwapResult{}, err
	}
	if runtime.RestoreID == "" {
		return platform.SwapResult{Action: platform.SwapNone}, nil
	}
	if runtime.Phase != platform.PhaseRestoreSwitching && runtime.Phase != platform.PhaseRestoreRollingBack {
		return platform.SwapResult{Action: platform.SwapNone}, nil
	}
	record, err := readSnapshotStageRecord(dataRoot, runtime.RestoreID)
	if err != nil {
		return platform.SwapResult{}, err
	}
	if !runtime.Maintenance || runtime.Revision < 1 || runtime.DataEpoch != record.HostEpoch || runtime.TargetDataEpoch != record.Input.TargetDataEpoch || runtime.BarrierID != record.Input.BarrierID || record.Receipt.State != "staged" {
		return platform.SwapResult{}, integrity("换目录决定与完整stage原host/target/barrier不相符")
	}
	entries, err := readRestoreLog(dataRoot)
	if err != nil {
		return platform.SwapResult{}, err
	}
	last, err := restoreLogState(entries, record)
	if err != nil {
		return platform.SwapResult{}, err
	}
	if last.RuntimeRevision > runtime.Revision {
		return platform.SwapResult{}, integrity("当前运行决定早于已持久恢复水位")
	}
	if runtime.Phase == platform.PhaseRestoreSwitching {
		return applySnapshotSwitch(dataRoot, runtime, record, last)
	}
	return applySnapshotRollback(dataRoot, runtime, record, last)
}

func snapshotDirectoryExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, snapshotIOError(err)
	}
	if err = ValidatePrivatePath(path, "directory", false); err != nil {
		return false, err
	}
	return true, nil
}
func snapshotSwapPaths(dataRoot, restoreID string) (string, string, string, string) {
	base := filepath.Join(dataRoot, restoreDir, restoreID)
	return filepath.Join(dataRoot, activeDir), filepath.Join(base, stagedDirName), filepath.Join(base, beforeDirName), base
}
func verifySnapshotHash(root, expected string) error {
	_, hash, err := fingerprintSnapshotTree(context.Background(), root, MaxArchiveBytes)
	if err != nil {
		return err
	}
	if hash != expected {
		return integrity("恢复目录完整树与持久阶段证明不一致")
	}
	return nil
}
func snapshotRename(from, to string) error {
	exists, err := snapshotDirectoryExists(to)
	if err != nil {
		return err
	}
	if exists {
		return integrity("rename目标目录已存在，拒绝覆盖未知数据")
	}
	if err = ValidatePrivatePath(from, "directory", false); err != nil {
		return err
	}
	if err = os.Rename(from, to); err != nil {
		return snapshotIOError(err)
	}
	return nil
}
func snapshotSwapLog(dataRoot string, entry restoreLogEntry, action string) (restoreLogEntry, error) {
	if err := syncDir(filepath.Join(dataRoot, restoreDir, entry.RestoreID)); err != nil {
		return entry, snapshotIOError(err)
	}
	if err := syncDir(dataRoot); err != nil {
		return entry, snapshotIOError(err)
	}
	entry.Action = action
	entry.At = platform.Now()
	return entry, appendRestoreLog(dataRoot, entry)
}
func applySnapshotSwitch(dataRoot string, runtime platform.RuntimeView, record snapshotStageRecord, last restoreLogEntry) (platform.SwapResult, error) {
	active, staged, before, _ := snapshotSwapPaths(dataRoot, record.Input.RestoreID)
	a, err := snapshotDirectoryExists(active)
	if err != nil {
		return platform.SwapResult{}, err
	}
	s, err := snapshotDirectoryExists(staged)
	if err != nil {
		return platform.SwapResult{}, err
	}
	b, err := snapshotDirectoryExists(before)
	if err != nil {
		return platform.SwapResult{}, err
	}
	if last.Action == "rolled-back" || last.Action == "aborted" || last.Action == "prepared-rollback" || last.Action == "new-renamed" {
		return platform.SwapResult{}, integrity("已进入换回的恢复不可凭旧stage再次换入")
	}
	if last.Action == "" {
		if !a || !s || b {
			return platform.SwapResult{}, integrity("未准备切换的active/staged/before组合不合法")
		}
		if err = verifySnapshotTarget(context.Background(), staged, record); err != nil {
			return platform.SwapResult{}, err
		}
		_, hostHash, err := fingerprintSnapshotTree(context.Background(), active, MaxArchiveBytes)
		if err != nil {
			return platform.SwapResult{}, err
		}
		last = restoreLogEntry{RestoreID: record.Input.RestoreID, StageID: record.Receipt.StageID, ManifestHash: record.TreeFingerprint, HostManifestHash: hostHash, InputHash: record.Receipt.InputHash, HostEpoch: record.HostEpoch, TargetEpoch: record.Input.TargetDataEpoch, BarrierID: record.Input.BarrierID, RuntimeRevision: runtime.Revision}
		last, err = snapshotSwapLog(dataRoot, last, "prepared-switch")
		if err != nil {
			return platform.SwapResult{}, err
		}
	}
	if last.Action == "prepared-switch" {
		if a && !b && s {
			if err = verifySnapshotHash(active, last.HostManifestHash); err != nil {
				return platform.SwapResult{}, err
			}
			if err = verifySnapshotTarget(context.Background(), staged, record); err != nil {
				return platform.SwapResult{}, err
			}
			if err = snapshotRename(active, before); err != nil {
				return platform.SwapResult{}, err
			}
			a, b = false, true
		} else if a || !b || !s {
			return platform.SwapResult{}, integrity("prepared-switch目录状态无法续接")
		}
		if err = verifySnapshotHash(before, last.HostManifestHash); err != nil {
			return platform.SwapResult{}, err
		}
		last, err = snapshotSwapLog(dataRoot, last, "old-renamed")
		if err != nil {
			return platform.SwapResult{}, err
		}
	}
	if last.Action == "old-renamed" {
		if !b {
			return platform.SwapResult{}, integrity("原目录已换出但before缺失")
		}
		if err = verifySnapshotHash(before, last.HostManifestHash); err != nil {
			return platform.SwapResult{}, err
		}
		if !a && s {
			if err = verifySnapshotTarget(context.Background(), staged, record); err != nil {
				return platform.SwapResult{}, err
			}
			if err = snapshotRename(staged, active); err != nil {
				return platform.SwapResult{}, err
			}
			a, s = true, false
		} else if !a || s {
			return platform.SwapResult{}, integrity("old-renamed目录状态无法续接")
		}
		if err = verifySnapshotHash(active, last.ManifestHash); err != nil {
			return platform.SwapResult{}, err
		}
		last, err = snapshotSwapLog(dataRoot, last, "switched")
		if err != nil {
			return platform.SwapResult{}, err
		}
	}
	if last.Action != "switched" || !a || !b || s {
		return platform.SwapResult{}, integrity("switched目录与日志不一致")
	}
	if err = verifySnapshotHash(active, last.ManifestHash); err != nil {
		return platform.SwapResult{}, err
	}
	if err = verifySnapshotHash(before, last.HostManifestHash); err != nil {
		return platform.SwapResult{}, err
	}
	return platform.SwapResult{Action: platform.SwapSwitched, RestoreID: record.Input.RestoreID}, nil
}
func applySnapshotRollback(dataRoot string, runtime platform.RuntimeView, record snapshotStageRecord, last restoreLogEntry) (platform.SwapResult, error) {
	active, staged, before, _ := snapshotSwapPaths(dataRoot, record.Input.RestoreID)
	a, err := snapshotDirectoryExists(active)
	if err != nil {
		return platform.SwapResult{}, err
	}
	s, err := snapshotDirectoryExists(staged)
	if err != nil {
		return platform.SwapResult{}, err
	}
	b, err := snapshotDirectoryExists(before)
	if err != nil {
		return platform.SwapResult{}, err
	}
	if last.Action == "" {
		if !a || !s || b {
			return platform.SwapResult{}, integrity("未切换域的换回目录状态不明")
		}
		if err = verifySnapshotTarget(context.Background(), staged, record); err != nil {
			return platform.SwapResult{}, err
		}
		_, hostHash, err := fingerprintSnapshotTree(context.Background(), active, MaxArchiveBytes)
		if err != nil {
			return platform.SwapResult{}, err
		}
		last = restoreLogEntry{RestoreID: record.Input.RestoreID, StageID: record.Receipt.StageID, ManifestHash: record.TreeFingerprint, HostManifestHash: hostHash, InputHash: record.Receipt.InputHash, HostEpoch: record.HostEpoch, TargetEpoch: record.Input.TargetDataEpoch, BarrierID: record.Input.BarrierID, RuntimeRevision: runtime.Revision}
		if _, err = snapshotSwapLog(dataRoot, last, "aborted"); err != nil {
			return platform.SwapResult{}, err
		}
		return platform.SwapResult{Action: platform.SwapNone}, nil
	}
	if last.Action == "prepared-switch" || last.Action == "old-renamed" {
		if !a && b && s {
			if err = verifySnapshotHash(before, last.HostManifestHash); err != nil {
				return platform.SwapResult{}, err
			}
			if err = verifySnapshotTarget(context.Background(), staged, record); err != nil {
				return platform.SwapResult{}, err
			}
			if err = snapshotRename(before, active); err != nil {
				return platform.SwapResult{}, err
			}
			a, b = true, false
		}
		if a && !b && s {
			if err = verifySnapshotHash(active, last.HostManifestHash); err != nil {
				return platform.SwapResult{}, err
			}
			_, err = snapshotSwapLog(dataRoot, last, "aborted")
			if err != nil {
				return platform.SwapResult{}, err
			}
			return platform.SwapResult{Action: platform.SwapNone}, nil
		}
		if last.Action != "old-renamed" || !a || !b || s {
			return platform.SwapResult{}, integrity("中断切换无法证明是否已换入")
		}
		if err = verifySnapshotHash(active, last.ManifestHash); err != nil {
			return platform.SwapResult{}, err
		}
		last, err = snapshotSwapLog(dataRoot, last, "switched")
		if err != nil {
			return platform.SwapResult{}, err
		}
	}
	if last.Action == "aborted" {
		if !a || b || !s {
			return platform.SwapResult{}, integrity("已取消换入的目录不一致")
		}
		if err = verifySnapshotHash(active, last.HostManifestHash); err != nil {
			return platform.SwapResult{}, err
		}
		return platform.SwapResult{Action: platform.SwapNone}, nil
	}
	if last.Action == "switched" {
		if !a || !b || s {
			return platform.SwapResult{}, integrity("换回前缺完整switched目录")
		}
		if err = verifySnapshotHash(before, last.HostManifestHash); err != nil {
			return platform.SwapResult{}, err
		}
		if err = verifySnapshotHash(active, last.ManifestHash); err != nil {
			return platform.SwapResult{}, err
		}
		last.RuntimeRevision = runtime.Revision
		last, err = snapshotSwapLog(dataRoot, last, "prepared-rollback")
		if err != nil {
			return platform.SwapResult{}, err
		}
	}
	if last.Action == "prepared-rollback" {
		if a && b && !s {
			if err = snapshotRename(active, staged); err != nil {
				return platform.SwapResult{}, err
			}
			a, s = false, true
		} else if a || !b || !s {
			return platform.SwapResult{}, integrity("prepared-rollback目录状态无法续接")
		}
		if err = verifySnapshotHash(staged, last.ManifestHash); err != nil {
			return platform.SwapResult{}, err
		}
		last, err = snapshotSwapLog(dataRoot, last, "new-renamed")
		if err != nil {
			return platform.SwapResult{}, err
		}
	}
	if last.Action == "new-renamed" {
		if !a && b && s {
			if err = verifySnapshotHash(before, last.HostManifestHash); err != nil {
				return platform.SwapResult{}, err
			}
			if err = snapshotRename(before, active); err != nil {
				return platform.SwapResult{}, err
			}
			a, b = true, false
		} else if !a || b || !s {
			return platform.SwapResult{}, integrity("new-renamed目录状态无法续接")
		}
		if err = verifySnapshotHash(active, last.HostManifestHash); err != nil {
			return platform.SwapResult{}, err
		}
		last, err = snapshotSwapLog(dataRoot, last, "rolled-back")
		if err != nil {
			return platform.SwapResult{}, err
		}
	}
	if last.Action != "rolled-back" || !a || b || !s {
		return platform.SwapResult{}, integrity("rolled-back目录与日志不一致")
	}
	if err = verifySnapshotHash(active, last.HostManifestHash); err != nil {
		return platform.SwapResult{}, err
	}
	if err = verifySnapshotHash(staged, last.ManifestHash); err != nil {
		return platform.SwapResult{}, err
	}
	return platform.SwapResult{Action: platform.SwapRolledBack, RestoreID: record.Input.RestoreID}, nil
}

// ReadRestoreSwapState只读完整本地stage与日志及目录组合，不修复/改写旧日志或从目录存在猜成功。
func ReadRestoreSwapState(dataRoot, restoreID string) (SnapshotSwapEvidence, error) {
	swapMu.Lock()
	defer swapMu.Unlock()
	record, err := readSnapshotStageRecord(dataRoot, restoreID)
	if err != nil {
		return SnapshotSwapEvidence{}, err
	}
	entries, err := readRestoreLog(dataRoot)
	if err != nil {
		return SnapshotSwapEvidence{}, err
	}
	last, err := restoreLogState(entries, record)
	if err != nil {
		return SnapshotSwapEvidence{}, err
	}
	active, staged, before, _ := snapshotSwapPaths(dataRoot, restoreID)
	a, err := snapshotDirectoryExists(active)
	if err != nil {
		return SnapshotSwapEvidence{}, err
	}
	s, err := snapshotDirectoryExists(staged)
	if err != nil {
		return SnapshotSwapEvidence{}, err
	}
	b, err := snapshotDirectoryExists(before)
	if err != nil {
		return SnapshotSwapEvidence{}, err
	}
	result := SnapshotSwapEvidence{StageID: record.Receipt.StageID, ManifestHash: record.TreeFingerprint, HostManifestHash: last.HostManifestHash, HostEpoch: record.HostEpoch, TargetEpoch: record.Input.TargetDataEpoch}
	switch last.Action {
	case "":
		result.Known = a && !b && (s || record.Receipt.State == "discarding-staged" || record.Receipt.State == "discarded-staged")
	case "switched":
		result.Switched = true
		result.Known = a && !s && (b || record.Receipt.State == "discarding-before" || record.Receipt.State == "discarded-before")
	case "rolled-back", "aborted":
		result.RolledBack = last.Action == "rolled-back"
		result.Aborted = last.Action == "aborted"
		result.Known = a && !b && (s || record.Receipt.State == "discarding-staged" || record.Receipt.State == "discarded-staged")
	default:
		return result, integrity("换目录仍在途，尚无完整本地终态")
	}
	if !result.Known {
		return result, integrity("恢复日志与目录组合矛盾")
	}
	return result, nil
}
func restoreLogState(entries []restoreLogEntry, record snapshotStageRecord) (restoreLogEntry, error) {
	var last restoreLogEntry
	for _, entry := range entries {
		if entry.RestoreID != record.Input.RestoreID {
			continue
		}
		if entry.StageID != record.Receipt.StageID || entry.ManifestHash != record.TreeFingerprint || entry.InputHash != record.Receipt.InputHash || entry.HostEpoch != record.HostEpoch || entry.TargetEpoch != record.Input.TargetDataEpoch || entry.BarrierID != record.Input.BarrierID || !platform.ValidHash(entry.HostManifestHash) || entry.RuntimeRevision < 1 {
			return last, integrity("恢复日志缺完整原stage/host树/运行决定证据")
		}
		if last.Action != "" && (entry.HostManifestHash != last.HostManifestHash || entry.RuntimeRevision < last.RuntimeRevision) {
			return last, integrity("恢复日志变更了原host树或运行水位倒退")
		}
		allowed := false
		switch entry.Action {
		case "prepared-switch":
			allowed = last.Action == ""
		case "old-renamed":
			allowed = last.Action == "prepared-switch"
		case "switched":
			allowed = last.Action == "old-renamed"
		case "prepared-rollback":
			allowed = last.Action == "switched"
		case "new-renamed":
			allowed = last.Action == "prepared-rollback"
		case "rolled-back":
			allowed = last.Action == "new-renamed"
		case "aborted":
			allowed = last.Action == "" || last.Action == "prepared-switch" || last.Action == "old-renamed"
		}
		if !allowed {
			return last, integrity("恢复日志阶段顺序不合法")
		}
		last = entry
	}
	return last, nil
}
func readRestoreLog(dataRoot string) ([]restoreLogEntry, error) {
	path := filepath.Join(dataRoot, controlDir, restoreLogName)
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, snapshotIOError(err)
	}
	if err = ValidatePrivatePath(path, "regular-file", false); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, snapshotIOError(err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err != nil {
		return nil, snapshotIOError(err)
	}
	if len(raw) > 16<<20 {
		return nil, snapshotBudget("恢复日志超过预算")
	}
	entries := []restoreLogEntry{}
	for _, line := range splitLines(raw) {
		var entry restoreLogEntry
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		parseErr := decoder.Decode(&entry)
		_, timeErr := platform.ParseTimestamp(entry.At)
		if parseErr != nil || timeErr != nil || !platform.ValidIdentifier(entry.RestoreID) || entry.Action == "" {
			return nil, integrity("恢复日志损坏")
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// appendRestoreLog复用原JSONL权威，以完整文件fsync+原子rename避免半行日志被猜测为成功。
func appendRestoreLog(dataRoot string, entry restoreLogEntry) error {
	entries, err := readRestoreLog(dataRoot)
	if err != nil {
		return err
	}
	entries = append(entries, entry)
	var raw []byte
	for _, item := range entries {
		line, err := platform.CanonicalJSON(item)
		if err != nil {
			return err
		}
		raw = append(raw, line...)
		raw = append(raw, '\n')
		if len(raw) > 16<<20 {
			return snapshotBudget("恢复日志超过预算")
		}
	}
	dir := filepath.Join(dataRoot, controlDir)
	if err = snapshotDirectory(dir, dataRoot); err != nil {
		return err
	}
	path := filepath.Join(dir, restoreLogName)
	if err = ValidatePrivatePath(path, "regular-file", true); err != nil {
		return err
	}
	temp := filepath.Join(dir, ".restore-log-"+platform.NewIdentifier()+".tmp")
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return snapshotIOError(err)
	}
	defer f.Close()
	installed := false
	defer func() {
		if !installed {
			_ = os.Remove(temp)
		}
	}()
	if _, err = f.Write(raw); err != nil {
		return snapshotIOError(err)
	}
	if err = f.Sync(); err != nil {
		return snapshotIOError(err)
	}
	if err = f.Close(); err != nil {
		return snapshotIOError(err)
	}
	if err = os.Rename(temp, path); err != nil {
		return snapshotIOError(err)
	}
	installed = true
	return snapshotIOError(syncDir(dir))
}
func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func splitLines(raw []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range raw {
		if b == '\n' {
			if i > start {
				lines = append(lines, raw[start:i])
			}
			start = i + 1
		}
	}
	if start < len(raw) {
		lines = append(lines, raw[start:])
	}
	return lines
}
