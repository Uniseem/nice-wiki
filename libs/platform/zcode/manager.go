package zcode

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"nice-wiki/libs/platform"
)

type execution struct {
	owner   string
	frames  []Frame
	pending map[string]bool
	changed chan struct{}
	done    bool
	stop    func() bool
}

// Manager 承载同一 Node 进程里的独立原生运行时，不接触业务表或模型凭据。
type Manager struct {
	mu                sync.Mutex
	writeMu           sync.Mutex
	worker, directory string
	command           *exec.Cmd
	input             io.WriteCloser
	encoder           *json.Encoder
	runs              map[string]*execution
	closed            bool
}

func NewManager(worker, directory string) *Manager {
	return &Manager{worker: worker, directory: directory, runs: map[string]*execution{}}
}

func (m *Manager) Ready() error {
	if _, err := exec.LookPath("node"); err != nil {
		return unavailable()
	}
	if _, err := os.Stat(m.worker); err != nil {
		return unavailable()
	}
	return nil
}

func unavailable() error {
	return platform.NewError(platform.CodeDependencyUnavailable, true, "原生 Agent 运行时不可用")
}

func (m *Manager) startProcess() error {
	if m.closed {
		return unavailable()
	}
	if m.command != nil {
		return nil
	}
	if err := m.Ready(); err != nil {
		return err
	}
	command := exec.Command("node", m.worker)
	command.Dir = m.directory
	for _, name := range []string{"PATH", "SystemRoot", "TEMP", "TMP", "TZ", "NODE_OPTIONS", "NW_DEBUG"} {
		if value, ok := os.LookupEnv(name); ok {
			command.Env = append(command.Env, name+"="+value)
		}
	}
	input, err := command.StdinPipe()
	if err != nil {
		return unavailable()
	}
	output, err := command.StdoutPipe()
	if err != nil {
		_ = input.Close()
		return unavailable()
	}
	// IPC 承载必要上下文；第三方诊断不写进服务日志，避免记录正文或提示词。
	command.Stderr = io.Discard
	if err = command.Start(); err != nil {
		_ = input.Close()
		return unavailable()
	}
	m.command, m.input, m.encoder = command, input, json.NewEncoder(input)
	go m.readProcess(command, output)
	return nil
}

func (m *Manager) readProcess(command *exec.Cmd, output io.Reader) {
	decoder := json.NewDecoder(output)
	for {
		var frame Frame
		if err := decoder.Decode(&frame); err != nil {
			break
		}
		m.mu.Lock()
		if run := m.runs[frame.RunID]; run != nil && !run.done {
			if frame.Kind == "invoke" || frame.Kind == "tool" {
				run.pending[frame.ID] = true
			}
			m.append(run, frame)
		}
		m.mu.Unlock()
	}
	_ = command.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.command != command {
		return
	}
	m.command, m.input, m.encoder = nil, nil, nil
	for id, run := range m.runs {
		if !run.done {
			m.append(run, Frame{RunID: id, Kind: "failed", Payload: `{"code":"DEPENDENCY_UNAVAILABLE"}`})
		}
	}
}

func (m *Manager) append(run *execution, frame Frame) {
	frame.Sequence = len(run.frames) + 1
	run.frames = append(run.frames, frame)
	if frame.Kind == "done" || frame.Kind == "failed" {
		run.done = true
	}
	close(run.changed)
	run.changed = make(chan struct{})
}

func (m *Manager) write(frame Frame) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	m.mu.Lock()
	encoder := m.encoder
	m.mu.Unlock()
	if encoder == nil || encoder.Encode(frame) != nil {
		return unavailable()
	}
	return nil
}

// Start 的最长生命周期来自原操作截止；已完成结果由 owning-service 取走后释放。
func (m *Manager) Start(owner string, input Input, deadline time.Time) (string, error) {
	m.mu.Lock()
	if err := m.startProcess(); err != nil {
		m.mu.Unlock()
		return "", err
	}
	id := platform.NewIdentifier()
	run := &execution{owner: owner, pending: map[string]bool{}, changed: make(chan struct{}), stop: func() bool { return false }}
	m.runs[id] = run
	m.mu.Unlock()
	if err := m.write(Frame{RunID: id, Kind: "start", Payload: payload(input)}); err != nil {
		_ = m.Cancel(owner, id)
		return "", err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return "", unavailable()
	}
	run.stop = time.AfterFunc(time.Until(deadline), func() { _ = m.Cancel(owner, id) }).Stop
	m.mu.Unlock()
	return id, nil
}

func (m *Manager) owned(owner, id string) (*execution, error) {
	run := m.runs[id]
	if run == nil {
		return nil, platform.NewError(platform.CodeNotFound, false, "Agent 运行不存在")
	}
	if run.owner != owner {
		return nil, platform.NewError(platform.CodeForbidden, false, "Agent 运行不属于当前操作")
	}
	return run, nil
}

func (m *Manager) Read(ctx context.Context, owner, id string, after int) (Events, error) {
	for {
		m.mu.Lock()
		run, err := m.owned(owner, id)
		if err != nil {
			m.mu.Unlock()
			return Events{}, err
		}
		if after > len(run.frames) {
			m.mu.Unlock()
			return Events{}, platform.NewError(platform.CodeValidation, false, "Agent 事件游标超出范围")
		}
		if after < len(run.frames) || run.done {
			events := Events{Frames: append([]Frame{}, run.frames[after:]...), Done: run.done}
			m.mu.Unlock()
			return events, nil
		}
		changed := run.changed
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return Events{}, ctx.Err()
		case <-changed:
		}
	}
}

func (m *Manager) Reply(owner, id, request string, reply Reply) error {
	m.mu.Lock()
	run, err := m.owned(owner, id)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	if !run.pending[request] || run.done {
		m.mu.Unlock()
		return platform.NewError(platform.CodeConflict, false, "Agent 请求已结束")
	}
	delete(run.pending, request)
	m.mu.Unlock()
	return m.write(Frame{RunID: id, Kind: "reply", ID: request, Payload: reply.Payload})
}

func (m *Manager) Cancel(owner, id string) error {
	m.mu.Lock()
	run, err := m.owned(owner, id)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	if !run.done {
		m.append(run, Frame{RunID: id, Kind: "failed", Payload: `{"code":"CANCELLED"}`})
	}
	run.stop()
	delete(m.runs, id)
	m.mu.Unlock()
	return m.write(Frame{RunID: id, Kind: "cancel", Payload: "{}"})
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	command, input := m.command, m.input
	for id, run := range m.runs {
		run.stop()
		if !run.done {
			m.append(run, Frame{RunID: id, Kind: "failed", Payload: `{"code":"CANCELLED"}`})
		}
	}
	m.mu.Unlock()
	if input != nil {
		_ = input.Close()
	}
	if command != nil && command.Process != nil {
		_ = command.Process.Kill()
	}
	return nil
}
