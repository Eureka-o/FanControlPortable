//go:build !legacydevice && windows

package device

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
	"golang.org/x/sys/windows"
)

const (
	hidControlReportLen = 23
	hidLightReportLen   = 65

	digcfPresent         = 0x00000002
	digcfDeviceInterface = 0x00000010
	waitTimeout          = 0x00000102
)

var (
	errFlyDigiHIDTimeout = errors.New("hid read timeout")

	hidDLL                       = windows.NewLazySystemDLL("hid.dll")
	setupapiDLL                  = windows.NewLazySystemDLL("setupapi.dll")
	procHidDGetHidGuid           = hidDLL.NewProc("HidD_GetHidGuid")
	procHidDGetAttributes        = hidDLL.NewProc("HidD_GetAttributes")
	procHidDSetOutputReport      = hidDLL.NewProc("HidD_SetOutputReport")
	procSetupDiGetClassDevsW     = setupapiDLL.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceIfaces  = setupapiDLL.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceIfaceDet = setupapiDLL.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiDestroyInfoList   = setupapiDLL.NewProc("SetupDiDestroyDeviceInfoList")
)

type flyDigiHIDDevice struct {
	handle    windows.Handle
	path      string
	productID uint16
}

type flyDigiHIDCandidate struct {
	path      string
	productID uint16
}

type hidAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
}

type spDeviceInterfaceData struct {
	CbSize             uint32
	InterfaceClassGuid windows.GUID
	Flags              uint32
	Reserved           uintptr
}

type spDeviceInterfaceDetailData struct {
	CbSize     uint32
	DevicePath [1]uint16
}

func (m *Manager) shouldUseLegacyHIDLocked() bool {
	return m.deviceTransport == types.DeviceTransportHID
}

func initFlyDigiHIDAPI() error {
	return nil
}

func exitFlyDigiHIDAPI() error {
	return nil
}

func (m *Manager) connectLegacyHIDLocked() (bool, map[string]string) {
	return m.connectFlyDigiHIDLocked()
}

func (m *Manager) connectFlyDigiHIDLocked() (bool, map[string]string) {
	if !m.shouldUseLegacyHIDLocked() {
		return false, nil
	}

	productIDs := flyDigiHIDProductIDsForProfile(m.activeProfile.ID)
	dev, err := openFlyDigiHIDDevice(productIDs)
	if err != nil {
		m.logWarn("飞智 HID 设备连接失败: %v", err)
		m.isConnected = false
		m.deviceType = ""
		m.productID = 0
		m.flyDigiHID = nil
		m.currentFanData.Store(nil)
		return false, m.flyDigiHIDInfoLocked(0, "")
	}

	m.flyDigiHID = dev
	m.isConnected = true
	m.deviceType = types.DeviceTransportHID
	m.productID = dev.productID
	if profile, ok := types.FlyDigiProfileForHIDProductID(dev.productID); ok {
		m.activeProfile = profile
	}
	m.currentFanData.Store(&types.FanData{
		ReportID:  deviceproto.ReportID,
		MagicSync: 0x5AA5,
		Transport: types.DeviceTransportHID,
		SpeedUnit: types.FanSpeedUnitRPM,
		WorkMode:  "HID 已连接",
	})

	m.startFlyDigiHIDReaderLocked(dev)

	info := m.flyDigiHIDInfoLocked(dev.productID, dev.path)
	m.logInfo("飞智 HID 设备连接成功: %s", info["product"])
	return true, info
}

func (m *Manager) disconnectFlyDigiHIDLocked() bool {
	if m.deviceType != types.DeviceTransportHID {
		return false
	}
	m.closeFlyDigiHIDLocked()
	m.isConnected = false
	m.deviceType = ""
	m.productID = 0
	m.currentFanData.Store(nil)
	return true
}

