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

	"github.com/Eureka-o/FanControlPortable/internal/appmeta"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"github.com/Microsoft/go-winio"
)

// currentProtocolVersion 是当前 IPC 协议版本,统一引用 appmeta 以避免版本号漂移。
const currentProtocolVersion = appmeta.ProtocolVersion

var messageCounter uint64

const (
	PipeName = appmeta.IPCPipeName
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
	ReqTransferDeviceImage    RequestType = "TransferDeviceImage"

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
	ReqGetTemperature                      RequestType = "GetTemperature"
	ReqGetTemperatureHistory               RequestType = "GetTemperatureHistory"
	ReqSetTemperatureHistoryEnabled        RequestType = "SetTemperatureHistoryEnabled"
	ReqSetTemperatureHistoryRetentionHours RequestType = "SetTemperatureHistoryRetentionHours"
	ReqTestTemperatureReading              RequestType = "TestTemperatureReading"
	ReqTestBridgeProgram                   RequestType = "TestBridgeProgram"
	ReqGetBridgeProgramStatus              RequestType = "GetBridgeProgramStatus"
	ReqRestartPawnIO                       RequestType = "RestartPawnIO"
	ReqReinstallPawnIO                     RequestType = "ReinstallPawnIO"

	// 自启动相关
	ReqSetWindowsAutoStart    RequestType = "SetWindowsAutoStart"
	ReqCheckWindowsAutoStart  RequestType = "CheckWindowsAutoStart"
	ReqIsRunningAsAdmin       RequestType = "IsRunningAsAdmin"
	ReqGetAutoStartMethod     RequestType = "GetAutoStartMethod"
	ReqSetAutoStartWithMethod RequestType = "SetAutoStartWithMethod"

	// 窗口相关
	ReqShowWindow  RequestType = "ShowWindow"
	ReqHideWindow  RequestType = "HideWindow"
	ReqQuitApp     RequestType = "QuitApp"
	ReqRestartCore RequestType = "RestartCore"

	// 调试相关
	ReqGetDebugInfo           RequestType = "GetDebugInfo"
	ReqExportDiagnostics      RequestType = "ExportDiagnostics"
	ReqSetDebugMode           RequestType = "SetDebugMode"
	ReqSendDeviceDebugCommand RequestType = "SendDeviceDebugCommand"
	ReqGetDeviceDebugFrames   RequestType = "GetDeviceDebugFrames"
	ReqUpdateGuiResponseTime  RequestType = "UpdateGuiResponseTime"

	// 黑鲨（BlackShark）BRB02 散热器
	ReqGetBlackSharkInfo             RequestType = "GetBlackSharkInfo"
	ReqSetBlackSharkLightingEnabled  RequestType = "SetBlackSharkLightingEnabled"
	ReqSetBlackSharkLcdScreenEnabled RequestType = "SetBlackSharkLcdScreenEnabled"
	ReqSetBlackSharkOnOffVector      RequestType = "SetBlackSharkOnOffVector"
	ReqGetBlackSharkRgbLighting      RequestType = "GetBlackSharkRgbLighting"
	// ReqGetBlackSharkRgbCache 只取"上次读到的那份"灯效状态，一次设备 IO 都不做。
	ReqGetBlackSharkRgbCache       RequestType = "GetBlackSharkRgbCache"
	ReqSelectBlackSharkRgbMode     RequestType = "SelectBlackSharkRgbMode"
	ReqSetBlackSharkRgbModeEffects RequestType = "SetBlackSharkRgbModeEffects"
	ReqSetBlackSharkRgbModeColor   RequestType = "SetBlackSharkRgbModeColor"
	// ReqSetBlackSharkRgbColorOption 写「颜色下拉选项」（payload[0] 高半字节，0..4），
	// 与 SetBlackSharkRgbModeColor（色相→RGB）是两件事。
	ReqSetBlackSharkRgbColorOption RequestType = "SetBlackSharkRgbColorOption"
	ReqGetBlackSharkLcdDisplay     RequestType = "GetBlackSharkLcdDisplay"
	ReqSetBlackSharkLcdDisplay     RequestType = "SetBlackSharkLcdDisplay"
	ReqGetBlackSharkConfigSnapshot RequestType = "GetBlackSharkConfigSnapshot"
	ReqRestoreBlackSharkConfig     RequestType = "RestoreBlackSharkConfig"
	ReqGetSceneRules               RequestType = "GetSceneRules"
	ReqSetSceneRules               RequestType = "SetSceneRules"
	ReqListSceneProcesses          RequestType = "ListSceneProcesses"
	// ReqResetBlackSharkRgb 是官方「灯效页 · 重置」：载荷全是录制下来的常量，不需要参数。
	ReqResetBlackSharkRgb RequestType = "ResetBlackSharkRgb"
	// ReqResetBlackSharkCooling 是「一键恢复四档出厂曲线」：主机侧把四个档位的方案曲线都恢复成出厂值，
	// 设备侧只保证各档是平曲线承载（不逐字节重放官方那 9 条 0x24）。
	ReqResetBlackSharkCooling RequestType = "ResetBlackSharkCooling"
	// ReqGetBlackSharkHostEffects 返回主机侧灯效驱动的状态（槽位 6「响应」/ 槽位 7「音频同步」）。
	// 纯内存读取，不发设备查询：只回答"现在到底在不在推、推了多少帧、失败原因是什么"。
	ReqGetBlackSharkHostEffects RequestType = "GetBlackSharkHostEffects"

	// ReqCheckBlackSharkFirmwareUpdate 主动检查一次固件版本（读设备 CMD 0x01 + 拉官方版本清单）。
	// 是本组里唯一做网络请求的命令（清单 6s 超时），也是唯一需要设备在线的版本查询，
	// 只在用户点「检查更新」时调用，绝不能进轮询。
	ReqCheckBlackSharkFirmwareUpdate RequestType = "CheckBlackSharkFirmwareUpdate"

	// ReqGetBlackSharkFirmwareStatus 只读已缓存的固件检查结果，一次设备/网络 IO 都不做。
	// 与上面成对：打开面板时先显示"上次查到的那份"，点「检查更新」才真去查。
	ReqGetBlackSharkFirmwareStatus RequestType = "GetBlackSharkFirmwareStatus"

	// ReqGetBlackSharkManualGearPresets 返回手动挡位面板要显示的派生转速预设（四档 × 三档，RPM）。
	//
	// 由 `deviceproto` 的标定表算出（唯一所有者），前端不再硬编码那 12 个数。
	ReqGetBlackSharkManualGearPresets RequestType = "GetBlackSharkManualGearPresets"

	// ReqGetBlackSharkCurveTempRange 返回黑鲨曲线 4 个点能被拖到的温度取值域（℃，含两端）。
	//
	// 取值域由 deviceproto 拥有（出厂曲线落在 20/40/60/80 只是默认值，不是格点限制）；
	// 前端据此限制横向拖动，避免在界面里再写一份区间。纯计算、不碰设备。
	ReqGetBlackSharkCurveTempRange RequestType = "GetBlackSharkCurveTempRange"

	// ReqReportClientIssue 让前端把界面上发生的异常 / 导航 / 卸载上报给核心，落进日志。
	ReqReportClientIssue RequestType = "ReportClientIssue"
	// 屏幕/屏保图像（见 docs/官方屏幕屏保逆向结论.md）
	ReqListScreenPresets  RequestType = "ListScreenPresets"
	ReqUploadScreenPreset RequestType = "UploadScreenPreset"
	ReqUploadScreenImage  RequestType = "UploadScreenImage"
	// ReqCancelScreenImageTransfer 取消进行中的屏幕图片传输（图传弹窗上的"取消"）。
	// 图传是一次设备独占任务，取消 = 让它的 ctx 结束，不必等十几秒。
	ReqCancelScreenImageTransfer RequestType = "CancelScreenImageTransfer"
	ReqGetScreenImageInfo RequestType = "GetScreenImageInfo"
	// ReqReadScreenImage 从设备读回当前屏上那张图（0xC7 + 反复 0xC8，约 2096 帧）。
	// 很慢（几十秒），只能由用户显式触发，不要挂进状态刷新。
	ReqReadScreenImage RequestType = "ReadScreenImage"
	// ReqReadScreenImageCache 只取"上次读到的那份"，一次设备 IO 都不做。
	ReqReadScreenImageCache RequestType = "ReadScreenImageCache"
	ReqScreenPresetThumb    RequestType = "ScreenPresetThumbnail"
	// ReqPreviewScreenCrop 只画图、不上传：手动裁剪编辑器每动一次滑杆就要重画一次。
	ReqPreviewScreenCrop RequestType = "PreviewScreenCrop"

	// 历史图片 = 本机屏幕图片缓存（`<安装目录>/screen-images/`，只记我们上传成功过屏的图）。
	// 三条都是纯本机文件操作；"列"与"删"一次设备 IO 都不做。
	// ReqListScreenHistory 列出缓存里的历史图片（排除"设备当前那张"，取最新 N 张，带缩略图）。
	ReqListScreenHistory RequestType = "ListScreenHistory"
	// ReqUploadScreenHistory 把缓存的画布原样直传上屏（*.bin 即 121552B RGB565 大端，免解码免裁剪）。
	ReqUploadScreenHistory RequestType = "UploadScreenHistory"
	// ReqDeleteScreenHistory 删除本机缓存里的一张历史图片（直接删文件，不进回收站）。
	ReqDeleteScreenHistory RequestType = "DeleteScreenHistory"

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
	// 主机侧灯效状态改由服务端推送，界面不再轮询。
	EventBlackSharkHostEffects  = "blackshark-host-effects"
	// EventScreenImageTransferProgress 屏幕图片传输进度（载荷 {"sent":n,"total":m}）。
	// 设备层每 16 帧回调一次，这里不再重复节流。
	EventScreenImageTransferProgress = "screen-image-transfer-progress"
	EventSystemResume           = "system-resume"
	EventHotkeyTriggered        = "hotkey-triggered"
	EventLegionPowerModeUpdate  = "legion-power-mode-update"
	EventLegionFnQSupportUpdate = "legion-fnq-support-update"
	EventHealthPing             = "health-ping"
	EventHeartbeat              = "heartbeat"
)

