//go:build windows

package deviceprofileexec

import (
	"github.com/TIANLI0/THRM/internal/types"
	"golang.org/x/sys/windows/registry"
)

const serialCommRegistryPath = `HARDWARE\DEVICEMAP\SERIALCOMM`

func ListSerialPorts() ([]types.SerialPortInfo, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, serialCommRegistryPath, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer key.Close()

	valueNames, err := key.ReadValueNames(0)
	if err != nil {
		return nil, err
	}

	ports := make([]types.SerialPortInfo, 0, len(valueNames))
	for _, valueName := range valueNames {
		portName, _, err := key.GetStringValue(valueName)
		if err != nil {
			continue
		}
		ports = append(ports, types.SerialPortInfo{
			Name:   portName,
			Path:   valueName,
			Source: "registry",
		})
	}
	return normalizeSerialPortInfos(ports), nil
}
