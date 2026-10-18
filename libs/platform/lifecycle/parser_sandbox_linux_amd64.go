//go:build linux && amd64

package lifecycle

import (
	"golang.org/x/sys/unix"
	"nice-wiki/libs/platform"
	"unsafe"
)

// EnterParserSandbox 对全部现有Go线程安装不可放宽的过滤器；新线程继承过滤器。
// 固定amd64 ABI；禁止文件打开、网络、执行其他程序及跨进程读取，失败即拒绝解析。
func EnterParserSandbox() error {
	for resource, limit := range map[int]uint64{unix.RLIMIT_CPU: 30, unix.RLIMIT_DATA: 512 << 20, unix.RLIMIT_FSIZE: 0, unix.RLIMIT_CORE: 0, unix.RLIMIT_NOFILE: 64} {
		if err := unix.Setrlimit(resource, &unix.Rlimit{Cur: limit, Max: limit}); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, false, "无法约束解析进程资源")
		}
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "无法约束解析进程权限")
	}
	filters := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 4},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: unix.AUDIT_ARCH_X86_64, Jt: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_KILL_PROCESS},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		// x32使用同一audit arch，必须拒绝该syscall编号空间以免绕过amd64规则。
		{Code: unix.BPF_JMP | unix.BPF_JGE | unix.BPF_K, K: 0x40000000, Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_KILL_PROCESS},
	}
	// Go线程创建只允许CLONE_THREAD，禁止fork型进程逃离父进程超时控制。
	filters = append(filters,
		unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: unix.SYS_CLONE, Jf: 5},
		unix.SockFilter{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 16},
		unix.SockFilter{Code: unix.BPF_ALU | unix.BPF_AND | unix.BPF_K, K: unix.CLONE_THREAD},
		unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: unix.CLONE_THREAD, Jf: 1},
		unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW},
		unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)},
	)
	// 封闭允许集只含Go运行时及三个标准管道所需调用；没有路径IO、网络、
	// 新程序、io_uring、跨进程读取、变更资源上限或命名空间的调用。
	allowed := []uint32{unix.SYS_READ, unix.SYS_WRITE, unix.SYS_READV, unix.SYS_WRITEV, unix.SYS_CLOSE, unix.SYS_FSTAT,
		unix.SYS_FCNTL, unix.SYS_IOCTL, unix.SYS_LSEEK, unix.SYS_PREAD64,
		unix.SYS_MMAP, unix.SYS_MPROTECT, unix.SYS_MUNMAP, unix.SYS_MREMAP, unix.SYS_MADVISE, unix.SYS_BRK,
		unix.SYS_FUTEX, unix.SYS_SCHED_YIELD, unix.SYS_SCHED_GETAFFINITY, unix.SYS_SCHED_GETPARAM, unix.SYS_GETRLIMIT,
		unix.SYS_RT_SIGACTION, unix.SYS_RT_SIGPROCMASK, unix.SYS_RT_SIGRETURN, unix.SYS_SIGALTSTACK, unix.SYS_TGKILL,
		unix.SYS_GETPID, unix.SYS_GETTID, unix.SYS_GETRANDOM, unix.SYS_CLOCK_GETTIME, unix.SYS_GETTIMEOFDAY,
		unix.SYS_NANOSLEEP, unix.SYS_CLOCK_NANOSLEEP, unix.SYS_RESTART_SYSCALL, unix.SYS_SET_ROBUST_LIST, unix.SYS_RSEQ,
		unix.SYS_EPOLL_CREATE1, unix.SYS_EPOLL_CTL, unix.SYS_EPOLL_PWAIT, unix.SYS_EVENTFD2, unix.SYS_POLL, unix.SYS_PPOLL,
		unix.SYS_PIPE2, unix.SYS_DUP, unix.SYS_DUP2, unix.SYS_DUP3, unix.SYS_EXIT, unix.SYS_EXIT_GROUP}
	for _, number := range allowed {
		filters = append(filters, unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: number, Jf: 1}, unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW})
	}
	filters = append(filters, unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)})
	program := unix.SockFprog{Len: uint16(len(filters)), Filter: &filters[0]}
	result, _, errno := unix.Syscall(unix.SYS_SECCOMP, unix.SECCOMP_SET_MODE_FILTER, unix.SECCOMP_FILTER_FLAG_TSYNC, uintptr(unsafe.Pointer(&program)))
	if errno != 0 || result != 0 {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "无法隔离解析进程系统调用")
	}
	return nil
}