// Server IPC 服务器
type Server struct {
	listener      net.Listener
	clients       map[net.Conn]*clientState
	mutex         sync.RWMutex
	handler       RequestHandler
	logger        types.Logger
	running       atomic.Bool
	writeTimeout  time.Duration
	throttleMutex sync.Mutex
	lastEventEmit map[string]time.Time
}

type clientState struct {
	conn      net.Conn
	writeCh   chan []byte
	respCh    chan []byte
	sem       chan struct{}
	closeOnce sync.Once
	closed    chan struct{}
	// interactive marks a client that has issued a non-probe request. Short
	// named-pipe probes must not be mistaken for a live GUI client.
	interactive atomic.Bool
}

const (
	clientWriteQueueSize           = 64
	clientResponseQueueSize        = 64
	maxConcurrentRequestsPerClient = 16
	criticalEventEnqueueTimeout    = 500 * time.Millisecond
)

var ErrRequestTimeout = errors.New("等待 IPC 响应超时")

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
		writeTimeout:  10 * time.Second,
		lastEventEmit: make(map[string]time.Time),
	}
}

// Start 启动服务器
func (s *Server) Start() error {
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
			respCh:  make(chan []byte, clientResponseQueueSize),
			sem:     make(chan struct{}, maxConcurrentRequestsPerClient),
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
		case data := <-state.respCh:
			if !s.writeToClient(state, data) {
				return
			}
			continue
		case <-state.closed:
			return
		default:
		}

		select {
		case data := <-state.respCh:
			if !s.writeToClient(state, data) {
				return
			}
		case data, ok := <-state.writeCh:
			if !ok {
				return
			}
			if !s.writeToClient(state, data) {
				return
			}
		case <-state.closed:
			return
		}
	}
}

