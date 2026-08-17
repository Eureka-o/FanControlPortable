//go:build !windows

package deviceprofileexec

import (
	"fmt"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func ListSerialPorts() ([]types.SerialPortInfo, error) {
	return nil, fmt.Errorf("serial COM port discovery is only implemented on Windows")
}
