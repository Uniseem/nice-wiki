package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"nice-wiki/libs/platform"
)

// 控制区目录与恢复日志文件名；control区永不随业务恢复替换。
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

// restoreLogEntry control区恢复日志的一行。
type restoreLogEntry struct {
	RestoreID platform.Identifier `json:"restoreId"`
	Action    string              `json:"action"`
	At        platform.Timestamp  `json:"at"`
}

var swapMu sync.Mutex

// ApplyRestoreSwap 启动时按恢复阶段换入或换回active目录（ADR0003）：
// restore_switching且存在restore/<id>/staged且未换入时，
// active→restore/<id>/before、staged→active，fsync父目录并写日志；
// restore_rolling_back且已换入时反向换回；其余情况不动目录；
// 目录状态与日志矛盾时返回INTEGRITY_ERROR拒绝启动。
func ApplyRestoreSwap(dataRoot string, runtime platform.RuntimeView) (platform.SwapResult, error) {
	swapMu.Lock()
	defer swapMu.Unlock()
	if dataRoot == "" {
		return platform.SwapResult{}, platform.NewError(platform.CodeValidation, false, "数据根缺失")
	}
	if runtime.RestoreID == "" {
		return platform.SwapResult{Action: platform.SwapNone}, nil
	}
	restoreBase := filepath.Join(dataRoot, restoreDir, runtime.RestoreID)
	entries, err := readRestoreLog(dataRoot)
	if err != nil {
		return platform.SwapResult{}, err
	}
	switched, rolledBack := logState(entries, runtime.RestoreID)
	switch runtime.Phase {
	case platform.PhaseRestoreSwitching:
		if rolledBack {
			return platform.SwapResult{}, integrity("目录与恢复日志矛盾：已换回却处于切换阶段")
		}
		if switched {
			// 上次启动已完成换入；幂等返回。
			return platform.SwapResult{Action: platform.SwapSwitched, RestoreID: runtime.RestoreID}, nil
		}
		if !dirExists(filepath.Join(restoreBase, stagedDirName)) {
			// 本域尚未暂存完成：不动作，等data-protection协调。
			return platform.SwapResult{Action: platform.SwapNone}, nil
		}
		if err := swapIn(dataRoot, restoreBase, runtime.RestoreID); err != nil {
			return platform.SwapResult{}, err
		}
		return platform.SwapResult{Action: platform.SwapSwitched, RestoreID: runtime.RestoreID}, nil
	case platform.PhaseRestoreRollingBack:
		if rolledBack {
			return platform.SwapResult{Action: platform.SwapRolledBack, RestoreID: runtime.RestoreID}, nil
		}
		if !switched {
			// 从未换入则无需换回。
			return platform.SwapResult{Action: platform.SwapNone}, nil
		}
		if !dirExists(filepath.Join(restoreBase, beforeDirName)) {
			return platform.SwapResult{}, integrity("已换入但before目录缺失，拒绝换回")
		}
		if err := swapBack(dataRoot, restoreBase, runtime.RestoreID); err != nil {
			return platform.SwapResult{}, err
		}
		return platform.SwapResult{Action: platform.SwapRolledBack, RestoreID: runtime.RestoreID}, nil
	default:
		return platform.SwapResult{Action: platform.SwapNone}, nil
	}
}

// swapIn active→before、staged→active，fsync父目录并写日志与标记。
func swapIn(dataRoot, restoreBase string, restoreID platform.Identifier) error {
	active := filepath.Join(dataRoot, activeDir)
	staged := filepath.Join(restoreBase, stagedDirName)
	before := filepath.Join(restoreBase, beforeDirName)
	if dirExists(active) {
		if err := os.Rename(active, before); err != nil {
			return platform.NewError(platform.CodeInternal, false, "换出active目录失败")
		}
	}
	if err := os.Rename(staged, active); err != nil {
		_ = restoreActiveFromBefore(before, active)
		return platform.NewError(platform.CodeInternal, false, "换入staged目录失败")
	}
	if err := syncDir(dataRoot); err != nil {
		return platform.NewError(platform.CodeInternal, false, "数据根fsync失败")
	}
	if err := markRestore(restoreBase, switchedMarker); err != nil {
		return err
	}
	return appendRestoreLog(dataRoot, restoreLogEntry{RestoreID: restoreID, Action: "switched", At: platform.Now()})
}

