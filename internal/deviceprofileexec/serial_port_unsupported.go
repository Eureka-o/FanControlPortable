//go:build !windows

package deviceprofileexec

import (
	"fmt"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

type DefaultSerialDialer struct{}

func (DefaultSerialDialer) OpenSerialPort(profile types.DeviceProfile) (SerialPort, error) {
	return nil, fmt.Errorf("serial COM transport is only implemented on Windows")
}
