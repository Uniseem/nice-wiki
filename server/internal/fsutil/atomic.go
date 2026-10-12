package fsutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WriteFile 将完整内容落盘后在同目录原子替换目标；目录同步失败也会报告。
func WriteFile(path string, data []byte, mode os.FileMode) (err error) {
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建父目录: %w", err)
	}
	file, err := os.CreateTemp(dir, ".nw-write-*")
	if err != nil {
		return fmt.Errorf("创建临时文件: %w", err)
	}
	name := file.Name()
	defer func() {
		if file != nil {
			err = errors.Join(err, file.Close())
		}
		if removeErr := os.Remove(name); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	if err = file.Chmod(mode); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		file = nil
		return err
	}
	file = nil
	if err = os.Rename(name, path); err != nil {
		return fmt.Errorf("替换文件: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 JSON: %w", err)
	}
	return WriteFile(path, append(data, '\n'), 0600)
}

// ReadJSON 拒绝尾随数据，避免配置文件被部分解析。
func ReadJSON(path string, value any) (err error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	decoder := json.NewDecoder(io.LimitReader(file, 32<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("读取 JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("JSON 含尾随数据或超过大小限制")
	}
	return nil
}

// EnsureJSON 仅首次创建，已有文件不会被默认值覆盖。
func EnsureJSON(path string, value any) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return WriteJSON(path, value)
}