func (m *Manager) closeFlyDigiHIDLocked() {
	if m.flyDigiHID == nil {
		return
	}
	m.stopFlyDigiHIDReaderLocked()
	if err := m.flyDigiHID.Close(); err != nil {
		m.logWarn("飞智 HID 设备关闭失败: %v", err)
	}
	m.flyDigiHID = nil
}

func (m *Manager) startFlyDigiHIDReaderLocked(dev *flyDigiHIDDevice) {
	m.stopFlyDigiHIDReaderLocked()
	stop := make(chan struct{})
	done := make(chan struct{})
	m.flyDigiHIDStop = stop
	m.flyDigiHIDDone = done
	go m.readFlyDigiHIDLoop(dev, stop, done)
}

func (m *Manager) stopFlyDigiHIDReaderLocked() {
	stop := m.flyDigiHIDStop
	done := m.flyDigiHIDDone
	m.flyDigiHIDStop = nil
	m.flyDigiHIDDone = nil
	if stop == nil {
		return
	}
	close(stop)
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(800 * time.Millisecond):
		m.logWarn("FlyDigi HID reader stop timed out")
	}
}

func (m *Manager) readFlyDigiHIDLoop(dev *flyDigiHIDDevice, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	if dev == nil {
		return
	}
	for {
		select {
		case <-stop:
			return
		default:
		}

		raw, err := dev.ReadReport(500 * time.Millisecond)
		if err != nil {
			if errors.Is(err, errFlyDigiHIDTimeout) {
				continue
			}
			select {
			case <-stop:
				return
			default:
				m.logWarn("FlyDigi HID read failed: %v", err)
				return
			}
		}
		if len(raw) == 0 {
			continue
		}
		m.handleFlyDigiHIDRX(raw)
	}
}

func (m *Manager) handleFlyDigiHIDRX(raw []byte) {
	m.recordDebugFrame("rx", types.DeviceTransportHID, raw)
	if fanData := parseFlyDigiFanData(raw); fanData != nil {
		m.currentFanData.Store(fanData)
		if m.onFanDataUpdate != nil {
			go m.onFanDataUpdate(fanData)
		}
	}
}

func (m *Manager) flyDigiHIDInfoLocked(productID uint16, path string) map[string]string {
	displayName := m.activeProfileDisplayNameLocked(flyDigiHIDModelName(productID))
	model := strings.TrimSpace(m.activeProfile.Model)
	if productID != 0 && m.activeProfile.ID == types.LegacyRPMProfileID {
		model = flyDigiHIDModelName(productID)
		displayName = "飞智（FlyDigi）" + model
	}
	if model == "" {
		model = flyDigiHIDModelName(productID)
	}
	if model == "" {
		model = displayName
	}
	return map[string]string{
		"manufacturer": "飞智（FlyDigi）",
		"product":      displayName,
		"serial":       path,
		"model":        model,
		"transport":    types.DeviceTransportHID,
		"endpoint":     path,
		"productId":    fmt.Sprintf("0x%04X", productID),
	}
}

func flyDigiHIDModelName(productID uint16) string {
	switch productID {
	case types.FlyDigiBS2ProductID:
		return "BS2"
	case types.FlyDigiBS2PROProductID:
		return "BS2PRO"
	case types.FlyDigiBS3ProductID:
		return "BS3"
	case types.FlyDigiBS3PROProductID:
		return "BS3PRO"
	default:
		return "FlyDigi HID"
	}
}

func flyDigiHIDProductIDsForProfile(profileID string) []uint16 {
	switch strings.TrimSpace(profileID) {
	case types.FlyDigiBS2ProfileID:
		return []uint16{types.FlyDigiBS2ProductID}
	case types.FlyDigiBS2PROProfileID:
		return []uint16{types.FlyDigiBS2PROProductID}
	case types.FlyDigiBS3ProfileID:
		return []uint16{types.FlyDigiBS3ProductID}
	case types.FlyDigiBS3PROProfileID:
		return []uint16{types.FlyDigiBS3PROProductID}
	default:
		return []uint16{
			types.FlyDigiBS2PROProductID,
			types.FlyDigiBS3ProductID,
			types.FlyDigiBS3PROProductID,
			types.FlyDigiBS2ProductID,
		}
	}
}

