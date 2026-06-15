//go:build !legacydevice && windows && cgo

package device

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
	"github.com/sstallion/go-hid"
)

type flyDigiHIDDevice struct {
	device    *hid.Device
	path      string
	productID uint16
}

type flyDigiHIDCandidate struct {
	path      string
	productID uint16
}

func initFlyDigiHIDAPI() error {
	return hid.Init()
}

func exitFlyDigiHIDAPI() error {
	return hid.Exit()
}

func openFlyDigiHIDDevice(productIDs []uint16) (*flyDigiHIDDevice, error) {
	if len(productIDs) == 0 {
		productIDs = flyDigiHIDProductIDsForProfile(types.LegacyRPMProfileID)
	}

	var lastErr error
	for _, productID := range productIDs {
		dev, err := hid.OpenFirst(types.FlyDigiHIDVendorID, productID)
		if err != nil {
			lastErr = err
			continue
		}

		wrapped := &flyDigiHIDDevice{
			device:    dev,
			productID: productID,
		}
		if info, err := dev.GetDeviceInfo(); err == nil && info != nil {
			wrapped.path = info.Path
			if info.ProductID != 0 {
				wrapped.productID = info.ProductID
			}
		}
		return wrapped, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("未找到匹配的飞智 HID 设备")
}

func scanFlyDigiHIDDevices(productIDs []uint16) []flyDigiHIDCandidate {
	wanted := map[uint16]bool{}
	for _, id := range productIDs {
		wanted[id] = true
	}

	seen := map[string]bool{}
	candidates := make([]flyDigiHIDCandidate, 0)
	_ = hid.Enumerate(types.FlyDigiHIDVendorID, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
		if info == nil {
			return nil
		}
		productID := info.ProductID
		if len(wanted) > 0 && !wanted[productID] {
			return nil
		}
		path := strings.TrimSpace(info.Path)
		if path == "" {
			path = fmt.Sprintf("hidapi:vid_%04x&pid_%04x", info.VendorID, info.ProductID)
		}
		if seen[path] {
			return nil
		}
		seen[path] = true
		candidates = append(candidates, flyDigiHIDCandidate{
			path:      path,
			productID: productID,
		})
		return nil
	})
	return candidates
}

func (d *flyDigiHIDDevice) Close() error {
	if d == nil || d.device == nil {
		return nil
	}
	err := d.device.Close()
	d.device = nil
	return err
}

func (d *flyDigiHIDDevice) SetNonblock(nonblocking bool) error {
	if d == nil || d.device == nil {
		return fmt.Errorf("hid device is not open")
	}
	return d.device.SetNonblock(nonblocking)
}

func (d *flyDigiHIDDevice) WriteReport(report []byte, timeout time.Duration) error {
	if d == nil || d.device == nil {
		return fmt.Errorf("hid device is not open")
	}
	if len(report) == 0 {
		return fmt.Errorf("hid report is empty")
	}
	if timeout > 0 {
		d.device.SetWriteTimeout(int(timeout / time.Millisecond))
	}

	var failures []string
	if _, err := d.device.Write(report); err == nil {
		return nil
	} else {
		failures = append(failures, fmt.Sprintf("Write: %v", err))
	}
	if _, err := d.device.SendOutputReport(report); err == nil {
		return nil
	} else {
		failures = append(failures, fmt.Sprintf("SendOutputReport: %v", err))
	}
	if len(report) < hidLightReportLen {
		padded := padFlyDigiHIDReport(report, hidLightReportLen)
		if _, err := d.device.Write(padded); err == nil {
			return nil
		} else {
			failures = append(failures, fmt.Sprintf("Write padded: %v", err))
		}
		if _, err := d.device.SendOutputReport(padded); err == nil {
			return nil
		} else {
			failures = append(failures, fmt.Sprintf("SendOutputReport padded: %v", err))
		}
	}
	return fmt.Errorf("flydigi hid write failed (%s)", strings.Join(failures, "; "))
}

func (d *flyDigiHIDDevice) ReadReport(timeout time.Duration) ([]byte, error) {
	if d == nil || d.device == nil {
		return nil, fmt.Errorf("hid device is not open")
	}
	buf := make([]byte, hidLightReportLen-1)
	n, err := d.device.ReadWithTimeout(buf, timeout)
	if err != nil {
		if errors.Is(err, hid.ErrTimeout) {
			return nil, errFlyDigiHIDTimeout
		}
		return nil, err
	}
	return buf[:n], nil
}