func (s *Server) writeToClient(state *clientState, data []byte) bool {
	if err := state.conn.SetWriteDeadline(time.Now().Add(s.writeTimeout)); err != nil {
		s.logDebug("设置 IPC 写超时失败: %v", err)
	}
	if _, err := state.conn.Write(data); err != nil {
		s.logDebug("发送数据失败: %v", err)
		s.closeClient(state)
		return false
	}
	return true
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
	resp := s.invokeHandler(req)
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
	case state.respCh <- append(respBytes, '\n'):
	case <-state.closed:
		s.logDebug("IPC 客户端已断开，响应已丢弃: request=%s", req.RequestID)
	}
}

func (s *Server) invokeHandler(req Request) (resp Response) {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.logError("处理 IPC 请求[%s] type=%s 时发生 panic: %v", req.RequestID, req.Type, recovered)
			resp = Response{
				Success:   false,
				ErrorCode: "internal_panic",
				Error:     fmt.Sprintf("核心服务内部错误: %v", recovered),
			}
		}
	}()
	return s.handler(req)
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
		if req.Type != ReqPing {
			state.interactive.Store(true)
		}
		s.logDebug("IPC 请求[%s]: %s", req.RequestID, req.Type)
		select {
		case state.sem <- struct{}{}:
		case <-state.closed:
			return
		}
		go func(req Request) {
			defer func() { <-state.sem }()
			s.handleRequest(state, req)
		}(req)
	}
}

