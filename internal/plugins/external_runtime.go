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

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	defaultPluginHandshakeTimeout = 3 * time.Second
	defaultPluginRequestTimeout   = 5 * time.Second
	defaultPluginStopTimeout      = 2 * time.Second
	pluginExitGrace               = 500 * time.Millisecond
	maxConcurrentPluginRequests   = 16
)

type CommandFactory func(ctx context.Context, spec RuntimeSpec) *exec.Cmd

type ProcessRuntimeOptions struct {
	Logger           types.Logger
	DataDir          string
	CommandFactory   CommandFactory
	HandshakeTimeout time.Duration
	RequestTimeout   time.Duration
	StopTimeout      time.Duration
	OnStatus         func(RuntimeStatus)
	OnEvent          func(RuntimeEvent)
}

type processWriteRequest struct {
	data []byte
	done chan error
}

type processHelloResult struct {
	message protocolEnvelope
	err     error
}

type processSession struct {
	ctx             context.Context
	cancel          context.CancelFunc
	cmd             *exec.Cmd
	stdin           io.WriteCloser
	stdinCloseOnce  sync.Once
	controlCh       chan processWriteRequest
	telemetryCh     chan TelemetrySnapshot
	helloCh         chan processHelloResult
	done            chan struct{}
	helloSeen       atomic.Bool
	helloAccepted   atomic.Bool
	stopRequested   atomic.Bool
	pendingMutex    sync.Mutex
	pendingRequests map[string]chan protocolEnvelope
}

type ProcessRuntime struct {
	spec RuntimeSpec

	logger           types.Logger
	dataDir          string
	commandFactory   CommandFactory
	handshakeTimeout time.Duration
	requestTimeout   time.Duration
	stopTimeout      time.Duration
	onStatus         func(RuntimeStatus)
	onEvent          func(RuntimeEvent)

	lifecycleMutex sync.Mutex
	mutex          sync.RWMutex
	session        *processSession
	state          CatalogState
	lastError      string
	capabilities   []string
	requestID      atomic.Uint64
	requestSlots   chan struct{}
}

