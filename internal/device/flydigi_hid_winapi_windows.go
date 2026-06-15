//go:build !legacydevice && windows && !cgo

package device

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"github.com/TIANLI0/THRM/internal/types"
	"golang.org/x/sys/windows"
)

const (
	digcfPresent         = 0x00000002
	digcfDeviceInterface = 0x00000010
	waitObject0          = 0x00000000
	waitTimeout          = 0x00000102
	waitFailed           = 0xFFFFFFFF
)

var (
	hidDLL                       = windows.NewLazySystemDLL("hid.dll")
	setupapiDLL                  = windows.NewLazySystemDLL("setupapi.dll")
	procHidDGetHidGuid           = hidDLL.NewProc("HidD_GetHidGuid")
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

func initFlyDigiHIDAPI() error {
	return nil
}

func exitFlyDigiHIDAPI() error {
	return nil
}

func openFlyDigiHIDDevice(productIDs []uint16) (*flyDigiHIDDevice, error) {
	if len(productIDs) == 0 {
		productIDs = flyDigiHIDProductIDsForProfile(types.LegacyRPMProfileID)
	}

	var lastErr error
	for _, candidate := range scanFlyDigiHIDDevices(productIDs) {
		dev, err := openHIDPath(candidate.path)
		if err != nil {
			lastErr = err
			continue
		}
		dev.productID = candidate.productID
		return dev, nil
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
		r1, _, _ = procSetupDiGetDeviceIfaceDet.Call(
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
	handle, err := windows.CreateFile(
		ptr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		return nil, err
	}
	return &flyDigiHIDDevice{handle: handle, path: path}, nil
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

func (d *flyDigiHIDDevice) SetNonblock(bool) error {
	return nil
}

func (d *flyDigiHIDDevice) WriteReport(report []byte, timeout time.Duration) error {
	if d == nil || d.handle == 0 || d.handle == windows.InvalidHandle {
		return fmt.Errorf("hid device is not open")
	}
	if len(report) == 0 {
		return fmt.Errorf("hid report is empty")
	}

	var failures []string
	if err := d.writeFileReport(report, timeout); err == nil {
		return nil
	} else {
		failures = append(failures, fmt.Sprintf("WriteFile: %v", err))
	}
	if err := d.setOutputReport(report); err == nil {
		return nil
	} else {
		failures = append(failures, fmt.Sprintf("HidD_SetOutputReport: %v", err))
	}
	if len(report) < hidLightReportLen {
		padded := padFlyDigiHIDReport(report, hidLightReportLen)
		if err := d.writeFileReport(padded, timeout); err == nil {
			return nil
		} else {
			failures = append(failures, fmt.Sprintf("WriteFile padded: %v", err))
		}
		if err := d.setOutputReport(padded); err == nil {
			return nil
		} else {
			failures = append(failures, fmt.Sprintf("HidD_SetOutputReport padded: %v", err))
		}
	}
	return fmt.Errorf("flydigi hid write failed (%s)", strings.Join(failures, "; "))
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

func (d *flyDigiHIDDevice) setOutputReport(report []byte) error {
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
	switch event {
	case waitObject0:
		return windows.GetOverlappedResult(handle, ov, done, false)
	case waitTimeout:
		_ = windows.CancelIoEx(handle, ov)
		_ = windows.GetOverlappedResult(handle, ov, done, true)
		if read {
			return errFlyDigiHIDTimeout
		}
		return fmt.Errorf("hid write timeout")
	case waitFailed:
		_ = windows.CancelIoEx(handle, ov)
		return windows.GetLastError()
	default:
		_ = windows.CancelIoEx(handle, ov)
		return fmt.Errorf("unexpected overlapped wait result 0x%X", event)
	}
}