var highFrequencyEventTypes = map[string]time.Duration{
	EventFanDataUpdate:            250 * time.Millisecond,
	EventTemperatureUpdate:        250 * time.Millisecond,
	EventTemperatureHistoryUpdate: 1000 * time.Millisecond,
	// 主机侧灯效状态：驱动循环是 204ms，但界面每秒看一次就够了 ⇒ 推之前先节流掉多余的。
	// `shouldDropEvent` 会自动按这张表丢弃高频事件，不用在驱动里自己数时间。
	EventBlackSharkHostEffects: 1000 * time.Millisecond,
}

func isHighFrequencyEvent(eventType string) bool {
	_, ok := highFrequencyEventTypes[eventType]
	return ok
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
	clients := make([]*clientState, 0, len(s.clients))
	for _, state := range s.clients {
		clients = append(clients, state)
	}
	s.mutex.RUnlock()

	for _, state := range clients {
		if isHighFrequencyEvent(eventType) {
			select {
			case state.writeCh <- payload:
			case <-state.closed:
			default:
				s.logDebug("客户端写队列已满，丢弃高频事件: %s", eventType)
			}
			continue
		}

		timer := time.NewTimer(criticalEventEnqueueTimeout)
		select {
		case state.writeCh <- payload:
			timer.Stop()
		case <-state.closed:
			timer.Stop()
		case <-timer.C:
			s.logDebug("客户端写队列持续拥塞，丢弃关键事件: %s", eventType)
		}
	}
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

// HasInteractiveClients reports whether a client has issued a real GUI/API
// request. Pipe probes and readiness Pings are deliberately excluded.
func (s *Server) HasInteractiveClients() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, state := range s.clients {
		if state != nil && state.interactive.Load() {
			return true
		}
	}
	return false
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
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		c.logDebug("设置 IPC 写超时失败: %v", err)
	}
	_, err = conn.Write(append(reqBytes, '\n'))
	c.mutex.Unlock()
	if err != nil {
		c.pendingMutex.Lock()
		delete(c.pending, requestID)
		c.pendingMutex.Unlock()
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			// 写超时说明这条管道已不可用，必须退休当前连接；否则后续请求会复用一个卡住的连接。
			timeoutErr := fmt.Errorf("%w: request=%s 发送超时", ErrRequestTimeout, reqType)
			c.disconnectCurrent(conn, generation, timeoutErr)
			return nil, generation, timeoutErr
		}
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
		return nil, generation, fmt.Errorf("%w: request=%s, timeout=%s", ErrRequestTimeout, reqType, timeout)
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

const coreServiceProbeTimeout = time.Second
const coreServiceReadyDialTimeout = 250 * time.Millisecond

