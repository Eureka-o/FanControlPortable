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

type hidNotifyFilter struct {
	cbSize     uint32
	flags      uint32
	filterType uint32
	reserved   uint32
	classGUID  windows.GUID
	unionTail  [maxDeviceIDLen*2 - 16]byte
}

type hidArrivalNotifier struct {
	handle      uintptr
	callback    uintptr
	identifiers []string
	onArrival   func(string)
	stopOnce    sync.Once
}

func RegisterHIDInterfaceArrivalNotifications(vendorID uint16, productIDs []uint16, onArrival func(string)) (func(), error) {
	if err := procCMRegisterNotification.Find(); err != nil {
		return nil, fmt.Errorf("HID interface notifications are unavailable: %w", err)
	}

	n := &hidArrivalNotifier{onArrival: onArrival}
	for _, productID := range productIDs {
		n.identifiers = append(n.identifiers, fmt.Sprintf("vid_%04x&pid_%04x", vendorID, productID))
	}
	n.callback = windows.NewCallback(func(_ uintptr, _ uintptr, action uint32, eventData unsafe.Pointer, eventDataSize uint32) uintptr {
		defer func() { _ = recover() }()
		if action != hidNotifyActionArrival || eventData == nil {
			return 0
		}
		path := hidInterfacePath(eventData, uintptr(eventDataSize))
		if path != "" && matchesHIDInterfacePath(path, n.identifiers) && n.onArrival != nil {
			go n.onArrival(path)
		}
		return 0
	})

	filter := hidNotifyFilter{
		cbSize:     uint32(unsafe.Sizeof(hidNotifyFilter{})),
		filterType: hidNotifyFilterTypeInterface,
		classGUID:  hidInterfaceGUID,
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