func NewProcessRuntime(spec RuntimeSpec, options ProcessRuntimeOptions) *ProcessRuntime {
	commandFactory := options.CommandFactory
	if commandFactory == nil {
		commandFactory = defaultPluginCommand
	}
	return &ProcessRuntime{
		spec:             cloneRuntimeSpec(spec),
		logger:           options.Logger,
		dataDir:          filepath.Clean(options.DataDir),
		commandFactory:   commandFactory,
		handshakeTimeout: positiveDuration(options.HandshakeTimeout, defaultPluginHandshakeTimeout),
		requestTimeout:   positiveDuration(options.RequestTimeout, defaultPluginRequestTimeout),
		stopTimeout:      positiveDuration(options.StopTimeout, defaultPluginStopTimeout),
		onStatus:         options.OnStatus,
		onEvent:          options.OnEvent,
		state:            CatalogStateDisabled,
		requestSlots:     make(chan struct{}, maxConcurrentPluginRequests),
	}
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func defaultPluginCommand(ctx context.Context, spec RuntimeSpec) *exec.Cmd {
	cmd := exec.CommandContext(ctx, spec.BackendPath)
	cmd.Dir = spec.PluginDir
	configureExternalPluginCommand(cmd)
	return cmd
}

func (runtime *ProcessRuntime) Spec() RuntimeSpec {
	return cloneRuntimeSpec(runtime.spec)
}

func (runtime *ProcessRuntime) MatchesSpec(spec RuntimeSpec) bool {
	return runtimeSpecKey(runtime.spec) == runtimeSpecKey(spec)
}

func (runtime *ProcessRuntime) Start(parent context.Context) error {
	runtime.lifecycleMutex.Lock()
	defer runtime.lifecycleMutex.Unlock()

	runtime.mutex.RLock()
	currentSession := runtime.session
	currentState := runtime.state
	runtime.mutex.RUnlock()
	if currentSession != nil && (currentState == CatalogStateStarting || currentState == CatalogStateReady || currentState == CatalogStateUnsupported) {
		return nil
	}
	if currentSession != nil {
		select {
		case <-currentSession.done:
			runtime.clearSession(currentSession)
		case <-time.After(pluginExitGrace):
			return fmt.Errorf("previous plugin process is still stopping")
		}
	}
	if parent == nil {
		parent = context.Background()
	}
	if err := os.MkdirAll(runtime.dataDir, 0o755); err != nil {
		return runtime.startFailure(nil, fmt.Errorf("create plugin data directory: %w", err))
	}

	ctx, cancel := context.WithCancel(parent)
	cmd := runtime.commandFactory(ctx, runtime.spec)
	if cmd == nil {
		cancel()
		return runtime.startFailure(nil, fmt.Errorf("plugin command factory returned nil"))
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return runtime.startFailure(nil, fmt.Errorf("open plugin stdin: %w", err))
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return runtime.startFailure(nil, fmt.Errorf("open plugin stdout: %w", err))
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return runtime.startFailure(nil, fmt.Errorf("open plugin stderr: %w", err))
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return runtime.startFailure(nil, fmt.Errorf("start plugin backend: %w", err))
	}

	session := &processSession{
		ctx:             ctx,
		cancel:          cancel,
		cmd:             cmd,
		stdin:           stdin,
		controlCh:       make(chan processWriteRequest, 32),
		telemetryCh:     make(chan TelemetrySnapshot, 1),
		helloCh:         make(chan processHelloResult, 1),
		done:            make(chan struct{}),
		pendingRequests: make(map[string]chan protocolEnvelope),
	}
	runtime.mutex.Lock()
	runtime.session = session
	runtime.mutex.Unlock()
	runtime.updateState(CatalogStateStarting, "", nil)

	go runtime.writerLoop(session)
	go runtime.readerLoop(session, stdout)
	go runtime.stderrLoop(session, stderr)
	go runtime.waitLoop(session)

	timer := time.NewTimer(runtime.handshakeTimeout)
	defer timer.Stop()
	var hello protocolEnvelope
	select {
	case result := <-session.helloCh:
		if result.err != nil {
			return runtime.startFailure(session, result.err)
		}
		hello = result.message
	case <-session.done:
		return runtime.startFailure(session, fmt.Errorf("plugin exited before handshake"))
	case <-timer.C:
		return runtime.startFailure(session, fmt.Errorf("plugin handshake timed out after %s", runtime.handshakeTimeout))
	case <-parent.Done():
		return runtime.startFailure(session, parent.Err())
	}

	capabilities, supported, reason, err := runtime.validateHello(hello)
	if err != nil {
		return runtime.startFailure(session, err)
	}
	session.helloAccepted.Store(true)
	hostInit := hostInitMessage{
		Type:                "host-init",
		ProtocolVersion:     ExternalProtocolVersion,
		InstanceID:          fmt.Sprintf("%s-%d", runtime.spec.ID, time.Now().UnixNano()),
		DataDir:             runtime.dataDir,
		HeartbeatIntervalMS: 2000,
	}
	initContext, initCancel := context.WithTimeout(parent, runtime.handshakeTimeout)
	err = runtime.writeControl(initContext, session, hostInit)
	initCancel()
	if err != nil {
		return runtime.startFailure(session, fmt.Errorf("send host-init: %w", err))
	}
	if !supported {
		runtime.updateState(CatalogStateUnsupported, reason, capabilities)
		return nil
	}
	runtime.updateState(CatalogStateReady, "", capabilities)
	return nil
}

func (runtime *ProcessRuntime) startFailure(session *processSession, err error) error {
	if err == nil {
		err = fmt.Errorf("plugin failed to start")
	}
	if session != nil {
		session.stopRequested.Store(true)
		runtime.closeSessionInput(session)
		session.cancel()
		if session.cmd.Process != nil {
			_ = session.cmd.Process.Kill()
		}
		select {
		case <-session.done:
		case <-time.After(pluginExitGrace):
		}
		runtime.clearSession(session)
	}
	runtime.updateState(CatalogStateFailed, err.Error(), nil)
	return err
}

func (runtime *ProcessRuntime) validateHello(hello protocolEnvelope) ([]string, bool, string, error) {
	if hello.Type != "hello" {
		return nil, false, "", fmt.Errorf("expected hello message")
	}
	if hello.ProtocolVersion != ExternalProtocolVersion {
		return nil, false, "", fmt.Errorf("plugin protocol %d does not match %d", hello.ProtocolVersion, ExternalProtocolVersion)
	}
	if hello.PluginID != runtime.spec.ID {
		return nil, false, "", fmt.Errorf("plugin hello id %q does not match %q", hello.PluginID, runtime.spec.ID)
	}
	if hello.Version != runtime.spec.Version {
		return nil, false, "", fmt.Errorf("plugin hello version %q does not match %q", hello.Version, runtime.spec.Version)
	}
	declared := make(map[string]struct{}, len(runtime.spec.Capabilities))
	for _, capability := range runtime.spec.Capabilities {
		declared[capability] = struct{}{}
	}
	capabilities := make([]string, 0, len(hello.Capabilities))
	for _, capability := range hello.Capabilities {
		if !validProtocolName(capability) {
			return nil, false, "", fmt.Errorf("invalid runtime capability %q", capability)
		}
		if _, ok := declared[capability]; !ok {
			return nil, false, "", fmt.Errorf("runtime capability %q was not declared", capability)
		}
		capabilities = append(capabilities, capability)
	}
	supported := hello.Supported == nil || *hello.Supported
	reason := strings.TrimSpace(hello.Reason)
	if len(reason) > 1024 {
		reason = reason[:1024]
	}
	if !supported && reason == "" {
		reason = "plugin reported unsupported hardware"
	}
	return capabilities, supported, reason, nil
}

func (runtime *ProcessRuntime) Invoke(ctx context.Context, method string, payload any) (json.RawMessage, error) {
	runtime.mutex.RLock()
	session := runtime.session
	state := runtime.state
	runtime.mutex.RUnlock()
	if session == nil || state != CatalogStateReady || session.stopRequested.Load() {
		return nil, fmt.Errorf("plugin %s is not ready", runtime.spec.ID)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, runtime.requestTimeout)
		defer cancel()
	}
	return runtime.invokeSession(ctx, session, method, payload)
}

