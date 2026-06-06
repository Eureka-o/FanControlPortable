//go:build windows

package deviceprofileexec

import (
	"fmt"
	"io"
	"strings"
	"unsafe"

	"github.com/TIANLI0/THRM/internal/types"
	"golang.org/x/sys/windows"
)

type DefaultSerialDialer struct{}

func (DefaultSerialDialer) OpenSerialPort(profile types.DeviceProfile) (SerialPort, error) {
	profile = types.NormalizeDeviceProfile(profile, "")
	name := strings.TrimSpace(profile.Connection.SerialPort)
	if name == "" {
		return nil, fmt.Errorf("serial profile does not define a COM port")
	}
	path := serialDevicePath(name)
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		ptr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}

	port := &windowsSerialPort{handle: handle}
	if err := configureWindowsSerialPort(handle, profile); err != nil {
		_ = port.Close()
		return nil, err
	}
	return port, nil
}

type windowsSerialPort struct {
	handle windows.Handle
}

func (p *windowsSerialPort) Read(buf []byte) (int, error) {
	var done uint32
	err := windows.ReadFile(p.handle, buf, &done, nil)
	if err != nil {
		return int(done), err
	}
	if done == 0 {
		return 0, io.EOF
	}
	return int(done), nil
}

func (p *windowsSerialPort) Write(buf []byte) (int, error) {
	var done uint32
	err := windows.WriteFile(p.handle, buf, &done, nil)
	return int(done), err
}

func (p *windowsSerialPort) Close() error {
	if p.handle == 0 || p.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(p.handle)
	p.handle = windows.InvalidHandle
	return err
}

func configureWindowsSerialPort(handle windows.Handle, profile types.DeviceProfile) error {
	connection := profile.Connection
	var dcb windows.DCB
	dcb.DCBlength = uint32(unsafe.Sizeof(dcb))
	if err := windows.GetCommState(handle, &dcb); err != nil {
		return err
	}
	baud := connection.SerialBaudRate
	if baud <= 0 {
		baud = 115200
	}
	dataBits := connection.SerialDataBits
	if dataBits <= 0 {
		dataBits = 8
	}
	dcb.BaudRate = uint32(baud)
	dcb.ByteSize = uint8(dataBits)
	dcb.Parity = windowsSerialParity(connection.SerialParity)
	dcb.StopBits = windowsSerialStopBits(connection.SerialStopBits)
	if err := windows.SetCommState(handle, &dcb); err != nil {
		return err
	}

	timeoutMs := uint32(durationFromMillis(connection.RequestTimeoutMs, defaultHTTPTimeout, maxProfileHTTPTimeout).Milliseconds())
	if timeoutMs == 0 {
		timeoutMs = uint32(defaultHTTPTimeout.Milliseconds())
	}
	timeouts := windows.CommTimeouts{
		ReadIntervalTimeout:         50,
		ReadTotalTimeoutMultiplier:  0,
		ReadTotalTimeoutConstant:    timeoutMs,
		WriteTotalTimeoutMultiplier: 0,
		WriteTotalTimeoutConstant:   timeoutMs,
	}
	return windows.SetCommTimeouts(handle, &timeouts)
}

func serialDevicePath(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, `\\.\`) {
		return name
	}
	upper := strings.ToUpper(name)
	if strings.HasPrefix(upper, "COM") {
		return `\\.\` + name
	}
	return name
}

func windowsSerialParity(value string) uint8 {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "odd":
		return windows.ODDPARITY
	case "even":
		return windows.EVENPARITY
	case "mark":
		return windows.MARKPARITY
	case "space":
		return windows.SPACEPARITY
	default:
		return windows.NOPARITY
	}
}

func windowsSerialStopBits(value int) uint8 {
	switch value {
	case 2:
		return windows.TWOSTOPBITS
	default:
		return windows.ONESTOPBIT
	}
}