// CheckCoreServiceRunning 检查核心服务命名管道是否可连接。
func CheckCoreServiceRunning() bool {
	for _, pipeName := range appmeta.IPCPipeCandidates() {
		timeout := coreServiceProbeTimeout
		pipePath := `\\.\pipe\` + pipeName
		conn, err := winio.DialPipe(pipePath, &timeout)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// CheckCoreServiceReady verifies that the core has started serving requests,
// rather than only having created the named pipe listener.
func CheckCoreServiceReady() bool {
	for _, pipeName := range appmeta.IPCPipeCandidates() {
		timeout := coreServiceReadyDialTimeout
		pipePath := `\\.\pipe\` + pipeName
		conn, err := winio.DialPipe(pipePath, &timeout)
		if err != nil {
			continue
		}

		ready := func() bool {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(750 * time.Millisecond))
			req := Request{
				ProtocolVersion: currentProtocolVersion,
				RequestID:       newMessageID("ready"),
				Timestamp:       time.Now().UnixMilli(),
				Type:            ReqPing,
			}
			payload, err := json.Marshal(req)
			if err != nil {
				return false
			}
			if _, err := conn.Write(append(payload, '\n')); err != nil {
				return false
			}
			var resp Response
			if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&resp); err != nil {
				return false
			}
			return resp.IsResponse && resp.Success && resp.RequestID == req.RequestID
		}()
		if ready {
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

// RestartCoreParams controls the core-only restart used by monitor-only mode.
type RestartCoreParams struct {
	MonitorOnlySession bool `json:"monitorOnlySession"`
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

// BlackSharkOnOffVectorParams 0x02 的两路开关向量参数。
type BlackSharkOnOffVectorParams struct {
	SmartStartStop   bool `json:"smartStartStop"`
	PowerOnSelfStart bool `json:"powerOnSelfStart"`
}

// ScreenPresetParams 上传一张内置精选的参数。
type ScreenPresetParams struct {
	OfficialPosition int `json:"officialPosition"`
}

// ScreenImagePathParams 上传一张本地图片的参数（绝对路径 + 可选裁剪框）。
type ScreenImagePathParams struct {
	Path    string `json:"path"`
	Zoom    int    `json:"zoom,omitempty"`
	OffsetX int    `json:"offsetX,omitempty"`
	OffsetY int    `json:"offsetY,omitempty"`
}

// ScreenCropPreview 手动裁剪的预览结果。
type ScreenCropPreview struct {
	DataURL   string `json:"dataUrl"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	RawWidth  int    `json:"rawWidth"`
	RawHeight int    `json:"rawHeight"`
	// SlackX / SlackY 是"该方向上还有多少可平移的余量"（百分比口径下：
	// 0 表示这个方向已无余量，offset 不会产生任何变化）。界面据此把滑杆置灰。
	SlackX int `json:"slackX"`
	SlackY int `json:"slackY"`
}

// ScreenPresetInfo 一张内置精选的描述（不含缩略图，缩略图另取，避免一次传 1MB+）。
type ScreenPresetInfo struct {
	OfficialPosition int    `json:"officialPosition"`
	AssetIndex       int    `json:"assetIndex"`
	Name             string `json:"name"`
}

// ScreenImageInfo 设备当前保存的屏保图像信息（0xC5 回读）。
type ScreenImageInfo struct {
	HasImage  bool   `json:"hasImage"`
	Timestamp uint32 `json:"timestamp"`
	Size      uint32 `json:"size"`
	CRC       uint16 `json:"crc"`
}

// ScreenHistoryItem 是本机屏幕图片缓存（`<安装目录>/screen-images/`）里的一张历史画布。
//
// 文件名是那次上屏的 Unix 秒，内容恒为 121552 字节（428×142 RGB565 大端）—— 就是上传时用的那份
// 字节，所以再传时能原样直发，免解码免裁剪。
type ScreenHistoryItem struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	UnixTime int64  `json:"unixTime"`
	Modified string `json:"modified"`
	Size     int64  `json:"size"`
	// Thumb 是 `data:image/png;base64,…`；只对"要显示的那几张"生成，避免一次传几百 KB。
	Thumb string `json:"thumb,omitempty"`
}

// ScreenHistoryList 是历史画布列表。
type ScreenHistoryList struct {
	// Dir 是扫描的目录（界面要如实告诉用户图片来自哪里）。
	Dir   string             `json:"dir"`
	Items []ScreenHistoryItem `json:"items"`
	// Current 是"设备当前正在用的那张"在本机缓存里的对应项（CRC 相同才给），带缩略图。
	// 界面用它填顶部预览 —— 免得为了看一张自己刚传过的图，还要等几十秒的"从设备读回"。
	// 对不上（别的程序换过图、或那张已被缓存上限清掉）就是 nil。
	Current *ScreenHistoryItem `json:"current,omitempty"`
	// Error 非空表示这一轮没读到（目录不存在 / 没权限），此时 Items 为空：
	// 这不叫失败，界面按"还没上传过屏保图片"显示即可。
	Error string `json:"error,omitempty"`
}

// ScreenHistoryQueryParams 列历史图的入参。
type ScreenHistoryQueryParams struct {
	// ExcludeCRC 是"设备当前那张图"的 CRC（取自 0xC5 回读）；命中的那条会被排除
	// —— 正在用的那张不该出现在"历史"里。0 表示设备上没有图，无需排除。
	ExcludeCRC uint16 `json:"excludeCrc"`
	// Limit 是最多返回几张；<=0 时按 2 张处理。
	Limit int `json:"limit"`
}

// ScreenHistoryPathParams 是单张历史图操作（上传 / 删除）的入参。
type ScreenHistoryPathParams struct {
	Path string `json:"path"`
}