func (runtime *ProcessRuntime) invokeSession(ctx context.Context, session *processSession, method string, payload any) (json.RawMessage, error) {
	if !validProtocolName(method) {
		return nil, fmt.Errorf("invalid plugin method %q", method)
	}
	select {
	case runtime.requestSlots <- struct{}{}:
		defer func() { <-runtime.requestSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	payloadBytes, err := marshalProtocolPayload(payload)
	if err != nil {
		return nil, err
	}
	requestID := fmt.Sprintf("%d", runtime.requestID.Add(1))
	responseCh := make(chan protocolEnvelope, 1)
	session.pendingMutex.Lock()
	session.pendingRequests[requestID] = responseCh
	session.pendingMutex.Unlock()
	defer func() {
		session.pendingMutex.Lock()
		delete(session.pendingRequests, requestID)
		session.pendingMutex.Unlock()
	}()

	request := protocolRequest{Type: "request", ID: requestID, Method: method, Payload: payloadBytes}
	if err := runtime.writeControl(ctx, session, request); err != nil {
		return nil, err
	}
	select {
	case response := <-responseCh:
		return protocolResponseResult(response)
	case <-session.done:
		select {
		case response := <-responseCh:
			return protocolResponseResult(response)
		default:
		}
		return nil, fmt.Errorf("plugin process exited")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func protocolResponseResult(response protocolEnvelope) (json.RawMessage, error) {
	if !response.OK {
		if response.Error == "" {
			response.Error = "plugin request failed"
		}
		return nil, errors.New(response.Error)
	}
	if len(response.Payload) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return append(json.RawMessage(nil), response.Payload...), nil
}

func marshalProtocolPayload(payload any) (json.RawMessage, error) {
	if payload == nil {
		return json.RawMessage(`{}`), nil
	}
	if raw, ok := payload.(json.RawMessage); ok {
		if len(raw) == 0 {
			return json.RawMessage(`{}`), nil
		}
		if !json.Valid(raw) {
			return nil, fmt.Errorf("plugin payload is not valid JSON")
		}
		return append(json.RawMessage(nil), raw...), nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode plugin payload: %w", err)
	}
	return data, nil
}

func (runtime *ProcessRuntime) SubmitTelemetry(snapshot TelemetrySnapshot) {
	filtered, ok := snapshot.Filter(runtime.spec.TelemetryInputs)
	if !ok {
		return
	}
	runtime.mutex.RLock()
	session := runtime.session
	ready := runtime.state == CatalogStateReady
	runtime.mutex.RUnlock()
	if session == nil || !ready || session.stopRequested.Load() {
		return
	}
	offerLatestTelemetry(session.telemetryCh, filtered)
}

func (runtime *ProcessRuntime) AcceptsTelemetry() bool {
	if len(runtime.spec.TelemetryInputs) == 0 {
		return false
	}
	runtime.mutex.RLock()
	defer runtime.mutex.RUnlock()
	return runtime.session != nil && runtime.state == CatalogStateReady && !runtime.session.stopRequested.Load()
}

func offerLatestTelemetry(channel chan TelemetrySnapshot, snapshot TelemetrySnapshot) {
	select {
	case channel <- snapshot:
		return
	default:
	}
	select {
	case <-channel:
	default:
	}
	select {
	case channel <- snapshot:
	default:
	}
}

func (runtime *ProcessRuntime) Stop(reason string) error {
	runtime.lifecycleMutex.Lock()
	defer runtime.lifecycleMutex.Unlock()

	runtime.mutex.RLock()
	session := runtime.session
	state := runtime.state
	runtime.mutex.RUnlock()
	finalState := CatalogStateDisabled
	method := "host.stop"
	if reason == "suspend" {
		finalState = CatalogStateSuspended
		method = "host.prepare-suspend"
	}
	if session == nil {
		runtime.updateState(finalState, "", runtime.capabilitiesSnapshot())
		return nil
	}
	if reason == "suspend" && state != CatalogStateSuspended {
		runtime.updateState(CatalogStateSuspending, "", runtime.capabilitiesSnapshot())
	}
	session.stopRequested.Store(true)

	var stopErr error
	if session.helloAccepted.Load() && (state == CatalogStateReady || state == CatalogStateUnsupported || state == CatalogStateSuspending) {
		ctx, cancel := context.WithTimeout(context.Background(), runtime.stopTimeout)
		_, stopErr = runtime.invokeSession(ctx, session, method, map[string]any{"reason": reason})
		cancel()
	}
	runtime.closeSessionInput(session)
	select {
	case <-session.done:
	case <-time.After(pluginExitGrace):
		session.cancel()
		if session.cmd.Process != nil {
			_ = session.cmd.Process.Kill()
		}
		select {
		case <-session.done:
		case <-time.After(pluginExitGrace):
			if stopErr == nil {
				stopErr = fmt.Errorf("plugin process did not exit after stop request")
			}
		}
	}
	session.cancel()
	runtime.clearSession(session)
	lastError := ""
	if stopErr != nil {
		lastError = stopErr.Error()
	}
	runtime.updateState(finalState, lastError, runtime.capabilitiesSnapshot())
	return stopErr
}

func (runtime *ProcessRuntime) Status() RuntimeStatus {
	runtime.mutex.RLock()
	defer runtime.mutex.RUnlock()
	return RuntimeStatus{
		ID:           runtime.spec.ID,
		State:        runtime.state,
		LastError:    runtime.lastError,
		Capabilities: append([]string(nil), runtime.capabilities...),
	}
}

func (runtime *ProcessRuntime) writeControl(ctx context.Context, session *processSession, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > maxProtocolMessageBytes {
		return fmt.Errorf("plugin message exceeds %d bytes", maxProtocolMessageBytes)
	}
	request := processWriteRequest{data: append(data, '\n'), done: make(chan error, 1)}
	select {
	case session.controlCh <- request:
	case <-ctx.Done():
		return ctx.Err()
	case <-session.done:
		return fmt.Errorf("plugin process exited")
	case <-session.ctx.Done():
		return session.ctx.Err()
	}
	select {
	case err := <-request.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-session.done:
		return fmt.Errorf("plugin process exited")
	case <-session.ctx.Done():
		return session.ctx.Err()
	}
}

func (runtime *ProcessRuntime) writerLoop(session *processSession) {
	for {
		var request processWriteRequest
		select {
		case request = <-session.controlCh:
		default:
			select {
			case request = <-session.controlCh:
			case snapshot := <-session.telemetryCh:
				message := protocolTelemetry{
					Type:      "telemetry",
					Sequence:  snapshot.Sequence,
					SampledAt: snapshot.SampledAt,
					Payload:   snapshot.Payload,
				}
				data, err := json.Marshal(message)
				if err != nil {
					runtime.logError("plugin telemetry encode failed: %s: %v", runtime.spec.ID, err)
					continue
				}
				request.data = append(data, '\n')
			case <-session.ctx.Done():
				return
			}
		}
		err := writeAll(session.stdin, request.data)
		if request.done != nil {
			request.done <- err
		}
		if err != nil {
			if !session.stopRequested.Load() {
				runtime.failSession(session, fmt.Errorf("write plugin stdin: %w", err))
			}
			return
		}
	}
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written <= 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

func (runtime *ProcessRuntime) readerLoop(session *processSession, stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 4096), maxProtocolMessageBytes)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var message protocolEnvelope
		if err := json.Unmarshal(line, &message); err != nil {
			runtime.failSession(session, fmt.Errorf("decode plugin stdout: %w", err))
			return
		}
		if message.Type == "hello" {
			if !session.helloSeen.CompareAndSwap(false, true) {
				runtime.failSession(session, fmt.Errorf("plugin sent duplicate hello"))
				return
			}
			session.helloCh <- processHelloResult{message: message}
			continue
		}
		if !session.helloAccepted.Load() {
			runtime.failSession(session, fmt.Errorf("plugin sent %q before handshake", message.Type))
			return
		}
		switch message.Type {
		case "response":
			if message.ID == "" {
				runtime.failSession(session, fmt.Errorf("plugin response is missing id"))
				return
			}
			session.pendingMutex.Lock()
			responseCh := session.pendingRequests[message.ID]
			session.pendingMutex.Unlock()
			if responseCh != nil {
				select {
				case responseCh <- message:
				default:
				}
			}
		case "event":
			if !validProtocolName(message.Event) {
				runtime.failSession(session, fmt.Errorf("invalid plugin event %q", message.Event))
				return
			}
			payload := message.Payload
			if len(payload) == 0 {
				payload = json.RawMessage(`{}`)
			}
			runtime.emitEvent(RuntimeEvent{
				PluginID: runtime.spec.ID,
				Event:    message.Event,
				Payload:  append(json.RawMessage(nil), payload...),
			})
		default:
			runtime.failSession(session, fmt.Errorf("unknown plugin message type %q", message.Type))
			return
		}
	}
	if err := scanner.Err(); err != nil && !session.stopRequested.Load() {
		runtime.failSession(session, fmt.Errorf("read plugin stdout: %w", err))
	}
}

func (runtime *ProcessRuntime) stderrLoop(session *processSession, stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 4096), 64*1024)
	for scanner.Scan() {
		runtime.logError("plugin[%s] stderr: %s", runtime.spec.ID, scanner.Text())
	}
	if err := scanner.Err(); err != nil && !session.stopRequested.Load() {
		runtime.logError("plugin[%s] stderr read failed: %v", runtime.spec.ID, err)
	}
}

