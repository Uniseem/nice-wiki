package datadir

import (
	"errors"
	"github.com/gofrs/flock"
)

type Lock struct{ file *flock.Flock }

func (l Layout) Acquire() (*Lock, error) {
	file := flock.New(l.LockFile())
	ok, err := file.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("数据目录已被其他进程使用")
	}
	return &Lock{file: file}, nil
}
func (l *Lock) Close() error { return l.file.Unlock() }
