package lifecycle

import (
	"golang.org/x/sys/windows"
	"os"
)

// OpenFileNoFollow 使用重解析点方式打开；不能让凭据路径在打开时跳转到链接目标。
func OpenFileNoFollow(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(handle)
		return nil, integrity("私有文件路径不能是重解析点")
	}
	return os.NewFile(uintptr(handle), path), nil
}

// ReadFilesystemSpace 返回Windows实际可用空间，不假定磁盘容量无限。
func ReadFilesystemSpace(root string) (uint64, uint64, error) {
	path, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return 0, 0, err
	}
	var available, total uint64
	if err := windows.GetDiskFreeSpaceEx(path, &available, &total, nil); err != nil {
		return 0, 0, err
	}
	return available, total, nil
}

func checkSingleFileLink(path string, _ os.FileInfo) error {
	file, err := os.Open(path)
	if err != nil {
		return snapshotIOError(err)
	}
	defer file.Close()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return snapshotIOError(err)
	}
	if info.NumberOfLinks != 1 {
		return integrity("私有权威文件存在硬链接")
	}
	return nil
}