func openFlyDigiHIDDevice(productIDs []uint16) (*flyDigiHIDDevice, error) {
	wanted := make(map[uint16]bool, len(productIDs))
	for _, id := range productIDs {
		wanted[id] = true
	}
	var lastErr error
	for _, candidate := range scanFlyDigiHIDDevices(productIDs) {
		dev, err := openHIDPath(candidate.path)
		if err != nil {
			lastErr = err
			continue
		}
		attrs, err := dev.Attributes()
		if attrs.VendorID == types.FlyDigiHIDVendorID && wanted[attrs.ProductID] {
			dev.productID = attrs.ProductID
			return dev, nil
		}
		if candidate.productID != 0 && wanted[candidate.productID] {
			dev.productID = candidate.productID
			return dev, nil
		}
		if err != nil {
			lastErr = err
		}
		_ = dev.Close()
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("未找到匹配的飞智 HID 设备")
}

func scanFlyDigiHIDDevices(productIDs []uint16) []flyDigiHIDCandidate {
	paths, err := enumerateHIDDevicePaths()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	candidates := make([]flyDigiHIDCandidate, 0)
	for _, path := range paths {
		productID, ok := flyDigiHIDProductIDFromPath(path, productIDs)
		if !ok || seen[path] {
			continue
		}
		seen[path] = true
		candidates = append(candidates, flyDigiHIDCandidate{
			path:      path,
			productID: productID,
		})
	}
	return candidates
}

func flyDigiHIDProductIDFromPath(path string, productIDs []uint16) (uint16, bool) {
	lower := strings.ToLower(path)
	vendor := fmt.Sprintf("%04x", types.FlyDigiHIDVendorID)
	vendorMatched := strings.Contains(lower, "vid_"+vendor) ||
		strings.Contains(lower, "vid&"+vendor) ||
		strings.Contains(lower, "vid&01"+vendor) ||
		strings.Contains(lower, "dev_vid&"+vendor) ||
		strings.Contains(lower, "dev_vid&01"+vendor)
	if !vendorMatched {
		return 0, false
	}
	for _, id := range productIDs {
		pid := fmt.Sprintf("%04x", id)
		if strings.Contains(lower, "pid_"+pid) || strings.Contains(lower, "pid&"+pid) {
			return id, true
		}
	}
	return 0, false
}

func enumerateHIDDevicePaths() ([]string, error) {
	var guid windows.GUID
	procHidDGetHidGuid.Call(uintptr(unsafe.Pointer(&guid)))

	r1, _, err := procSetupDiGetClassDevsW.Call(
		uintptr(unsafe.Pointer(&guid)),
		0,
		0,
		uintptr(digcfPresent|digcfDeviceInterface),
	)
	if r1 == uintptr(windows.InvalidHandle) {
		return nil, err
	}
	infoSet := windows.Handle(r1)
	defer procSetupDiDestroyInfoList.Call(uintptr(infoSet))

	var paths []string
	for index := uint32(0); ; index++ {
		ifaceData := spDeviceInterfaceData{CbSize: uint32(unsafe.Sizeof(spDeviceInterfaceData{}))}
		r1, _, err = procSetupDiEnumDeviceIfaces.Call(
			uintptr(infoSet),
			0,
			uintptr(unsafe.Pointer(&guid)),
			uintptr(index),
			uintptr(unsafe.Pointer(&ifaceData)),
		)
		if r1 == 0 {
			if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
				break
			}
			return paths, err
		}

		var required uint32
		procSetupDiGetDeviceIfaceDet.Call(
			uintptr(infoSet),
			uintptr(unsafe.Pointer(&ifaceData)),
			0,
			0,
			uintptr(unsafe.Pointer(&required)),
			0,
		)
		if required == 0 {
			continue
		}
		buf := make([]byte, required)
		detail := (*spDeviceInterfaceDetailData)(unsafe.Pointer(&buf[0]))
		detail.CbSize = uint32(unsafe.Sizeof(spDeviceInterfaceDetailData{}))
		r1, _, err = procSetupDiGetDeviceIfaceDet.Call(
			uintptr(infoSet),
			uintptr(unsafe.Pointer(&ifaceData)),
			uintptr(unsafe.Pointer(detail)),
			uintptr(required),
			uintptr(unsafe.Pointer(&required)),
			0,
		)
		if r1 == 0 {
			continue
		}

		offset := unsafe.Offsetof(spDeviceInterfaceDetailData{}.DevicePath)
		chars := int((uintptr(required) - offset) / unsafe.Sizeof(uint16(0)))
		if chars <= 0 {
			continue
		}
		u16 := unsafe.Slice((*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0]))+offset)), chars)
		path := windows.UTF16ToString(u16)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func openHIDPath(path string) (*flyDigiHIDDevice, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, access := range []uint32{windows.GENERIC_READ | windows.GENERIC_WRITE} {
		handle, err := windows.CreateFile(
			ptr,
			access,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OVERLAPPED,
			0,
		)
		if err == nil {
			return &flyDigiHIDDevice{handle: handle, path: path}, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (d *flyDigiHIDDevice) Attributes() (hidAttributes, error) {
	attrs := hidAttributes{Size: uint32(unsafe.Sizeof(hidAttributes{}))}
	r1, _, err := procHidDGetAttributes.Call(uintptr(d.handle), uintptr(unsafe.Pointer(&attrs)))
	if r1 == 0 {
		return attrs, err
	}
	return attrs, nil
}

func (d *flyDigiHIDDevice) Close() error {
	if d == nil {
		return nil
	}
	if d.handle == 0 || d.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(d.handle)
	d.handle = 0
	return err
}

func (d *flyDigiHIDDevice) WriteReport(report []byte, timeout time.Duration) error {
	if d == nil || d.handle == 0 || d.handle == windows.InvalidHandle {
		return fmt.Errorf("hid device is not open")
	}
	if len(report) == 0 {
		return fmt.Errorf("hid report is empty")
	}
	writeReport := padFlyDigiHIDReport(report, hidLightReportLen)
	var failures []string
	if err := d.writeFileReport(writeReport, timeout); err == nil {
		return nil
	} else {
		failures = append(failures, fmt.Sprintf("WriteFile: %v", err))
	}
	if err := d.SetOutputReport(writeReport); err == nil {
		return nil
	} else {
		failures = append(failures, fmt.Sprintf("HidD_SetOutputReport: %v", err))
	}
	return fmt.Errorf("flydigi hid write failed (%s)", strings.Join(failures, "; "))
}

func padFlyDigiHIDReport(report []byte, size int) []byte {
	if len(report) >= size {
		return report
	}
	padded := make([]byte, size)
	copy(padded, report)
	return padded
}

func (d *flyDigiHIDDevice) writeFileReport(report []byte, timeout time.Duration) error {
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(event)

	ov := windows.Overlapped{HEvent: event}
	var done uint32
	err = windows.WriteFile(d.handle, report, &done, &ov)
	if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return err
	}
	if err == nil {
		return nil
	}
	return waitOverlapped(d.handle, &ov, &done, timeout, false)
}

func (d *flyDigiHIDDevice) SetOutputReport(report []byte) error {
	if len(report) == 0 {
		return fmt.Errorf("hid report is empty")
	}
	r1, _, err := procHidDSetOutputReport.Call(
		uintptr(d.handle),
		uintptr(unsafe.Pointer(&report[0])),
		uintptr(len(report)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

func (d *flyDigiHIDDevice) ReadReport(timeout time.Duration) ([]byte, error) {
	if d == nil || d.handle == 0 || d.handle == windows.InvalidHandle {
		return nil, fmt.Errorf("hid device is not open")
	}
	buf := make([]byte, hidLightReportLen)
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(event)

	ov := windows.Overlapped{HEvent: event}
	var done uint32
	err = windows.ReadFile(d.handle, buf, &done, &ov)
	if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return nil, err
	}
	if err != nil {
		if err := waitOverlapped(d.handle, &ov, &done, timeout, true); err != nil {
			return nil, err
		}
	}
	return buf[:done], nil
}

func waitOverlapped(handle windows.Handle, ov *windows.Overlapped, done *uint32, timeout time.Duration, read bool) error {
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	ms := uint32(timeout / time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	event, err := windows.WaitForSingleObject(ov.HEvent, ms)
	if err != nil {
		return err
	}
	if event == waitTimeout {
		_ = windows.CancelIoEx(handle, ov)
		_ = windows.GetOverlappedResult(handle, ov, done, true)
		if read {
			return errFlyDigiHIDTimeout
		}
		return fmt.Errorf("hid write timeout")
	}
	return windows.GetOverlappedResult(handle, ov, done, false)
}

func (m *Manager) writeFlyDigiHIDFrameLocked(cmd byte, payload []byte, reportLen int) error {
	if m.flyDigiHID == nil {
		return fmt.Errorf("飞智 HID 设备未连接")
	}
	frame := deviceproto.BuildFrame(cmd, payload...)
	report := deviceproto.BuildReport(frame, reportLen)
	m.recordDebugFrame("tx", types.DeviceTransportHID, report)
	return m.flyDigiHID.WriteReport(report, 800*time.Millisecond)
}

func (m *Manager) setFlyDigiHIDTargetSpeedLocked(speed types.FanSpeedValue) bool {
	if !m.isConnected || m.flyDigiHID == nil {
		return false
	}
	speed = speed.Normalized()
	if !types.IsRPMSpeedUnit(speed.Unit) {
		m.logWarn("飞智 HID 设备不支持百分比直控")
		return false
	}
	rpm := types.ClampRPM(speed.Value)
	if rpm > 4000 {
		rpm = 4000
	}
	if err := m.writeFlyDigiHIDFrameLocked(deviceproto.CmdEnterRealtimeRPM, nil, hidControlReportLen); err != nil {
		m.logError("进入实时转速模式失败: %v", err)
		return false
	}
	time.Sleep(50 * time.Millisecond)

	payload := make([]byte, 2)
	binary.LittleEndian.PutUint16(payload, uint16(rpm))
	if err := m.writeFlyDigiHIDFrameLocked(deviceproto.CmdSetRealtimeRPM, payload, hidControlReportLen); err != nil {
		m.logError("设置飞智 HID 转速失败: %v", err)
		return false
	}
	m.storeFlyDigiHIDFanDataLocked(rpm, "自动模式(实时转速)")
	return true
}

func (m *Manager) setFlyDigiHIDManualGearRPM(gear, level string, rpm int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isConnected || m.flyDigiHID == nil {
		return false
	}
	idx, ok := types.GearIndex(gear)
	if !ok {
		m.logError("未知挡位 %s", gear)
		return false
	}
	rpm = types.ClampRPM(rpm)
	cmd := types.BuildGearRPMCommand(idx, rpm)
	report := deviceproto.BuildReport(cmd, hidControlReportLen)
	m.recordDebugFrame("tx", types.DeviceTransportHID, report)
	if err := m.flyDigiHID.WriteReport(report, 800*time.Millisecond); err != nil {
		m.logError("设置飞智 HID 挡位 %s %s (%d RPM) 失败: %v", gear, level, rpm, err)
		return false
	}
	m.storeFlyDigiHIDFanDataLocked(rpm, "挡位工作模式")
	return true
}

func (m *Manager) setFlyDigiHIDGearLight(enabled bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	payload := byte(0x00)
	if enabled {
		payload = 0x01
	}
	if err := m.writeFlyDigiHIDFrameLocked(deviceproto.CmdGearLight, []byte{payload}, hidControlReportLen); err != nil {
		m.logError("设置飞智 HID 挡位灯失败: %v", err)
		return false
	}
	return true
}

func (m *Manager) setFlyDigiHIDPowerOnStart(enabled bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	payload := byte(0x02)
	if enabled {
		payload = 0x01
	}
	if err := m.writeFlyDigiHIDFrameLocked(deviceproto.CmdSetPowerOnStart, []byte{payload}, hidControlReportLen); err != nil {
		m.logError("设置飞智 HID 通电自启失败: %v", err)
		return false
	}
	return true
}

func (m *Manager) setFlyDigiHIDSmartStartStop(mode string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	var payload byte
	switch mode {
	case "off":
		payload = 0x00
	case "immediate":
		payload = 0x01
	case "delayed":
		payload = 0x02
	default:
		return false
	}
	if err := m.writeFlyDigiHIDFrameLocked(deviceproto.CmdSetSmartStartStop, []byte{payload}, hidControlReportLen); err != nil {
		m.logError("设置飞智 HID 智能启停失败: %v", err)
		return false
	}
	return true
}

func (m *Manager) setFlyDigiHIDBrightness(percentage int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if percentage < 0 || percentage > 100 {
		return false
	}
	switch percentage {
	case 0:
		payload := []byte{0x1C, 0x00, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		if err := m.writeFlyDigiHIDFrameLocked(0x47, payload, hidControlReportLen); err != nil {
			m.logError("关闭飞智 HID 亮度失败: %v", err)
			return false
		}
	case 100:
		if err := m.writeFlyDigiHIDFrameLocked(0x43, nil, hidControlReportLen); err != nil {
			m.logError("设置飞智 HID 全亮失败: %v", err)
			return false
		}
	default:
		return false
	}
	return true
}

func (m *Manager) storeFlyDigiHIDFanDataLocked(rpm int, workMode string) {
	rpm = types.ClampRPM(rpm)
	fanData := &types.FanData{
		ReportID:    deviceproto.ReportID,
		MagicSync:   0x5AA5,
		Command:     deviceproto.CmdSetRealtimeRPM,
		CurrentRPM:  0,
		TargetRPM:   uint16(rpm),
		Transport:   types.DeviceTransportHID,
		SpeedUnit:   types.FanSpeedUnitRPM,
		WorkMode:    workMode,
		CurrentMode: 0x01,
	}
	if previous := m.currentFanData.Load(); previous != nil && previous.Transport == types.DeviceTransportHID {
		fanData = &types.FanData{
			ReportID:     previous.ReportID,
			MagicSync:    previous.MagicSync,
			Command:      deviceproto.CmdSetRealtimeRPM,
			Status:       previous.Status,
			GearSettings: previous.GearSettings,
			CurrentMode:  previous.CurrentMode,
			Reserved1:    previous.Reserved1,
			CurrentRPM:   previous.CurrentRPM,
			TargetRPM:    uint16(rpm),
			MaxGear:      previous.MaxGear,
			SetGear:      previous.SetGear,
			WorkMode:     workMode,
			Transport:    types.DeviceTransportHID,
			SpeedUnit:    types.FanSpeedUnitRPM,
		}
		if fanData.ReportID == 0 {
			fanData.ReportID = deviceproto.ReportID
		}
		if fanData.MagicSync == 0 {
			fanData.MagicSync = 0x5AA5
		}
	}
	m.currentFanData.Store(fanData)
	if m.onFanDataUpdate != nil {
		go m.onFanDataUpdate(fanData)
	}
}
