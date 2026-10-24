//go:build !windows

package lifecycle

import (
	"os"
	"syscall"
)

// OpenFileNoFollow 只读打开最终路径，拒绝符号链接；调用方继续校验打开前后的文件身份。
func OpenFileNoFollow(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

// ReadFilesystemSpace 返回调用进程实际可用空间与文件系统总空间，单位为字节。
func ReadFilesystemSpace(root string) (uint64, uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(root, &stat); err != nil {
		return 0, 0, err
	}
	if stat.Bsize <= 0 {
		return 0, 0, integrity("文件系统空间单位未知")
	}
	return stat.Bavail * uint64(stat.Bsize), stat.Blocks * uint64(stat.Bsize), nil
}

func checkSingleFileLink(_ string, info os.FileInfo) error {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Nlink != 1 {
		return integrity("私有权威文件存在硬链接")
	}
	return nil
}

func checkSnapshotDiskSpace(root string, required int64) error {
	available, _, err := ReadFilesystemSpace(root)
	if err != nil {
		return snapshotIOError(err)
	}
	if available < uint64(required) {
		return snapshotBudget("快照隔离空间不足")
	}
	return nil
}
