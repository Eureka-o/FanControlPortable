//go:build !windows

package deviceprofileexec

import (
	"fmt"

	"github.com/TIANLI0/THRM/internal/types"
)

type DefaultSerialDialer struct{}

func (DefaultSerialDialer) OpenSerialPort(profile types.DeviceProfile) (SerialPort, error) {
	return nil, fmt.Errorf("serial COM transport is only implemented on Windows")
}