func (runtime *ProcessRuntime) waitLoop(session *processSession) {
	err := session.cmd.Wait()
	close(session.done)
	runtime.failPending(session, fmt.Errorf("plugin process exited"))

	runtime.mutex.Lock()
	current := runtime.session == session
	if current {
		runtime.session = nil
	}
	unexpected := current && !session.stopRequested.Load()
	if unexpected {
		runtime.state = CatalogStateFailed
		if err != nil {
			runtime.lastError = err.Error()
		} else {
			runtime.lastError = "plugin process exited unexpectedly"
		}
		runtime.capabilities = nil
	}
	status := runtime.statusLocked()
	runtime.mutex.Unlock()
	if unexpected {
		runtime.emitStatus(status)
	}
}

func (runtime *ProcessRuntime) failSession(session *processSession, err error) {
	if err == nil || !session.stopRequested.CompareAndSwap(false, true) {
		return
	}
	if !session.helloAccepted.Load() {
		select {
		case session.helloCh <- processHelloResult{err: err}:
		default:
		}
	}
	runtime.failPending(session, err)
	runtime.updateState(CatalogStateFailed, err.Error(), nil)
	runtime.closeSessionInput(session)
	session.cancel()
	if session.cmd.Process != nil {
		_ = session.cmd.Process.Kill()
	}
}

