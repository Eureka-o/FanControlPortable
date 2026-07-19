// Package ipc 提供核心服务与 GUI 之间的进程间通信
package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
)

// currentProtocolVersion 是当前 IPC 协议版本,统一引用 appmeta 以避免版本号漂移。
const currentProtocolVersion = appmeta.ProtocolVersion

var messageCounter uint64

const (
	// PipeName 命名管道名称
	PipeName = appmeta.IPCPipeName
	// PipePath 命名管道完整路径
	PipePath = `\\.\pipe\` + PipeName
)

// RequestType 请求类型
type RequestType string

const (
	// 设备相关
	ReqConnect                RequestType = "Connect"
	ReqAutoScanDevices        RequestType = "AutoScanDevices"
	ReqScanDeviceCandidates   RequestType = "ScanDeviceCandidates"
	ReqConnectDeviceCandidate RequestType = "ConnectDeviceCandidate"
	ReqConnectNativeDevice    RequestType = "ConnectNativeDevice"
	ReqScanWiFiDevices        RequestType = "ScanWiFiDevices"
	ReqControlWiFiScan        RequestType = "ControlWiFiScan"
	ReqDisconnect             RequestType = "Disconnect"
	ReqGetDeviceStatus        RequestType = "GetDeviceStatus"
	ReqGetCurrentFanData      RequestType = "GetCurrentFanData"
	ReqRefreshDeviceSettings  RequestType = "RefreshDeviceSettings"

	// 配置相关
	ReqGetConfig                  RequestType = "GetConfig"
	ReqUpdateConfig               RequestType = "UpdateConfig"
	ReqSetFanCurve                RequestType = "SetFanCurve"
	ReqGetFanCurve                RequestType = "GetFanCurve"
	ReqGetDeviceProfiles          RequestType = "GetDeviceProfiles"
	ReqGetSupportedDeviceProfiles RequestType = "GetSupportedDeviceProfiles"
	ReqGetUserDeviceProfiles      RequestType = "GetUserDeviceProfiles"
	ReqSetActiveDeviceProfile     RequestType = "SetActiveDeviceProfile"
	ReqSaveDeviceProfile          RequestType = "SaveDeviceProfile"
	ReqDeleteDeviceProfile        RequestType = "DeleteDeviceProfile"
	ReqExportDeviceProfiles       RequestType = "ExportDeviceProfiles"
	ReqImportDeviceProfiles       RequestType = "ImportDeviceProfiles"
	ReqTestDeviceProfile          RequestType = "TestDeviceProfile"
	ReqGetFanCurveProfiles        RequestType = "GetFanCurveProfiles"
	ReqSetActiveFanCurveProfile   RequestType = "SetActiveFanCurveProfile"
	ReqSaveFanCurveProfile        RequestType = "SaveFanCurveProfile"
	ReqDeleteFanCurveProfile      RequestType = "DeleteFanCurveProfile"
	ReqExportFanCurveProfiles     RequestType = "ExportFanCurveProfiles"
	ReqImportFanCurveProfiles     RequestType = "ImportFanCurveProfiles"
	ReqResetLearnedOffsets        RequestType = "ResetLearnedOffsets"

	// 控制相关
	ReqSetAutoControl                    RequestType = "SetAutoControl"
	ReqSetManualGear                     RequestType = "SetManualGear"
	ReqGetAvailableGears                 RequestType = "GetAvailableGears"
	ReqSetCustomSpeed                    RequestType = "SetCustomSpeed"
	ReqSetGearLight                      RequestType = "SetGearLight"
	ReqSetPowerOnStart                   RequestType = "SetPowerOnStart"
	ReqSetSmartStartStop                 RequestType = "SetSmartStartStop"
	ReqSetWiFiSmartStartStopStandbySpeed RequestType = "SetWiFiSmartStartStopStandbySpeed"
	ReqSetBrightness                     RequestType = "SetBrightness"
	ReqSetLightStrip                     RequestType = "SetLightStrip"
	ReqBeginNoiseDiagnostic              RequestType = "BeginNoiseDiagnostic"
	ReqSetNoiseDiagnosticTarget          RequestType = "SetNoiseDiagnosticTarget"
	ReqEndNoiseDiagnostic                RequestType = "EndNoiseDiagnostic"
	ReqCancelNoiseDiagnostic             RequestType = "CancelNoiseDiagnostic"
	ReqSaveNoiseDiagnosticResult         RequestType = "SaveNoiseDiagnosticResult"
	ReqSaveAxisNoiseProfile              RequestType = "SaveAxisNoiseProfile"

	// 温度相关
	ReqGetTemperature               RequestType = "GetTemperature"
	ReqGetTemperatureHistory        RequestType = "GetTemperatureHistory"
	ReqSetTemperatureHistoryEnabled RequestType = "SetTemperatureHistoryEnabled"
	ReqTestTemperatureReading       RequestType = "TestTemperatureReading"
	ReqTestBridgeProgram            RequestType = "TestBridgeProgram"
	ReqGetBridgeProgramStatus       RequestType = "GetBridgeProgramStatus"
	ReqRestartPawnIO                RequestType = "RestartPawnIO"
	ReqReinstallPawnIO              RequestType = "ReinstallPawnIO"

	// 自启动相关
	ReqSetWindowsAutoStart    RequestType = "SetWindowsAutoStart"
	ReqCheckWindowsAutoStart  RequestType = "CheckWindowsAutoStart"
	ReqIsRunningAsAdmin       RequestType = "IsRunningAsAdmin"
	ReqGetAutoStartMethod     RequestType = "GetAutoStartMethod"
	ReqSetAutoStartWithMethod RequestType = "SetAutoStartWithMethod"

	// 窗口相关
	ReqShowWindow RequestType = "ShowWindow"
	ReqHideWindow RequestType = "HideWindow"
	ReqQuitApp    RequestType = "QuitApp"

	// 调试相关
	ReqGetDebugInfo           RequestType = "GetDebugInfo"
	ReqExportDiagnostics      RequestType = "ExportDiagnostics"
	ReqSetDebugMode           RequestType = "SetDebugMode"
	ReqSendDeviceDebugCommand RequestType = "SendDeviceDebugCommand"
	ReqGetDeviceDebugFrames   RequestType = "GetDeviceDebugFrames"
	ReqUpdateGuiResponseTime  RequestType = "UpdateGuiResponseTime"

	// 系统相关
	ReqPing              RequestType = "Ping"
	ReqIsAutoStartLaunch RequestType = "IsAutoStartLaunch"
	ReqSubscribeEvents   RequestType = "SubscribeEvents"
	ReqUnsubscribeEvents RequestType = "UnsubscribeEvents"
)

// Request IPC 请求
type Request struct {
	ProtocolVersion string          `json:"protocolVersion,omitempty"`
	RequestID       string          `json:"requestId,omitempty"`
	Timestamp       int64           `json:"timestamp,omitempty"`
	Type            RequestType     `json:"type"`
	Data            json.RawMessage `json:"data,omitempty"`
}

// Response IPC 响应
type Response struct {
	ProtocolVersion string          `json:"protocolVersion,omitempty"`
	RequestID       string          `json:"requestId,omitempty"`
	Timestamp       int64           `json:"timestamp,omitempty"`
	IsResponse      bool            `json:"isResponse"` // 标识这是响应而非事件
	Success         bool            `json:"success"`
	ErrorCode       string          `json:"errorCode,omitempty"`
	Error           string          `json:"error,omitempty"`
	Data            json.RawMessage `json:"data,omitempty"`
}

// Event IPC 事件（服务器推送给客户端）
type Event struct {
	SchemaVersion string          `json:"schemaVersion,omitempty"`
	EventID       string          `json:"eventId,omitempty"`
	Timestamp     int64           `json:"timestamp,omitempty"`
	Source        string          `json:"source,omitempty"`
	IsEvent       bool            `json:"isEvent"` // 标识这是事件
	Type          string          `json:"type"`
	Data          json.RawMessage `json:"data,omitempty"`
}

// EventType 事件类型
const (
	EventFanDataUpdate            = "fan-data-update"
	EventTemperatureUpdate        = "temperature-update"
	EventTemperatureHistoryUpdate = "temperature-history-update"
	EventDeviceConnected          = "device-connected"
	EventDeviceDisconnected       = "device-disconnected"
	EventDeviceError              = "device-error"
	EventDeviceSettingsUpdate     = "device-settings-update"
	EventConfigUpdate             = "config-update"
	EventSystemResume             = "system-resume"
	EventHotkeyTriggered          = "hotkey-triggered"
	EventLegionPowerModeUpdate    = "legion-power-mode-update"
	EventLegionFnQSupportUpdate   = "legion-fnq-support-update"
	EventHealthPing               = "health-ping"
	EventHeartbeat                = "heartbeat"
)

// Server IPC 服务器
type Server struct {
	listener      net.Listener
	clients       map[net.Conn]*clientState
	mutex         sync.RWMutex
	handler       RequestHandler
	logger        types.Logger
	running       atomic.Bool
	throttleMutex sync.Mutex
	lastEventEmit map[string]time.Time
}

type clientState struct {
	conn      net.Conn
	writeCh   chan []byte
	closeOnce sync.Once
	closed    chan struct{}
}

const clientWriteQueueSize = 64

// RequestHandler 请求处理函数类型
type RequestHandler func(req Request) Response

func newMessageID(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), atomic.AddUint64(&messageCounter, 1))
}

// NewServer 创建 IPC 服务器
func NewServer(handler RequestHandler, logger types.Logger) *Server {
	return &Server{
		clients:       make(map[net.Conn]*clientState),
		handler:       handler,
		logger:        logger,
		lastEventEmit: make(map[string]time.Time),
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	// 创建命名管道监听器
	cfg := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;WD)", // 允许所有用户访问
	}

	listener, err := winio.ListenPipe(PipePath, cfg)
	if err != nil {
		return fmt.Errorf("创建命名管道失败: %v", err)
	}

	s.listener = listener
	s.running.Store(true)
	s.logInfo("IPC 服务器已启动: %s", PipePath)

	// 接受连接
	go s.acceptConnections()

	return nil
}

// acceptConnections 接受客户端连接
func (s *Server) acceptConnections() {
	consecutiveFailures := 0
	for s.running.Load() {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.running.Load() {
				return
			}
			// 监听器持续故障时退避重试，避免热循环空转占满 CPU 并刷爆日志。
			consecutiveFailures++
			s.logError("接受连接失败（连续第 %d 次）: %v", consecutiveFailures, err)
			backoff := time.Duration(consecutiveFailures*100) * time.Millisecond
			if backoff > 3*time.Second {
				backoff = 3 * time.Second
			}
			time.Sleep(backoff)
			continue
		}
		consecutiveFailures = 0

		state := &clientState{
			conn:    conn,
			writeCh: make(chan []byte, clientWriteQueueSize),
			closed:  make(chan struct{}),
		}

		s.mutex.Lock()
		s.clients[conn] = state
		s.mutex.Unlock()

		s.logInfo("新的 IPC 客户端已连接")

		go s.clientWriter(state)
		go s.handleClient(conn, state)
	}
}

func (s *Server) clientWriter(state *clientState) {
	for {
		select {
		case data, ok := <-state.writeCh:
			if !ok {
				return
			}
			if _, err := state.conn.Write(data); err != nil {
				s.logDebug("发送数据失败: %v", err)
				s.closeClient(state)
				return
			}
		case <-state.closed:
			return
		}
	}
}

func (s *Server) closeClient(state *clientState) {
	state.closeOnce.Do(func() {
		close(state.closed)
		s.mutex.Lock()
		delete(s.clients, state.conn)
		s.mutex.Unlock()
		state.conn.Close()
	})
}

func (s *Server) handleRequest(state *clientState, req Request) {
	resp := s.handler(req)
	if resp.ProtocolVersion == "" {
		resp.ProtocolVersion = currentProtocolVersion
	}
	if resp.RequestID == "" {
		resp.RequestID = req.RequestID
	}
	if resp.Timestamp == 0 {
		resp.Timestamp = time.Now().UnixMilli()
	}
	resp.IsResponse = true

	respBytes, err := json.Marshal(resp)
	if err != nil {
		s.logError("序列化响应失败: %v", err)
		return
	}
	select {
	case state.writeCh <- append(respBytes, '\n'):
	case <-state.closed:
		s.logDebug("IPC 客户端已断开，响应已丢弃: request=%s", req.RequestID)
	}
}

// handleClient 处理客户端连接
func (s *Server) handleClient(conn net.Conn, state *clientState) {
	defer func() {
		s.closeClient(state)
		s.logInfo("IPC 客户端已断开")
	}()

	reader := bufio.NewReader(conn)

	for s.running.Load() {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			s.logDebug("读取客户端请求失败: %v", err)
			return
		}

		// 解析请求
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.logError("解析请求失败: %v", err)
			continue
		}
		if req.ProtocolVersion == "" {
			req.ProtocolVersion = currentProtocolVersion
		}
		if req.RequestID == "" {
			req.RequestID = newMessageID("req")
		}
		if req.Timestamp == 0 {
			req.Timestamp = time.Now().UnixMilli()
		}
		s.logDebug("IPC 请求[%s]: %s", req.RequestID, req.Type)
		// Target settling is long-running; keep this reader free so cancellation
		// can close the diagnostic lease on the same connection.
		if req.Type == ReqSetNoiseDiagnosticTarget {
			go s.handleRequest(state, req)
			continue
		}
		s.handleRequest(state, req)
	}
}

var highFrequencyEventTypes = map[string]time.Duration{
	EventFanDataUpdate:            250 * time.Millisecond,
	EventTemperatureUpdate:        250 * time.Millisecond,
	EventTemperatureHistoryUpdate: 1000 * time.Millisecond,
}

func (s *Server) shouldDropEvent(eventType string) bool {
	threshold, ok := highFrequencyEventTypes[eventType]
	if !ok {
		return false
	}
	now := time.Now()
	s.throttleMutex.Lock()
	defer s.throttleMutex.Unlock()
	last, exists := s.lastEventEmit[eventType]
	if exists && now.Sub(last) < threshold {
		return true
	}
	s.lastEventEmit[eventType] = now
	return false
}

// BroadcastEvent 广播事件给所有客户端
func (s *Server) BroadcastEvent(eventType string, data any) {
	if !s.HasClients() {
		return
	}

	if s.shouldDropEvent(eventType) {
		return
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		s.logError("序列化事件数据失败: %v", err)
		return
	}

	event := Event{
		SchemaVersion: currentProtocolVersion,
		EventID:       newMessageID("evt"),
		Timestamp:     time.Now().UnixMilli(),
		Source:        "core",
		IsEvent:       true,
		Type:          eventType,
		Data:          dataBytes,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		s.logError("序列化事件失败: %v", err)
		return
	}
	payload := append(eventBytes, '\n')

	s.mutex.RLock()
	for _, state := range s.clients {
		select {
		case state.writeCh <- payload:
		default:
			s.logDebug("客户端写队列已满，丢弃事件: %s", eventType)
		}
	}
	s.mutex.RUnlock()
}

// Stop 停止服务器
func (s *Server) Stop() {
	s.running.Store(false)
	if s.listener != nil {
		s.listener.Close()
	}

	s.mutex.Lock()
	clients := make([]*clientState, 0, len(s.clients))
	for _, state := range s.clients {
		clients = append(clients, state)
	}
	s.clients = make(map[net.Conn]*clientState)
	s.mutex.Unlock()
	for _, state := range clients {
		s.closeClient(state)
	}

	s.logInfo("IPC 服务器已停止")
}

// HasClients 检查是否有客户端连接
func (s *Server) HasClients() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.clients) > 0
}

// 日志辅助方法
func (s *Server) logInfo(format string, v ...any) {
	if s.logger != nil {
		s.logger.Info(format, v...)
	}
}

func (s *Server) logError(format string, v ...any) {
	if s.logger != nil {
		s.logger.Error(format, v...)
	}
}

func (s *Server) logDebug(format string, v ...any) {
	if s.logger != nil {
		s.logger.Debug(format, v...)
	}
}

// Client IPC 客户端
//
// 响应路由：每条 SendRequest 注册一个 (requestID -> chan *Response)，readLoop 收到响应时
// 按 requestID 派发到对应 channel。这样并发请求互不串扰，且超时未取消的旧响应被自动丢弃。
func isClosedConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "closed") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "use of closed network connection")
}

type Client struct {
	conn               net.Conn
	mutex              sync.Mutex
	eventDispatchMutex sync.Mutex
	logger             types.Logger
	eventHandler       func(Event)

	pendingMutex sync.Mutex
	pending      map[string]pendingRequest

	connected  bool
	generation uint64
	connMutex  sync.RWMutex
}

type requestResult struct {
	response *Response
	err      error
}

type pendingRequest struct {
	generation uint64
	result     chan requestResult
}

// NewClient 创建 IPC 客户端
func NewClient(logger types.Logger) *Client {
	return &Client{
		logger:  logger,
		pending: make(map[string]pendingRequest),
	}
}

// Connect 连接到服务器
func (c *Client) Connect() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if c.connected {
		return nil
	}

	timeout := 5 * time.Second
	var conn net.Conn
	var err error
	for _, pipeName := range appmeta.IPCPipeCandidates() {
		pipePath := `\\.\pipe\` + pipeName
		conn, err = winio.DialPipe(pipePath, &timeout)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("连接 IPC 服务器失败: %v", err)
	}

	c.conn = conn
	c.connected = true
	c.generation++
	generation := c.generation
	c.logInfo("已连接到 IPC 服务器")

	// 启动消息接收循环
	go c.readLoop(conn, bufio.NewReader(conn), generation)

	return nil
}

// readLoop 统一的消息读取循环
func (c *Client) readLoop(conn net.Conn, reader *bufio.Reader, generation uint64) {
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			c.logDebug("读取消息失败: %v", err)
			c.disconnectCurrent(conn, generation, fmt.Errorf("IPC 连接已断开: %w", err))
			return
		}

		// 使用通用结构来检测消息类型
		var msg struct {
			IsResponse bool `json:"isResponse"`
			IsEvent    bool `json:"isEvent"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			c.logDebug("解析消息类型失败: %v", err)
			continue
		}

		if msg.IsResponse {
			var resp Response
			if err := json.Unmarshal(line, &resp); err == nil {
				// 按 RequestID 路由到对应等待者；找不到则说明请求已超时取消，直接丢弃
				c.pendingMutex.Lock()
				pending, ok := c.pending[resp.RequestID]
				if ok && pending.generation == generation {
					delete(c.pending, resp.RequestID)
				} else {
					ok = false
				}
				c.pendingMutex.Unlock()
				if ok {
					// channel 容量 1 + delete 后立即送达，不会阻塞
					pending.result <- requestResult{response: &resp}
				} else {
					c.logDebug("收到无主响应，丢弃: requestID=%s", resp.RequestID)
				}
			}
		} else if msg.IsEvent {
			var event Event
			if err := json.Unmarshal(line, &event); err == nil && event.Type != "" {
				c.eventDispatchMutex.Lock()
				c.connMutex.RLock()
				current := c.connected && c.generation == generation && c.conn == conn
				handler := c.eventHandler
				c.connMutex.RUnlock()
				if !current {
					c.eventDispatchMutex.Unlock()
					return
				}
				if handler != nil {
					handler(event)
				}
				c.eventDispatchMutex.Unlock()
			}
		}
	}
}

