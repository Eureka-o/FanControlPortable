//go:build windows

package powernotify

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
	"unsafe"
)

func TestHIDNotifyFilterLayout(t *testing.T) {
	if got := unsafe.Sizeof(hidNotifyFilter{}); got != 416 {
		t.Fatalf("HID notify filter size = %d, want 416", got)
	}
}

func TestBluetoothLENotifyFilterLayout(t *testing.T) {
	if got := unsafe.Sizeof(hidNotifyFilter{}); got != 416 {
		t.Fatalf("BLE notify filter size = %d, want 416", got)
	}
	if !matchesBluetoothLEInterfacePath(`\\?\BTHLEDEVICE#abc`) {
		t.Fatal("Bluetooth LE interface path was not matched")
	}
	if matchesBluetoothLEInterfacePath(`\\?\HID#VID_37D7&PID_1002#abc`) {
		t.Fatal("unrelated HID interface path was matched as Bluetooth LE")
	}
}

func TestHIDInterfacePathAndMatch(t *testing.T) {
	want := `\\?\HID#VID_37D7&PID_1002#abc`
	encoded := append(utf16.Encode([]rune(want)), 0)
	eventData := make([]byte, hidNotifyEventPathOffset+len(encoded)*2)
	for i, value := range encoded {
		binary.LittleEndian.PutUint16(eventData[hidNotifyEventPathOffset+i*2:], value)
	}

	got := hidInterfacePath(unsafe.Pointer(&eventData[0]), uintptr(len(eventData)))
	if got != want {
		t.Fatalf("hidInterfacePath() = %q, want %q", got, want)
	}
	if !matchesHIDInterfacePath(got, []string{"vid_37d7&pid_1002"}) {
		t.Fatal("supported FlyDigi HID path was not matched")
	}
	if matchesHIDInterfacePath(got, []string{"vid_37d7&pid_1003"}) {
		t.Fatal("unrelated FlyDigi HID product was matched")
	}
}

// USB 那条通知不按路径过滤（matches 为 nil），但空路径仍必须丢掉 —— 否则会把没有路径的
// 事件当成"设备到了"，触发一次注定失败的探测。
func TestNotifyShouldFire(t *testing.T) {
	path := `\\?\USB#VID_E2B7&PID_7001#abc`
	cases := []struct {
		name    string
		path    string
		matches func(string) bool
		want    bool
	}{
		{"无过滤且路径非空", path, nil, true},
		{"无过滤但路径为空", "", nil, false},
		{"过滤器命中", path, func(string) bool { return true }, true},
		{"过滤器未命中", path, func(string) bool { return false }, false},
	}
	for _, c := range cases {
		if got := notifyShouldFire(c.path, c.matches); got != c.want {
			t.Fatalf("%s: notifyShouldFire() = %v, want %v", c.name, got, c.want)
		}
	}
}

// BRB02 走 WinUSB，不该被当成 HID 接口；USB 那条用的是通用接口 GUID。
func TestUSBDeviceInterfaceGUID(t *testing.T) {
	if usbDeviceInterfaceGUID == hidInterfaceGUID {
		t.Fatal("USB device interface GUID must not reuse the HID one")
	}
	if usbDeviceInterfaceGUID.Data1 != 0xa5dcbf10 || usbDeviceInterfaceGUID.Data2 != 0x6530 {
		t.Fatalf("USB device interface GUID = %#v", usbDeviceInterfaceGUID)
	}
}
