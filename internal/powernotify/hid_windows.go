//go:build windows

package powernotify

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modCfgMgr32                  = windows.NewLazySystemDLL("cfgmgr32.dll")
	procCMRegisterNotification   = modCfgMgr32.NewProc("CM_Register_Notification")
	procCMUnregisterNotification = modCfgMgr32.NewProc("CM_Unregister_Notification")
)

const (
	hidNotifyFilterTypeInterface = 0
	hidNotifyActionArrival       = 0
	hidNotifyEventPathOffset     = 24
	maxDeviceIDLen               = 200
)

var hidInterfaceGUID = windows.GUID{
	Data1: 0x4d1e55b2,
	Data2: 0xf16f,
	Data3: 0x11cf,
	Data4: [8]byte{0x88, 0xcb, 0x00, 0x11, 0x11, 0x00, 0x00, 0x30},
}

// GUID_BLUETOOTHLE_DEVICE_INTERFACE from bthledef.h.
var bluetoothLEInterfaceGUID = windows.GUID{
	Data1: 0x781aee18,
	Data2: 0x7733,
	Data3: 0x4ce4,
	Data4: [8]byte{0xad, 0xd0, 0x91, 0xf4, 0x1c, 0x67, 0xb5, 0x92},
}

// GUID_DEVINTERFACE_USB_DEVICE from usbiodef.h；每个 USB 设备都会挂上它，与绑定哪个驱动无关。
var usbDeviceInterfaceGUID = windows.GUID{
	Data1: 0xa5dcbf10,
	Data2: 0x6530,
	Data3: 0x11d2,
	Data4: [8]byte{0x90, 0x1f, 0x00, 0xc0, 0x4f, 0xb9, 0x51, 0xed},
}

type hidNotifyFilter struct {
	cbSize     uint32
	flags      uint32
	filterType uint32
	reserved   uint32
	classGUID  windows.GUID
	unionTail  [maxDeviceIDLen*2 - 16]byte
}

type hidArrivalNotifier struct {
	handle    uintptr
	callback  uintptr
	matches   func(string) bool
	onArrival func(string)
	stopOnce  sync.Once
}

func RegisterHIDInterfaceArrivalNotifications(vendorID uint16, productIDs []uint16, onArrival func(string)) (func(), error) {
	identifiers := make([]string, 0, len(productIDs))
	for _, productID := range productIDs {
		identifiers = append(identifiers, fmt.Sprintf("vid_%04x&pid_%04x", vendorID, productID))
	}
	return registerInterfaceArrivalNotifications(
		hidInterfaceGUID,
		func(path string) bool { return matchesHIDInterfacePath(path, identifiers) },
		onArrival,
	)
}

func RegisterBluetoothLEInterfaceArrivalNotifications(onArrival func(string)) (func(), error) {
	return registerInterfaceArrivalNotifications(
		bluetoothLEInterfaceGUID,
		matchesBluetoothLEInterfacePath,
		onArrival,
	)
}

// RegisterUSBDeviceInterfaceArrivalNotifications 注册"有 USB 设备接口到达"的通知。
//
// 比 HID 那条粗：它不按 VID/PID 过滤（设备路径里未必带得上），凡是 USB 设备到达都回调一次。
// 需要它的原因：走 WinUSB（批量端点）的设备**不会**触发 GUID_DEVINTERFACE_HID，
// 而这类设备恰恰是"插上就该切过去"的那一类。真伪判断交给调用方自己的存在性探针。
func RegisterUSBDeviceInterfaceArrivalNotifications(onArrival func(string)) (func(), error) {
	return registerInterfaceArrivalNotifications(usbDeviceInterfaceGUID, nil, onArrival)
}

func registerInterfaceArrivalNotifications(classGUID windows.GUID, matches func(string) bool, onArrival func(string)) (func(), error) {
	if err := procCMRegisterNotification.Find(); err != nil {
		return nil, fmt.Errorf("device interface notifications are unavailable: %w", err)
	}

	n := &hidArrivalNotifier{matches: matches, onArrival: onArrival}
	n.callback = windows.NewCallback(func(_ uintptr, _ uintptr, action uint32, eventData unsafe.Pointer, eventDataSize uint32) uintptr {
		defer func() { _ = recover() }()
		if action != hidNotifyActionArrival || eventData == nil {
			return 0
		}
		path := hidInterfacePath(eventData, uintptr(eventDataSize))
		if notifyShouldFire(path, n.matches) && n.onArrival != nil {
			go n.onArrival(path)
		}
		return 0
	})

	filter := hidNotifyFilter{
		cbSize:     uint32(unsafe.Sizeof(hidNotifyFilter{})),
		filterType: hidNotifyFilterTypeInterface,
		classGUID:  classGUID,
	}
	ret, _, callErr := procCMRegisterNotification.Call(
		uintptr(unsafe.Pointer(&filter)),
		0,
		n.callback,
		uintptr(unsafe.Pointer(&n.handle)),
	)
	if ret != 0 {
		return nil, fmt.Errorf("register HID interface notifications failed (%d): %v", ret, callErr)
	}

	return func() {
		n.stopOnce.Do(func() {
			if n.handle != 0 {
				_, _, _ = procCMUnregisterNotification.Call(n.handle)
				n.handle = 0
			}
		})
	}, nil
}

func hidInterfacePath(eventData unsafe.Pointer, eventDataSize uintptr) string {
	if eventDataSize <= hidNotifyEventPathOffset {
		return ""
	}
	length := (eventDataSize - hidNotifyEventPathOffset) / 2
	path := (*uint16)(unsafe.Add(eventData, hidNotifyEventPathOffset))
	return windows.UTF16ToString(unsafe.Slice(path, length))
}

func matchesHIDInterfacePath(path string, identifiers []string) bool {
	lower := strings.ToLower(path)
	for _, identifier := range identifiers {
		if strings.Contains(lower, identifier) {
			return true
		}
	}
	return false
}

// notifyShouldFire 决定一条到达事件要不要交给回调：路径为空则丢弃；
// matches 为 nil 表示"不按路径过滤"（USB 那条），由调用方自己再判。
func notifyShouldFire(path string, matches func(string) bool) bool {
	if path == "" {
		return false
	}
	return matches == nil || matches(path)
}

func matchesBluetoothLEInterfacePath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "bthledevice") || strings.Contains(lower, "bthenum")
}