func (c *Client) disconnectCurrent(conn net.Conn, generation uint64, err error) {
	c.connMutex.Lock()
	if c.generation != generation || c.conn != conn {
		c.connMutex.Unlock()
		c.failPending(generation, err)
		return
	}
	c.connected = false
	c.conn = nil
	c.generation++
	c.connMutex.Unlock()

	_ = conn.Close()
	c.failPending(generation, err)
}

func (c *Client) failPending(generation uint64, err error) {
	c.pendingMutex.Lock()
	results := make([]chan requestResult, 0, len(c.pending))
	for requestID, pending := range c.pending {
		if pending.generation != generation {
			continue
		}
		delete(c.pending, requestID)
		results = append(results, pending.result)
	}
	c.pendingMutex.Unlock()
	for _, ch := range results {
		ch <- requestResult{err: err}
	}
}

// SetEventHandler 设置事件处理函数
func (c *Client) SetEventHandler(handler func(Event)) {
	c.connMutex.Lock()
	c.eventHandler = handler
	c.connMutex.Unlock()
}

// SendRequest 发送请求并等待响应
func (c *Client) SendRequest(reqType RequestType, data any) (*Response, error) {
	return c.SendRequestWithTimeout(reqType, data, 10*time.Second)
}

func (c *Client) SendRequestWithTimeout(reqType RequestType, data any, timeout time.Duration) (*Response, error) {
	response, _, err := c.SendRequestWithTimeoutGeneration(reqType, data, timeout)
	return response, err
}

