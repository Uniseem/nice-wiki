package datadir

import (
	"errors"
	"github.com/gofrs/flock"
	"sync"
)

// Lock 包装进程锁并保证关闭幂等。启动失败清理和正常关闭可能沿不同路径
// 汇合，幂等关闭可避免重复 Unlock 把真实的关闭错误掩盖掉。
type Lock struct {
	file *flock.Flock
	once sync.Once
	err  error
}

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

func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	l.once.Do(func() { l.err = l.file.Unlock() })
	return l.err
}