func (runtime *ProcessRuntime) failPending(session *processSession, err error) {
	message := protocolEnvelope{Type: "response", OK: false, Error: err.Error()}
	session.pendingMutex.Lock()
	channels := make([]chan protocolEnvelope, 0, len(session.pendingRequests))
	for _, responseCh := range session.pendingRequests {
		channels = append(channels, responseCh)
	}
	session.pendingMutex.Unlock()
	for _, responseCh := range channels {
		select {
		case responseCh <- message:
		default:
		}
	}
}

func (runtime *ProcessRuntime) closeSessionInput(session *processSession) {
	session.stdinCloseOnce.Do(func() {
		_ = session.stdin.Close()
	})
}

func (runtime *ProcessRuntime) clearSession(session *processSession) {
	runtime.mutex.Lock()
	if runtime.session == session {
		runtime.session = nil
	}
	runtime.mutex.Unlock()
}

func (runtime *ProcessRuntime) updateState(state CatalogState, lastError string, capabilities []string) {
	runtime.mutex.Lock()
	runtime.state = state
	runtime.lastError = strings.TrimSpace(lastError)
	if len(runtime.lastError) > 2048 {
		runtime.lastError = runtime.lastError[:2048]
	}
	runtime.capabilities = append([]string(nil), capabilities...)
	status := runtime.statusLocked()
	runtime.mutex.Unlock()
	runtime.emitStatus(status)
}