// SendRequestWithTimeoutGeneration 同时返回请求实际使用的连接代次。
func (c *Client) SendRequestWithTimeoutGeneration(reqType RequestType, data any, timeout time.Duration) (*Response, uint64, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	c.connMutex.RLock()
	generation := c.generation
	if !c.connected || c.conn == nil {
		c.connMutex.RUnlock()
		return nil, generation, fmt.Errorf("未连接到服务器")
	}
	conn := c.conn
	c.connMutex.RUnlock()

	var dataBytes json.RawMessage
	if data != nil {
		var err error
		dataBytes, err = json.Marshal(data)
		if err != nil {
			return nil, generation, fmt.Errorf("序列化请求数据失败: %v", err)
		}
	}

	requestID := newMessageID("req")
	req := Request{
		ProtocolVersion: currentProtocolVersion,
		RequestID:       requestID,
		Timestamp:       time.Now().UnixMilli(),
		Type:            reqType,
		Data:            dataBytes,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, generation, fmt.Errorf("序列化请求失败: %v", err)
	}

	respCh := make(chan requestResult, 1)
	c.pendingMutex.Lock()
	c.pending[requestID] = pendingRequest{generation: generation, result: respCh}
	c.pendingMutex.Unlock()

	c.mutex.Lock()
	_, err = conn.Write(append(reqBytes, '\n'))
	c.mutex.Unlock()
	if err != nil {
		c.pendingMutex.Lock()
		delete(c.pending, requestID)
		c.pendingMutex.Unlock()
		c.disconnectCurrent(conn, generation, fmt.Errorf("发送请求失败: %w", err))
		return nil, generation, fmt.Errorf("发送请求失败: %v", err)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-respCh:
		return result.response, generation, result.err
	case <-timer.C:
		c.pendingMutex.Lock()
		delete(c.pending, requestID)
		c.pendingMutex.Unlock()
		return nil, generation, fmt.Errorf("等待响应超时")
	}
}

