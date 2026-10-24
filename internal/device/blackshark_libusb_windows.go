//go:build !legacydevice && windows

package device

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/Eureka-o/FanControlPortable/internal/types"
	"golang.org/x/sys/windows"
)

const (
	blackSharkUSBOutEndpoint = 0x01
	blackSharkUSBInEndpoint  = 0x81
	blackSharkUSBInterface   = 0
)

// libusb_error code -7 (LIBUSB_ERROR_TIMEOUT); the read loop must treat it as
// an ordinary poll timeout, not a device failure.
var errBlackSharkUSBTimeout = errors.New("libusb transfer timeout")

// libusbDLL resolves libusb-1.0.dll from the install directory first: the
// portable package ships the DLL next to the executable, and NewLazySystemDLL
// only searches System32, so a bundled DLL would never be found there.
func newLibUSBDLL() *windows.LazyDLL {
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "libusb-1.0.dll")
		if _, err := os.Stat(candidate); err == nil {
			return windows.NewLazyDLL(candidate)
		}
	}
	return windows.NewLazySystemDLL("libusb-1.0.dll")
}

var (
	libusbDLL                  = newLibUSBDLL()
	libusbInitProc             = libusbDLL.NewProc("libusb_init")
	libusbExitProc             = libusbDLL.NewProc("libusb_exit")
	libusbOpenProc             = libusbDLL.NewProc("libusb_open_device_with_vid_pid")
	libusbCloseProc            = libusbDLL.NewProc("libusb_close")
	libusbSetConfigProc        = libusbDLL.NewProc("libusb_set_configuration")
	libusbClaimInterfaceProc   = libusbDLL.NewProc("libusb_claim_interface")
	libusbReleaseInterfaceProc = libusbDLL.NewProc("libusb_release_interface")
	// BRB02 的 0x01/0x81 是 interrupt 端点;官方与已验证的第三方实现都走中断传输,
	// 不用 libusb_bulk_transfer。
	libusbInterruptTransferProc = libusbDLL.NewProc("libusb_interrupt_transfer")
	libusbAutoDetachProc        = libusbDLL.NewProc("libusb_set_auto_detach_kernel_driver")
)

type blackSharkUSBDevice struct {
	ctx    uintptr
	handle uintptr
}

func scanBlackSharkUSBDevice() (string, error) {
	dev, err := openBlackSharkUSBDevice()
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("libusb:vid_%04x&pid_%04x", types.BlackSharkHIDVendorID, types.BlackSharkBRB02HIDProductID)
	_ = dev.close()
	return path, nil
}

func openBlackSharkUSBTransportDevice() (*flyDigiHIDDevice, error) {
	usb, err := openBlackSharkUSBDevice()
	if err != nil {
		return nil, err
	}
	return &flyDigiHIDDevice{
		usb:       usb,
		path:      fmt.Sprintf("libusb:vid_%04x&pid_%04x", types.BlackSharkHIDVendorID, types.BlackSharkBRB02HIDProductID),
		vendorID:  types.BlackSharkHIDVendorID,
		productID: types.BlackSharkBRB02HIDProductID,
	}, nil
}

func openBlackSharkUSBDevice() (*blackSharkUSBDevice, error) {
	// LazyProc.Call panics when the DLL is missing; load it up front so a
	// missing/broken libusb-1.0.dll degrades to an error and lets the caller
	// fall back to BLE/HID instead of crashing the reconnect loop.
	if err := libusbDLL.Load(); err != nil {
		return nil, fmt.Errorf("加载 libusb-1.0.dll 失败: %w", err)
	}
	var ctx uintptr
	if r, _, err := libusbInitProc.Call(uintptr(unsafe.Pointer(&ctx))); int32(r) != 0 {
		return nil, fmt.Errorf("libusb init failed: %v", err)
	}
	if ctx == 0 {
		return nil, fmt.Errorf("libusb init returned nil context")
	}
	handle, _, _ := libusbOpenProc.Call(ctx, uintptr(types.BlackSharkHIDVendorID), uintptr(types.BlackSharkBRB02HIDProductID))
	if handle == 0 {
		libusbExitProc.Call(ctx)
		return nil, fmt.Errorf("黑鲨 USB 设备未找到 (VID 0x%04X PID 0x%04X)", types.BlackSharkHIDVendorID, types.BlackSharkBRB02HIDProductID)
	}
	// 已验证的打开流程:set_configuration(1) 后直接对 EP 读写。Windows 下设备通常
	// 已处于配置态,该调用是 best-effort,失败不阻塞连接。
	libusbSetConfigProc.Call(handle, 1)
	libusbAutoDetachProc.Call(handle, 1)
	if r, _, _ := libusbClaimInterfaceProc.Call(handle, blackSharkUSBInterface); int32(r) != 0 {
		libusbCloseProc.Call(handle)
		libusbExitProc.Call(ctx)
		return nil, fmt.Errorf("libusb claim interface %d failed: %d", blackSharkUSBInterface, int32(r))
	}
	return &blackSharkUSBDevice{ctx: ctx, handle: handle}, nil
}

func (d *blackSharkUSBDevice) close() error {
	if d == nil {
		return nil
	}
	if d.handle != 0 {
		libusbReleaseInterfaceProc.Call(d.handle, blackSharkUSBInterface)
		libusbCloseProc.Call(d.handle)
		d.handle = 0
	}
	if d.ctx != 0 {
		libusbExitProc.Call(d.ctx)
		d.ctx = 0
	}
	return nil
}

func (d *blackSharkUSBDevice) transfer(endpoint byte, buf []byte, timeout uint32) error {
	if d == nil || d.handle == 0 {
		return fmt.Errorf("libusb device is not open")
	}
	// 控制帧 65 字节; 图传会话帧 64 字节 (官方抓包同款, 见 blackshark_image_transport.go)。
	if len(buf) != blackSharkHIDReportLen && len(buf) != blackSharkImageWireLen {
		return fmt.Errorf("黑鲨 USB report length must be %d or %d, got %d", blackSharkHIDReportLen, blackSharkImageWireLen, len(buf))
	}
	var transferred int32
	r, _, _ := libusbInterruptTransferProc.Call(
		d.handle,
		uintptr(endpoint),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&transferred)),
		uintptr(timeout),
	)
	if int32(r) == -7 {
		return errBlackSharkUSBTimeout
	}
	if int32(r) != 0 {
		return fmt.Errorf("libusb interrupt transfer endpoint 0x%02X failed: %d", endpoint, int32(r))
	}
	// Windows 的 WinUSB 后端对 interrupt OUT 上报的 transferred 会向上取整到端点
	// wMaxPacketSize (该设备为 65): 64 字节图传帧会报 "65/64" 短传输。OUT 方向以
	// rc==0 为准 (pyusb/工具箱同样不校验 OUT 字节数), 实际到达由设备 C6 应答流校验;
	// IN 方向短包仍视为异常。
	if endpoint&0x80 != 0 && transferred != int32(len(buf)) {
		return fmt.Errorf("libusb short transfer endpoint 0x%02X: %d/%d", endpoint, transferred, len(buf))
	}
	return nil
}