// swapBack 把before换回active；新代数据退回staged目录等待清理。
func swapBack(dataRoot, restoreBase string, restoreID platform.Identifier) error {
	active := filepath.Join(dataRoot, activeDir)
	staged := filepath.Join(restoreBase, stagedDirName)
	before := filepath.Join(restoreBase, beforeDirName)
	if dirExists(active) {
		if err := os.Rename(active, staged); err != nil {
			return platform.NewError(platform.CodeInternal, false, "退回新代数据失败")
		}
	}
	if err := os.Rename(before, active); err != nil {
		_ = restoreActiveFromBefore(staged, active)
		return platform.NewError(platform.CodeInternal, false, "换回before目录失败")
	}
	if err := syncDir(dataRoot); err != nil {
		return platform.NewError(platform.CodeInternal, false, "数据根fsync失败")
	}
	if err := markRestore(restoreBase, rolledbackMark); err != nil {
		return err
	}
	return appendRestoreLog(dataRoot, restoreLogEntry{RestoreID: restoreID, Action: "rolled-back", At: platform.Now()})
}

// restoreActiveFromBefore 换目录中途失败时尽力恢复原状；失败则保持
// 现状由INTEGRITY检查拒绝启动，不做进一步破坏性操作。
func restoreActiveFromBefore(from, to string) error {
	if dirExists(from) && !dirExists(to) {
		return os.Rename(from, to)
	}
	return nil
}

func integrity(message string) error {
	return platform.NewError(platform.CodeIntegrityError, false, "%s", message)
}

// readRestoreLog 读取control区恢复日志；日志损坏拒绝启动。
func readRestoreLog(dataRoot string) ([]restoreLogEntry, error) {
	raw, err := os.ReadFile(filepath.Join(dataRoot, controlDir, restoreLogName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "恢复日志不可读")
	}
	var entries []restoreLogEntry
	for _, line := range splitLines(raw) {
		var entry restoreLogEntry
		if err := json.Unmarshal(line, &entry); err != nil || entry.RestoreID == "" || entry.Action == "" {
			return nil, integrity("恢复日志损坏")
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// logState 取某次恢复的最新日志状态。
func logState(entries []restoreLogEntry, restoreID platform.Identifier) (switched, rolledBack bool) {
	for _, entry := range entries {
		if entry.RestoreID != restoreID {
			continue
		}
		switch entry.Action {
		case "switched":
			switched, rolledBack = true, false
		case "rolled-back":
			switched, rolledBack = false, true
		}
	}
	return switched, rolledBack
}

// appendRestoreLog 追加一行恢复日志并fsync。
func appendRestoreLog(dataRoot string, entry restoreLogEntry) error {
	if err := os.MkdirAll(filepath.Join(dataRoot, controlDir), 0o700); err != nil {
		return platform.NewError(platform.CodeInternal, false, "创建control目录失败")
	}
	file, err := os.OpenFile(filepath.Join(dataRoot, controlDir, restoreLogName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "打开恢复日志失败")
	}
	defer file.Close()
	line, err := json.Marshal(entry)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "编码恢复日志失败")
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		return platform.NewError(platform.CodeInternal, false, "写入恢复日志失败")
	}
	if err := file.Sync(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "恢复日志fsync失败")
	}
	return nil
}

// markRestore 写入阶段标记目录，供WatchRuntime判断已换入。
func markRestore(restoreBase, name string) error {
	if err := os.MkdirAll(filepath.Join(restoreBase, name), 0o700); err != nil {
		return platform.NewError(platform.CodeInternal, false, "写恢复阶段标记失败")
	}
	return nil
}

// syncDir fsync目录项，保证重命名在崩溃后可见。
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