// ConnectionGeneration 返回当前连接代次的线程安全快照。
func (c *Client) ConnectionGeneration() uint64 {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	return c.generation
}

// CloseGeneration 仅在 generation 仍是当前代次时关闭连接。
func (c *Client) CloseGeneration(generation uint64) bool {
	c.connMutex.Lock()
	if c.generation != generation {
		c.connMutex.Unlock()
		c.failPending(generation, errors.New("IPC 连接已关闭"))
		return false
	}
	conn := c.conn
	c.connected = false
	c.conn = nil
	c.generation++
	c.connMutex.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	c.failPending(generation, errors.New("IPC 连接已关闭"))
	return true
}

// Close 关闭调用时观察到的连接，不影响随后建立的新代连接。
func (c *Client) Close() {
	c.CloseGeneration(c.ConnectionGeneration())
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	return c.connected
}

// 日志辅助方法
func (c *Client) logInfo(format string, v ...any) {
	if c.logger != nil {
		c.logger.Info(format, v...)
	}
}

func (c *Client) logDebug(format string, v ...any) {
	if c.logger != nil {
		c.logger.Debug(format, v...)
	}
}

// CheckCoreServiceRunning 检查核心服务是否正在运行
func CheckCoreServiceRunning() bool {
	timeout := 1 * time.Second
	for _, pipeName := range appmeta.IPCPipeCandidates() {
		pipePath := `\\.\pipe\` + pipeName
		conn, err := winio.DialPipe(pipePath, &timeout)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// GetCoreLockFilePath 获取核心服务锁文件路径
func GetCoreLockFilePath() string {
	tempDir := os.TempDir()
	return fmt.Sprintf("%s/fancontrol core.lock", tempDir)
}

// StartCoreRequestParams 启动核心服务的请求参数
type StartCoreRequestParams struct {
	ShowGUI bool `json:"showGUI"`
}

// SetAutoControlParams 设置智能变频参数
type SetAutoControlParams struct {
	Enabled bool `json:"enabled"`
}

// SetManualGearParams 设置手动挡位参数
type SetManualGearParams struct {
	Gear  string `json:"gear"`
	Level string `json:"level"`
}

// SetCustomSpeedParams 设置自定义转速参数
type SetCustomSpeedParams struct {
	Enabled bool `json:"enabled"`
	RPM     int  `json:"rpm"`
}

// SetBoolParams 布尔参数
type SetBoolParams struct {
	Enabled bool `json:"enabled"`
}

// SetStringParams 字符串参数
type SetStringParams struct {
	Value string `json:"value"`
}

// SetIntParams 整数参数
type SetIntParams struct {
	Value int `json:"value"`
}

// DeviceDebugCommandParams contains a raw protocol command for the debug panel.
type DeviceDebugCommandParams struct {
	Hex    string `json:"hex"`
	WaitMs int    `json:"waitMs"`
}

// SetAutoStartWithMethodParams 设置自启动方式参数
type SetAutoStartWithMethodParams struct {
	Enable bool   `json:"enable"`
	Method string `json:"method"`
}

// SetLightStripParams 设置灯带参数
type SetLightStripParams struct {
	Config types.LightStripConfig `json:"config"`
}

type BeginNoiseDiagnosticParams struct {
	Request types.NoiseDiagnosticBeginRequest `json:"request"`
}

type SetNoiseDiagnosticTargetParams struct {
	SessionID string `json:"sessionId"`
	Value     int    `json:"value"`
}

type NoiseDiagnosticSessionParams struct {
	SessionID string `json:"sessionId"`
}

type SaveNoiseDiagnosticResultParams struct {
	Result types.NoiseDiagnosticResult `json:"result"`
}

type SaveAxisNoiseProfileParams struct {
	Profile types.AxisNoiseProfile `json:"profile"`
}

// SetActiveFanCurveProfileParams 设置激活曲线方案参数
type SetActiveFanCurveProfileParams struct {
	ID string `json:"id"`
}

type SetActiveDeviceProfileParams struct {
	ID string `json:"id"`
}

type SaveDeviceProfileParams struct {
	Profile   types.DeviceProfile `json:"profile"`
	SetActive bool                `json:"setActive"`
}

type DeleteDeviceProfileParams struct {
	ID string `json:"id"`
}

type ImportDeviceProfilesParams struct {
	Code string `json:"code"`
}

type TestDeviceProfileParams struct {
	Profile    types.DeviceProfile `json:"profile"`
	Action     string              `json:"action"`
	SpeedValue float64             `json:"speedValue,omitempty"`
	TimeoutMs  int                 `json:"timeoutMs,omitempty"`
}

type ConnectNativeDeviceParams struct {
	ProfileID string `json:"profileId,omitempty"`
}

type ScanDeviceCandidatesParams struct {
	Mode string `json:"mode,omitempty"`
}

type ConnectDeviceCandidateParams struct {
	Candidate types.DeviceConnectRequest `json:"candidate"`
}

type ScanWiFiDevicesParams struct {
	Mode string `json:"mode"`
}

type ControlWiFiScanParams struct {
	Action string `json:"action"`
}

// SaveFanCurveProfileParams 保存曲线方案参数
type SaveFanCurveProfileParams struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Curve     []types.FanCurvePoint `json:"curve"`
	SetActive bool                  `json:"setActive"`
}

// DeleteFanCurveProfileParams 删除曲线方案参数
type DeleteFanCurveProfileParams struct {
	ID string `json:"id"`
}

// ImportFanCurveProfilesParams 导入曲线方案参数
type ImportFanCurveProfilesParams struct {
	Code string `json:"code"`
}
