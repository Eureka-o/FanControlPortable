package guiapp

import "github.com/Eureka-o/FanControlPortable/internal/deviceprofileexec"

func (a *App) ListSerialPorts() []SerialPortInfo {
	ports, err := deviceprofileexec.ListSerialPorts()
	if err != nil {
		guiLogger.Warnf("list serial ports failed: %v", err)
		return []SerialPortInfo{}
	}
	return ports
}
