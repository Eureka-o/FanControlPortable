package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const (
	defaultHandshakeTimeout = 3 * time.Second
	defaultInvokeTimeout    = 10 * time.Second
	defaultSuspendGrace     = 2 * time.Second
	defaultHeartbeatPeriod  = 30 * time.Second
	maxSidecarLineBytes     = 1 << 20
	maxSidecarLogLines      = 100
	crashWindowDuration     = 60 * time.Second
	maxCrashesInWindow      = 3
)

var defaultBackoffDelays = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	10 * time.Second,
}

type sidecarEnvelope struct {
	Type            string          `json:"type"`
	ProtocolVersion int             `json:"protocolVersion,omitempty"`
	PluginID        string          `json:"pluginId,omitempty"`
	Version         string          `json:"version,omitempty"`
	Supported       *bool           `json:"supported,omitempty"`
	Reason          string          `json:"reason,omitempty"`
	Capabilities    []string        `json:"capabilities,omitempty"`
	ID              string          `json:"id,omitempty"`
	Method          string          `json:"method,omitempty"`
	Event           string          `json:"event,omitempty"`
	OK              bool            `json:"ok,omitempty"`
	Error           string          `json:"error,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
}

// SidecarOptions configures a managed out-of-process plugin instance.
type SidecarOptions struct {
	Manifest          Manifest
	PluginDir         string
	DataDir           string
	CoreVersion       string
	InitialSettings   json.RawMessage
	Logger            types.Logger
	OnEvent           func(pluginID, event string, payload json.RawMessage)
	OnStateChange     func()
	HandshakeTimeout  time.Duration
	InvokeTimeout     time.Duration
	HeartbeatInterval time.Duration
	BackoffDelays     []time.Duration
	CommandFactory    func(ctx context.Context, manifest Manifest, pluginDir string) *exec.Cmd
}

// SidecarRunner supervises one out-of-process sidecar plugin via stdin/stdout JSON Lines.
type SidecarRunner struct {
	opts SidecarOptions

	mutex          sync.Mutex
	desiredEnabled bool
	suspended      bool
	suspending     bool
	stopping       bool
	state          string
	running        bool
	reason         string
	lastError      string
	capabilities   []string

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	writeMu sync.Mutex
	job     *jobObjectHandle
	runStop chan struct{}
	loopDone chan struct{}

	pendingMu sync.Mutex
	pending   map[string]chan sidecarEnvelope
	reqSeq    atomic.Uint64

	logMu    sync.Mutex
	logLines []string

	crashTimes []time.Time
	generation uint64
}

func NewSidecarRunner(opts SidecarOptions) *SidecarRunner {
	if opts.HandshakeTimeout <= 0 {
		opts.HandshakeTimeout = defaultHandshakeTimeout
	}
	if opts.InvokeTimeout <= 0 {
		opts.InvokeTimeout = defaultInvokeTimeout
	}
	if opts.HeartbeatInterval == 0 {
		opts.HeartbeatInterval = defaultHeartbeatPeriod
	}
	if len(opts.BackoffDelays) == 0 {
		opts.BackoffDelays = append([]time.Duration(nil), defaultBackoffDelays...)
	}
	return &SidecarRunner{
		opts:         opts,
		state:        StateDisabled,
		capabilities: append([]string(nil), opts.Manifest.Capabilities...),
		pending:      make(map[string]chan sidecarEnvelope),
	}
}

func (r *SidecarRunner) Start() error {
	r.mutex.Lock()
	if r.desiredEnabled && (r.running || r.state == StateStarting || r.state == StateRestarting) {
		r.mutex.Unlock()
		return nil
	}
	r.desiredEnabled = true
	r.suspended = false
	r.suspending = false
	r.stopping = false
	r.lastError = ""
	r.reason = ""
	r.crashTimes = nil
	r.state = StateStarting
	r.generation++
	gen := r.generation
	loopDone := make(chan struct{})
	r.loopDone = loopDone
	r.mutex.Unlock()

	r.notifyChange()
	go func() {
		defer close(loopDone)
		r.launchLoop(gen, 0)
	}()
	return nil
}

func (r *SidecarRunner) Stop() error {
	r.mutex.Lock()
	r.desiredEnabled = false
	r.stopping = true
	r.suspending = false
	r.suspended = false
	r.generation++
	r.state = StateDisabled
	r.running = false
	runStop := r.runStop
	loopDone := r.loopDone
	cmd := r.cmd
	stdin := r.stdin
	job := r.job
	r.cmd = nil
	r.stdin = nil
	r.job = nil
	r.runStop = nil
	r.loopDone = nil
	r.mutex.Unlock()

	if runStop != nil {
		closeSafe(runStop)
	}
	r.sendShutdownAndTerminate(cmd, stdin, job, 1200*time.Millisecond)
	if loopDone != nil {
		select {
		case <-loopDone:
		case <-time.After(2 * time.Second):
		}
	}
	r.failAllPending(errors.New("插件已停止"))
	r.notifyChange()
	return nil
}

func (r *SidecarRunner) Restart() error {
	_ = r.Stop()
	return r.Start()
}

func (r *SidecarRunner) Suspend(grace time.Duration) error {
	if grace <= 0 {
		grace = defaultSuspendGrace
	}
	r.mutex.Lock()
	if !r.desiredEnabled {
		r.suspended = true
		r.mutex.Unlock()
		return nil
	}
	wasRunning := r.running
	r.suspending = true
	r.generation++
	r.state = StateSuspending
	r.mutex.Unlock()
	r.notifyChange()

	if wasRunning {
		ctx, cancel := context.WithTimeout(context.Background(), grace)
		_, _ = r.invokeInternal(ctx, "prepare-suspend", json.RawMessage(`{}`), true)
		cancel()
	}

	r.mutex.Lock()
	runStop := r.runStop
	cmd := r.cmd
	stdin := r.stdin
	job := r.job
	r.cmd = nil
	r.stdin = nil
	r.job = nil
	r.runStop = nil
	r.running = false
	r.suspending = false
	r.suspended = true
	r.state = StateSuspended
	r.mutex.Unlock()

	if runStop != nil {
		closeSafe(runStop)
	}
	r.terminateProcess(cmd, stdin, job)
	r.failAllPending(errors.New("系统休眠，插件已暂停"))
	r.notifyChange()
	return nil
}

func (r *SidecarRunner) Resume() {
	r.mutex.Lock()
	if !r.suspended {
		r.mutex.Unlock()
		return
	}
	r.suspended = false
	shouldStart := r.desiredEnabled && !r.stopping
	if shouldStart {
		r.state = StateStarting
		r.crashTimes = nil
		r.generation++
		gen := r.generation
		loopDone := make(chan struct{})
		r.loopDone = loopDone
		r.mutex.Unlock()
		r.notifyChange()
		go func() {
			defer close(loopDone)
			r.launchLoop(gen, 0)
		}()
		return
	}
	r.state = StateDisabled
	r.mutex.Unlock()
	r.notifyChange()
}

func (r *SidecarRunner) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	return r.invokeInternal(ctx, method, payload, false)
}

func (r *SidecarRunner) SendHostEvent(event string, payload any) error {
	r.mutex.Lock()
	running := r.running && !r.suspending && !r.suspended && !r.stopping
	stdin := r.stdin
	r.mutex.Unlock()
	if !running || stdin == nil {
		return nil
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return r.writeLine(stdin, sidecarEnvelope{
		Type:    "host-event",
		Event:   event,
		Payload: rawPayload,
	})
}

func (r *SidecarRunner) StateSnapshot() (state string, running bool, supported bool, reason, lastError string, caps []string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	supported = r.state != StateUnsupported
	return r.state, r.running, supported, r.reason, r.lastError, append([]string(nil), r.capabilities...)
}

func (r *SidecarRunner) Logs() []string {
	r.logMu.Lock()
	defer r.logMu.Unlock()
	return append([]string(nil), r.logLines...)
}

func (r *SidecarRunner) invokeInternal(ctx context.Context, method string, payload json.RawMessage, allowDuringSuspend bool) (json.RawMessage, error) {
	trimmedMethod := strings.TrimSpace(method)
	if trimmedMethod == "" {
		return nil, fmt.Errorf("方法名不能为空")
	}

	r.mutex.Lock()
	running := r.running
	stdin := r.stdin
	blocked := r.stopping || r.suspended || (r.suspending && !allowDuringSuspend)
	r.mutex.Unlock()

	if blocked {
		return nil, fmt.Errorf("插件当前已暂停或停止")
	}
	if !running || stdin == nil {
		return nil, fmt.Errorf("插件进程尚未就绪")
	}

	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	reqID := fmt.Sprintf("req-%d", r.reqSeq.Add(1))
	ch := make(chan sidecarEnvelope, 1)

	r.pendingMu.Lock()
	r.pending[reqID] = ch
	r.pendingMu.Unlock()
	defer func() {
		r.pendingMu.Lock()
		delete(r.pending, reqID)
		r.pendingMu.Unlock()
	}()

	if err := r.writeLine(stdin, sidecarEnvelope{
		Type:    "request",
		ID:      reqID,
		Method:  trimmedMethod,
		Payload: payload,
	}); err != nil {
		return nil, err
	}

	timeout := r.opts.InvokeTimeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("插件连接已断开")
		}
		if !resp.OK {
			msg := strings.TrimSpace(resp.Error)
			if msg == "" {
				msg = "插件调用返回失败"
			}
			return nil, errors.New(msg)
		}
		if len(resp.Payload) == 0 {
			return json.RawMessage(`{}`), nil
		}
		return resp.Payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, fmt.Errorf("插件调用超时 (%s)", trimmedMethod)
	}
}

func (r *SidecarRunner) launchLoop(gen uint64, attempt int) {
	for {
		r.mutex.Lock()
		if gen != r.generation || !r.desiredEnabled || r.stopping || r.suspended || r.suspending {
			r.mutex.Unlock()
			return
		}
		r.mutex.Unlock()

		retry, nextAttempt, delay := r.runOnce(gen, attempt)
		if !retry {
			return
		}
		attempt = nextAttempt

		timer := time.NewTimer(delay)
		canceled := false
		for !canceled {
			select {
			case <-timer.C:
				canceled = true
			default:
				r.mutex.Lock()
				stillActive := gen == r.generation && r.desiredEnabled && !r.stopping && !r.suspended && !r.suspending
				r.mutex.Unlock()
				if !stillActive {
					timer.Stop()
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
}

func (r *SidecarRunner) runOnce(gen uint64, attempt int) (retry bool, nextAttempt int, delay time.Duration) {
	var cmd *exec.Cmd
	if r.opts.CommandFactory != nil {
		cmd = r.opts.CommandFactory(context.Background(), r.opts.Manifest, r.opts.PluginDir)
	} else {
		exePath := filepath.Clean(filepath.Join(r.opts.PluginDir, filepath.FromSlash(r.opts.Manifest.Backend)))
		cmd = exec.Command(exePath)
		cmd.Dir = r.opts.PluginDir
		configureSidecarSysProcAttr(cmd)
	}

	baseEnv := cmd.Env
	if len(baseEnv) == 0 {
		baseEnv = os.Environ()
	}
	cmd.Env = append(baseEnv,
		"FANCONTROL_PLUGIN_ID="+r.opts.Manifest.ID,
		"FANCONTROL_PLUGIN_DATA_DIR="+r.opts.DataDir,
		"FANCONTROL_CORE_VERSION="+r.opts.CoreVersion,
	)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return r.handleProcessFailure(gen, attempt, fmt.Errorf("创建 stdin 管道失败: %w", err))
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdinPipe.Close()
		return r.handleProcessFailure(gen, attempt, fmt.Errorf("创建 stdout 管道失败: %w", err))
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdinPipe.Close()
		_ = stdoutPipe.Close()
		return r.handleProcessFailure(gen, attempt, fmt.Errorf("创建 stderr 管道失败: %w", err))
	}

	job, jobErr := createKillOnCloseJob()
	if jobErr != nil {
		r.appendLog("创建 JobObject 失败（将继续启动）: %v", jobErr)
	}

	if err := cmd.Start(); err != nil {
		_ = stdinPipe.Close()
		if job != nil {
			job.close()
		}
		return r.handleProcessFailure(gen, attempt, fmt.Errorf("启动插件进程失败: %w", err))
	}
	if job != nil {
		if assignErr := job.assignProcess(cmd); assignErr != nil {
			r.appendLog("绑定进程到 JobObject 失败: %v", assignErr)
		}
	}

	runStop := make(chan struct{})
	r.mutex.Lock()
	if gen != r.generation || !r.desiredEnabled || r.stopping || r.suspended || r.suspending {
		r.mutex.Unlock()
		closeSafe(runStop)
		r.terminateProcess(cmd, stdinPipe, job)
		_ = cmd.Wait()
		return false, attempt, 0
	}
	r.cmd = cmd
	r.stdin = stdinPipe
	r.job = job
	r.runStop = runStop
	r.state = StateStarting
	r.mutex.Unlock()

	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		r.consumeStderr(stderrPipe)
	}()

	lineCh := make(chan string, 32)
	scanErrCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 0, 16*1024), maxSidecarLineBytes)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) != "" {
				lineCh <- line
			}
		}
		scanErr := scanner.Err()
		close(lineCh)
		scanErrCh <- scanErr
	}()

	// Wait for handshake hello within HandshakeTimeout.
	hsTimer := time.NewTimer(r.opts.HandshakeTimeout)
	var helloLine string
	select {
	case line, ok := <-lineCh:
		hsTimer.Stop()
		if !ok {
			_ = cmd.Wait()
			<-stderrDone
			r.clearProcessRefs(gen)
			return r.handleProcessFailure(gen, attempt, errors.New("插件握手前进程退出"))
		}
		helloLine = line
	case <-hsTimer.C:
		r.terminateProcess(cmd, stdinPipe, job)
		_ = cmd.Wait()
		<-stderrDone
		r.clearProcessRefs(gen)
		return r.handleProcessFailure(gen, attempt, fmt.Errorf("插件握手超时 (%s)", r.opts.HandshakeTimeout))
	case <-runStop:
		hsTimer.Stop()
		<-stderrDone
		return false, attempt, 0
	}

	supported, fatalUnsupported, hsErr := r.processHandshake(helloLine)
	if hsErr != nil {
		r.terminateProcess(cmd, stdinPipe, job)
		_ = cmd.Wait()
		<-stderrDone
		r.clearProcessRefs(gen)
		if fatalUnsupported {
			r.mutex.Lock()
			if gen == r.generation {
				r.desiredEnabled = false
				r.running = false
				r.state = StateUnsupported
				r.reason = hsErr.Error()
			}
			r.mutex.Unlock()
			r.notifyChange()
			return false, attempt, 0
		}
		if !supported {
			return false, attempt, 0
		}
		return r.handleProcessFailure(gen, attempt, hsErr)
	}

	r.mutex.Lock()
	if gen == r.generation && r.desiredEnabled && !r.stopping && !r.suspended && !r.suspending {
		r.running = true
		r.state = StateReady
		r.lastError = ""
		r.reason = ""
	}
	r.mutex.Unlock()
	r.notifyChange()

	if len(r.opts.InitialSettings) > 0 {
		_ = r.SendHostEvent("settings-changed", r.opts.InitialSettings)
	}

	if r.opts.HeartbeatInterval > 0 {
		go r.heartbeatLoop(gen, runStop)
	}

	for line := range lineCh {
		r.handleStdoutLine(line)
	}

	scanErr := <-scanErrCh
	waitErr := cmd.Wait()
	<-stderrDone

	r.clearProcessRefs(gen)
	r.failAllPending(errors.New("插件进程已退出"))

	r.mutex.Lock()
	expectedStop := gen != r.generation || !r.desiredEnabled || r.stopping || r.suspended || r.suspending
	r.mutex.Unlock()
	if expectedStop {
		return false, attempt, 0
	}

	exitErr := waitErr
	if exitErr == nil && scanErr != nil {
		exitErr = scanErr
	}
	if exitErr == nil {
		exitErr = errors.New("插件进程意外退出")
	}
	return r.handleProcessFailure(gen, attempt, exitErr)
}

func (r *SidecarRunner) processHandshake(line string) (supported bool, fatalUnsupported bool, err error) {
	var msg sidecarEnvelope
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return true, false, fmt.Errorf("插件握手 JSON 无效: %w", err)
	}
	if msg.Type != "hello" {
		return true, false, fmt.Errorf("插件首条消息必须为 hello，收到: %s", msg.Type)
	}
	if msg.ProtocolVersion != SupportedProtocolVersion {
		return true, false, fmt.Errorf("插件协议版本不匹配: %d", msg.ProtocolVersion)
	}
	if msg.PluginID != r.opts.Manifest.ID {
		return true, false, fmt.Errorf("插件握手 ID 不匹配: %s", msg.PluginID)
	}
	if msg.Supported != nil && !*msg.Supported {
		reason := strings.TrimSpace(msg.Reason)
		if reason == "" {
			reason = "当前硬件或系统不支持此插件"
		}
		return false, true, errors.New(reason)
	}
	if len(msg.Capabilities) > 0 {
		r.mutex.Lock()
		r.capabilities = append([]string(nil), msg.Capabilities...)
		r.mutex.Unlock()
	}
	return true, false, nil
}

func (r *SidecarRunner) handleStdoutLine(line string) {
	var msg sidecarEnvelope
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		r.appendLog("无效的 stdout JSON: %v", err)
		return
	}
	switch msg.Type {
	case "response":
		if msg.ID == "" {
			return
		}
		r.pendingMu.Lock()
		ch := r.pending[msg.ID]
		r.pendingMu.Unlock()
		if ch != nil {
			select {
			case ch <- msg:
			default:
			}
		}
	case "event":
		if msg.Event == "" || r.opts.OnEvent == nil {
			return
		}
		r.opts.OnEvent(r.opts.Manifest.ID, msg.Event, msg.Payload)
	}
}

func (r *SidecarRunner) heartbeatLoop(gen uint64, runStop <-chan struct{}) {
	ticker := time.NewTicker(r.opts.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-runStop:
			return
		case <-ticker.C:
			r.mutex.Lock()
			ok := gen == r.generation && r.running && !r.suspending && !r.suspended && !r.stopping
			r.mutex.Unlock()
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, _ = r.invokeInternal(ctx, "ping", json.RawMessage(`{}`), false)
			cancel()
		}
	}
}

func (r *SidecarRunner) handleProcessFailure(gen uint64, attempt int, err error) (bool, int, time.Duration) {
	now := time.Now()
	r.appendLog("运行异常: %v", err)

	r.mutex.Lock()
	if gen != r.generation || !r.desiredEnabled || r.stopping || r.suspended || r.suspending {
		r.mutex.Unlock()
		return false, attempt, 0
	}

	cutoff := now.Add(-crashWindowDuration)
	kept := r.crashTimes[:0]
	for _, ts := range r.crashTimes {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	kept = append(kept, now)
	r.crashTimes = kept
	r.running = false
	r.lastError = err.Error()

	if len(r.crashTimes) >= maxCrashesInWindow {
		r.desiredEnabled = false
		r.state = StateFailed
		r.mutex.Unlock()
		r.notifyChange()
		return false, attempt, 0
	}

	r.state = StateRestarting
	delays := r.opts.BackoffDelays
	idx := attempt
	if idx >= len(delays) {
		idx = len(delays) - 1
	}
	delay := delays[idx]
	r.mutex.Unlock()

	r.notifyChange()
	return true, attempt + 1, delay
}

func (r *SidecarRunner) clearProcessRefs(gen uint64) {
	r.mutex.Lock()
	if gen == r.generation {
		if r.runStop != nil {
			closeSafe(r.runStop)
			r.runStop = nil
		}
		if r.job != nil {
			r.job.close()
			r.job = nil
		}
		r.cmd = nil
		r.stdin = nil
		r.running = false
	}
	r.mutex.Unlock()
}

func (r *SidecarRunner) writeLine(stdin io.WriteCloser, msg sidecarEnvelope) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err = stdin.Write(raw)
	return err
}

func (r *SidecarRunner) consumeStderr(stderr io.ReadCloser) {
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 8*1024), 64*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			r.appendLog("%s", line)
		}
	}
}

func (r *SidecarRunner) appendLog(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if r.opts.Logger != nil {
		r.opts.Logger.Info("[plugin:%s] %s", r.opts.Manifest.ID, msg)
	}
	r.logMu.Lock()
	if len(r.logLines) >= maxSidecarLogLines {
		copy(r.logLines, r.logLines[1:])
		r.logLines[len(r.logLines)-1] = msg
	} else {
		r.logLines = append(r.logLines, msg)
	}
	r.logMu.Unlock()
}

func (r *SidecarRunner) sendShutdownAndTerminate(cmd *exec.Cmd, stdin io.WriteCloser, job *jobObjectHandle, grace time.Duration) {
	if cmd == nil {
		if job != nil {
			job.close()
		}
		return
	}
	if stdin != nil {
		ctx, cancel := context.WithTimeout(context.Background(), grace)
		reqID := fmt.Sprintf("stop-%d", r.reqSeq.Add(1))
		ch := make(chan sidecarEnvelope, 1)
		r.pendingMu.Lock()
		r.pending[reqID] = ch
		r.pendingMu.Unlock()
		_ = r.writeLine(stdin, sidecarEnvelope{Type: "request", ID: reqID, Method: "shutdown", Payload: json.RawMessage(`{}`)})
		select {
		case <-ch:
		case <-ctx.Done():
		}
		cancel()
		r.pendingMu.Lock()
		delete(r.pending, reqID)
		r.pendingMu.Unlock()
	}
	r.terminateProcess(cmd, stdin, job)
}

func (r *SidecarRunner) terminateProcess(cmd *exec.Cmd, stdin io.WriteCloser, job *jobObjectHandle) {
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if job != nil {
		job.close()
	}
}

func (r *SidecarRunner) failAllPending(err error) {
	r.pendingMu.Lock()
	pending := r.pending
	r.pending = make(map[string]chan sidecarEnvelope)
	r.pendingMu.Unlock()

	for _, ch := range pending {
		select {
		case ch <- sidecarEnvelope{Type: "response", OK: false, Error: err.Error()}:
		default:
		}
		close(ch)
	}
}

func (r *SidecarRunner) notifyChange() {
	if r.opts.OnStateChange != nil {
		r.opts.OnStateChange()
	}
}

func closeSafe(ch chan struct{}) {
	defer func() {
		_ = recover()
	}()
	close(ch)
}