// ScreenImageReadResult = 从设备读回当前屏图的结果。
//
// 与 ScreenImageInfo 分开：那个是 11 字节信息（秒级），这个是整张图（几十秒、约 2096 帧）。
type ScreenImageReadResult struct {
	// Ok=false 时看 Error。
	Ok bool `json:"ok"`
	// DataURL 是 `data:image/png;base64,…`，供界面直接显示。
	DataURL string `json:"dataUrl,omitempty"`
	// Bytes 读回的原始字节数（应为 121552）。
	Bytes int `json:"bytes"`
	// CRC 是本机对读回内容算的 CRC（不是设备声称的那个），可与 0xC5 信息帧对照。
	CRC   uint16 `json:"crc"`
	Error string `json:"error,omitempty"`

	// FromCache / CachedAtUnix 表示这是上次读回的那一份，不是刚刚读的。
	FromCache    bool  `json:"fromCache,omitempty"`
	CachedAtUnix int64 `json:"cachedAtUnix,omitempty"`
}

// ScreenUploadResult 一次上传的结果。
type ScreenUploadResult struct {
	Reports   int    `json:"reports"`
	Short     int    `json:"short"`
	Timestamp uint32 `json:"timestamp"`
	Size      uint32 `json:"size"`
	CRC       uint16 `json:"crc"`
	Verified  bool   `json:"verified"`
}

// SceneRulesParams 情景规则的读写参数。
//
// BaselineGear 是没有任何规则匹配时回落到的档位，0 表示不改变。
type SceneRulesParams struct {
	Rules        []types.SceneRule `json:"rules"`
	BaselineGear int               `json:"baselineGear"`
}

// BlackSharkRgbModeEffectParams 修改某个灯效模式的速度与亮度。
type BlackSharkRgbModeEffectParams struct {
	Index      int `json:"index"`
	Speed      int `json:"speed"`
	Brightness int `json:"brightness"`
}

// BlackSharkRgbColorParams 设置某个灯效模式的颜色。
type BlackSharkRgbColorParams struct {
	Index       int  `json:"index"`
	Hue         int  `json:"hue"`
	StaticColor bool `json:"staticColor"`
}

// BlackSharkRgbColorOptionParams 设置某个模式的颜色下拉选项（`payload[0]` 高半字节）。
type BlackSharkRgbColorOptionParams struct {
	Index  int `json:"index"`
	Option int `json:"option"`
}

// BlackSharkLcdDisplayParams LCD 显示参数（CMD 0xC2）的读写参数。
type BlackSharkLcdDisplayParams struct {
	Pos   int   `json:"pos"`
	Items []int `json:"items"`
	// Options 是可选项的有序 id 列表（顺序 = 官方 UI 的排列顺序）。
	Options []int `json:"options"`
	// DefaultItems 是官方「重置屏幕设置」写回的那一组。
	//
	// 唯一所有者是 `types.DefaultBlackSharkLcdItems()`（id 0/1/7 = CPU温度/GPU温度/时间）。
	DefaultItems []int `json:"defaultItems"`
}

// BlackSharkLcdDisplayResult 下发结果：Saved 表示已存进本机配置，Applied 表示帧已写出。
// Saved 是本机配置里确定知道的值，Applied 只表示"帧已写出"，设备是否采纳并不确定，两者分开报。
type BlackSharkLcdDisplayResult struct {
	Saved   bool   `json:"saved"`
	Applied bool   `json:"applied"`
	Pos     int    `json:"pos"`
	Items   []int  `json:"items"`
	Error   string `json:"error,omitempty"`
}

// ClientIssueReport 前端上报的一条"界面上发生的事"（见 ReqReportClientIssue 的注释）。
type ClientIssueReport struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
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

// BlackSharkRestoreConfigParams 用一份快照还原设备配置。
type BlackSharkRestoreConfigParams struct {
	Snapshot types.BlackSharkConfigSnapshot `json:"snapshot"`
}

// SetLightStripParams 设置灯带参数
type SetLightStripParams struct {
	Config types.LightStripConfig `json:"config"`
}

type TransferDeviceImageParams struct {
	DataBase64 string `json:"dataBase64"`
	FileName   string `json:"fileName,omitempty"`
	MimeType   string `json:"mimeType,omitempty"`
	Format     string `json:"format,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
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

type ExportFanCurveProfilesParams struct {
	ProfileIDs []string `json:"profileIds"`
}

// ImportFanCurveProfilesParams 导入曲线方案参数
type ImportFanCurveProfilesParams struct {
	Code string `json:"code"`
}
