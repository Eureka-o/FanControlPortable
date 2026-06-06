package deviceprofileexec

import (
	"sort"
	"strconv"
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

func normalizeSerialPortInfos(ports []types.SerialPortInfo) []types.SerialPortInfo {
	seen := make(map[string]bool, len(ports))
	out := make([]types.SerialPortInfo, 0, len(ports))
	for _, port := range ports {
		port.Name = strings.TrimSpace(port.Name)
		port.Path = strings.TrimSpace(port.Path)
		port.DisplayName = strings.TrimSpace(port.DisplayName)
		port.Source = strings.TrimSpace(port.Source)
		if port.Name == "" {
			continue
		}
		key := strings.ToUpper(port.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		if port.DisplayName == "" {
			if port.Path != "" {
				port.DisplayName = port.Name + " (" + port.Path + ")"
			} else {
				port.DisplayName = port.Name
			}
		}
		out = append(out, port)
	}
	sort.SliceStable(out, func(i, j int) bool {
		leftNumber, leftOK := serialCOMNumber(out[i].Name)
		rightNumber, rightOK := serialCOMNumber(out[j].Name)
		if leftOK && rightOK {
			return leftNumber < rightNumber
		}
		if leftOK != rightOK {
			return leftOK
		}
		return strings.ToUpper(out[i].Name) < strings.ToUpper(out[j].Name)
	})
	return out
}

func serialCOMNumber(name string) (int, bool) {
	name = strings.ToUpper(strings.TrimSpace(name))
	if !strings.HasPrefix(name, "COM") {
		return 0, false
	}
	numberText := strings.TrimPrefix(name, "COM")
	if numberText == "" {
		return 0, false
	}
	number, err := strconv.Atoi(numberText)
	if err != nil || number < 0 {
		return 0, false
	}
	return number, true
}
