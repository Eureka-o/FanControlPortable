//go:build !legacydevice && windows

package device

import (
	"fmt"
	"unsafe"

	"github.com/Eureka-o/FanControlPortable/internal/types"
	"golang.org/x/sys/windows"
)

const (
	blackSharkUSBOutEndpoint = 0x01
	blackSharkUSBInEndpoint  = 0x81
	blackSharkUSBInterface   = 0
)

var (
	libusbDLL                  = windows.NewLazySystemDLL("libusb-1.0.dll")
	libusbInitProc             = libusbDLL.NewProc("libusb_init")
	libusbExitProc             = libusbDLL.NewProc("libusb_exit")
	libusbOpenProc             = libusbDLL.NewProc("libusb_open_device_with_vid_pid")
	libusbCloseProc            = libusbDLL.NewProc("libusb_close")
	libusbClaimInterfaceProc   = libusbDLL.NewProc("libusb_claim_interface")
	libusbReleaseInterfaceProc = libusbDLL.NewProc("libusb_release_interface")
	libusbBulkTransferProc     = libusbDLL.NewProc("libusb_bulk_transfer")
	libusbAutoDetachProc       = libusbDLL.NewProc("libusb_set_auto_detach_kernel_driver")
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
	if len(buf) != blackSharkHIDReportLen {
		return fmt.Errorf("黑鲨 USB report length must be %d, got %d", blackSharkHIDReportLen, len(buf))
	}
	var transferred int32
	r, _, _ := libusbBulkTransferProc.Call(
		d.handle,
		uintptr(endpoint),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&transferred)),
		uintptr(timeout),
	)
	if int32(r) != 0 {
		return fmt.Errorf("libusb bulk transfer endpoint 0x%02X failed: %d", endpoint, int32(r))
	}
	if transferred != int32(len(buf)) {
		return fmt.Errorf("libusb short transfer endpoint 0x%02X: %d/%d", endpoint, transferred, len(buf))
	}
	return nil
}