func (runtime *ProcessRuntime) statusLocked() RuntimeStatus {
	return RuntimeStatus{
		ID:           runtime.spec.ID,
		State:        runtime.state,
		LastError:    runtime.lastError,
		Capabilities: append([]string(nil), runtime.capabilities...),
	}
}

func (runtime *ProcessRuntime) capabilitiesSnapshot() []string {
	runtime.mutex.RLock()
	defer runtime.mutex.RUnlock()
	return append([]string(nil), runtime.capabilities...)
}

func (runtime *ProcessRuntime) emitStatus(status RuntimeStatus) {
	if runtime.onStatus == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			runtime.logError("plugin status callback panicked: %s: %v", runtime.spec.ID, recovered)
		}
	}()
	runtime.onStatus(status)
}

func (runtime *ProcessRuntime) emitEvent(event RuntimeEvent) {
	if runtime.onEvent == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			runtime.logError("plugin event callback panicked: %s: %v", runtime.spec.ID, recovered)
		}
	}()
	runtime.onEvent(event)
}

func (runtime *ProcessRuntime) logError(format string, values ...any) {
	if runtime.logger != nil {
		runtime.logger.Error(format, values...)
	}
}

func cloneRuntimeSpec(spec RuntimeSpec) RuntimeSpec {
	spec.Capabilities = append([]string(nil), spec.Capabilities...)
	spec.TelemetryInputs = append([]string(nil), spec.TelemetryInputs...)
	return spec
}

func runtimeSpecKey(spec RuntimeSpec) string {
	return strings.Join([]string{
		spec.ID,
		spec.Version,
		filepath.Clean(spec.PluginDir),
		filepath.Clean(spec.BackendPath),
		strings.Join(spec.Capabilities, ","),
		strings.Join(spec.TelemetryInputs, ","),
	}, "\x00")
}
