//go:build !windows

package deviceprofileexec

import (
	"fmt"

	"github.com/TIANLI0/THRM/internal/types"
)

func ListSerialPorts() ([]types.SerialPortInfo, error) {
	return nil, fmt.Errorf("serial COM port discovery is only implemented on Windows")
}
