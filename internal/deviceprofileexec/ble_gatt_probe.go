package deviceprofileexec

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
	"tinygo.org/x/bluetooth"
)

const (
	bleGATTPropertyBroadcast            uint32 = 0x01
	bleGATTPropertyRead                 uint32 = 0x02
	bleGATTPropertyWriteWithoutResponse uint32 = 0x04
	bleGATTPropertyWrite                uint32 = 0x08
	bleGATTPropertyNotify               uint32 = 0x10
	bleGATTPropertyIndicate             uint32 = 0x20
	bleGATTPropertyAuthenticatedWrite   uint32 = 0x40
	bleGATTPropertyExtended             uint32 = 0x80
)

type BLEGATTProber interface {
	ProbeBLEGATT(ctx context.Context, params types.BLEGATTProbeParams) (*types.BLEGATTProbeResult, error)
}

type BLEGATTProberFunc func(ctx context.Context, params types.BLEGATTProbeParams) (*types.BLEGATTProbeResult, error)

func (f BLEGATTProberFunc) ProbeBLEGATT(ctx context.Context, params types.BLEGATTProbeParams) (*types.BLEGATTProbeResult, error) {
	return f(ctx, params)
}

type DefaultBLEGATTProber struct {
	Scanner BLEScanner
}

func ProbeBLEGATT(params types.BLEGATTProbeParams) (*types.BLEGATTProbeResult, error) {
	return ProbeBLEGATTWithProber(context.Background(), DefaultBLEGATTProber{}, params)
}

func ProbeBLEGATTWithProber(ctx context.Context, prober BLEGATTProber, params types.BLEGATTProbeParams) (*types.BLEGATTProbeResult, error) {
	if prober == nil {
		return nil, errors.New("ble gatt prober is not configured")
	}
	params = normalizeBLEGATTProbeParams(params)
	probeCtx, cancel := context.WithTimeout(ctxWithDefault(ctx), time.Duration(params.TimeoutMs)*time.Millisecond)
	defer cancel()

	result, err := prober.ProbeBLEGATT(probeCtx, params)
	if err != nil {
		return nil, err
	}
	return NormalizeBLEGATTProbeResult(result, params), nil
}

func (p DefaultBLEGATTProber) ProbeBLEGATT(ctx context.Context, params types.BLEGATTProbeParams) (*types.BLEGATTProbeResult, error) {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return nil, err
	}

	address, deviceInfo, err := p.resolveProbeTarget(ctx, params)
	if err != nil {
		return nil, err
	}
	device, err := adapter.Connect(address, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, err
	}
	defer device.Disconnect()

	serviceUUID := params.ServiceUUID
	if serviceUUID == "" {
		serviceUUID = params.Profile.Connection.BLEServiceUUID
	}
	services, err := discoverServicesForProfile(device, serviceUUID)
	if err != nil {
		return nil, err
	}

	result := &types.BLEGATTProbeResult{
		Address: deviceInfo.Address,
		Name:    deviceInfo.Name,
	}
	if result.Address == "" {
		result.Address = params.Address
	}

	for _, service := range services {
		serviceInfo := types.BLEGATTServiceInfo{
			UUID: normalizeBLEUUID(service.UUID().String()),
		}
		chars, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			serviceInfo.Error = err.Error()
			result.Services = append(result.Services, serviceInfo)
			continue
		}
		for i := range chars {
			serviceInfo.Characteristics = append(serviceInfo.Characteristics, bleGATTCharacteristicInfo(chars[i]))
		}
		result.Services = append(result.Services, serviceInfo)
	}
	return result, nil
}

func (p DefaultBLEGATTProber) resolveProbeTarget(ctx context.Context, params types.BLEGATTProbeParams) (bluetooth.Address, types.BLEDeviceInfo, error) {
	addressText := strings.TrimSpace(params.Address)
	if addressText == "" {
		addressText = strings.TrimSpace(params.Profile.Connection.Endpoint)
	}
	if addressText != "" {
		address, err := parseBluetoothAddress(addressText)
		return address, types.BLEDeviceInfo{Address: addressText}, err
	}

	scanner := p.Scanner
	if scanner == nil {
		scanner = DefaultBLEScanner{}
	}
	profile := params.Profile
	devices, err := ScanBLEDevicesWithScanner(ctxWithDefault(ctx), scanner, types.BLEScanParams{
		TimeoutMs:                params.TimeoutMs,
		NameFilter:               profile.Connection.BLENameFilter,
		ServiceUUID:              firstNonEmpty(params.ServiceUUID, profile.Connection.BLEServiceUUID),
		WriteCharacteristicUUID:  profile.Connection.BLEWriteCharacteristic,
		NotifyCharacteristicUUID: profile.Connection.BLENotifyCharacteristic,
		OnlyMatched:              true,
		Profiles:                 []types.DeviceProfile{profile},
	})
	if err != nil {
		return bluetooth.Address{}, types.BLEDeviceInfo{}, err
	}
	if len(devices) == 0 {
		return bluetooth.Address{}, types.BLEDeviceInfo{}, fmt.Errorf("no BLE device matched profile %q for GATT probing", profile.DisplayName)
	}
	address, err := parseBluetoothAddress(devices[0].Address)
	return address, devices[0], err
}

func normalizeBLEGATTProbeParams(params types.BLEGATTProbeParams) types.BLEGATTProbeParams {
	params.TimeoutMs = clampBLEScanTimeout(params.TimeoutMs)
	params.Address = strings.TrimSpace(params.Address)
	params.ServiceUUID = normalizeBLEUUID(params.ServiceUUID)
	params.Profile = types.NormalizeDeviceProfile(params.Profile, "")
	if params.Profile.Transport != types.DeviceTransportBLE {
		params.Profile.Transport = types.DeviceTransportBLE
	}
	params.Profile.Connection.BLEServiceUUID = normalizeBLEUUID(params.Profile.Connection.BLEServiceUUID)
	params.Profile.Connection.BLEWriteCharacteristic = normalizeBLEUUID(params.Profile.Connection.BLEWriteCharacteristic)
	params.Profile.Connection.BLENotifyCharacteristic = normalizeBLEUUID(params.Profile.Connection.BLENotifyCharacteristic)
	return params
}

func NormalizeBLEGATTProbeResult(result *types.BLEGATTProbeResult, params types.BLEGATTProbeParams) *types.BLEGATTProbeResult {
	if result == nil {
		result = &types.BLEGATTProbeResult{}
	}
	params = normalizeBLEGATTProbeParams(params)
	result.Address = strings.TrimSpace(result.Address)
	result.Name = strings.TrimSpace(result.Name)
	for serviceIndex := range result.Services {
		service := &result.Services[serviceIndex]
		service.UUID = normalizeBLEUUID(service.UUID)
		service.Error = strings.TrimSpace(service.Error)
		for charIndex := range service.Characteristics {
			characteristic := &service.Characteristics[charIndex]
			characteristic.UUID = normalizeBLEUUID(characteristic.UUID)
			characteristic.Properties = normalizeBLEGATTProperties(characteristic.Properties)
			applyBLEGATTPropertyBools(characteristic)
		}
		sort.SliceStable(service.Characteristics, func(i, j int) bool {
			return service.Characteristics[i].UUID < service.Characteristics[j].UUID
		})
	}
	sort.SliceStable(result.Services, func(i, j int) bool {
		return result.Services[i].UUID < result.Services[j].UUID
	})

	result.SuggestedServiceUUID, result.SuggestedWriteCharacteristic, result.SuggestedNotifyCharacteristic = suggestBLEGATTFields(result.Services, params)
	return result
}

func bleGATTCharacteristicInfo(characteristic bluetooth.DeviceCharacteristic) types.BLEGATTCharacteristicInfo {
	info := types.BLEGATTCharacteristicInfo{
		UUID: normalizeBLEUUID(characteristic.UUID().String()),
	}
	if properties, ok := any(characteristic).(interface{ Properties() uint32 }); ok {
		applyBLEGATTProperties(&info, properties.Properties())
	}
	if mtu, err := characteristic.GetMTU(); err == nil && mtu > 0 {
		info.MTU = int(mtu)
	}
	return info
}

func applyBLEGATTProperties(info *types.BLEGATTCharacteristicInfo, flags uint32) {
	if flags&bleGATTPropertyBroadcast != 0 {
		info.Properties = append(info.Properties, "broadcast")
	}
	if flags&bleGATTPropertyRead != 0 {
		info.Properties = append(info.Properties, "read")
		info.CanRead = true
	}
	if flags&bleGATTPropertyWriteWithoutResponse != 0 {
		info.Properties = append(info.Properties, "writeWithoutResponse")
		info.CanWriteWithoutResponse = true
	}
	if flags&bleGATTPropertyWrite != 0 {
		info.Properties = append(info.Properties, "write")
		info.CanWrite = true
	}
	if flags&bleGATTPropertyNotify != 0 {
		info.Properties = append(info.Properties, "notify")
		info.CanNotify = true
	}
	if flags&bleGATTPropertyIndicate != 0 {
		info.Properties = append(info.Properties, "indicate")
		info.CanIndicate = true
	}
	if flags&bleGATTPropertyAuthenticatedWrite != 0 {
		info.Properties = append(info.Properties, "authenticatedWrite")
	}
	if flags&bleGATTPropertyExtended != 0 {
		info.Properties = append(info.Properties, "extended")
	}
	info.Properties = normalizeBLEGATTProperties(info.Properties)
}

func applyBLEGATTPropertyBools(info *types.BLEGATTCharacteristicInfo) {
	for _, property := range info.Properties {
		switch property {
		case "read":
			info.CanRead = true
		case "write":
			info.CanWrite = true
		case "writeWithoutResponse":
			info.CanWriteWithoutResponse = true
		case "notify":
			info.CanNotify = true
		case "indicate":
			info.CanIndicate = true
		}
	}
}

func normalizeBLEGATTProperties(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func suggestBLEGATTFields(services []types.BLEGATTServiceInfo, params types.BLEGATTProbeParams) (string, string, string) {
	preferredService := firstNonEmpty(params.ServiceUUID, params.Profile.Connection.BLEServiceUUID)
	preferredWrite := params.Profile.Connection.BLEWriteCharacteristic
	preferredNotify := params.Profile.Connection.BLENotifyCharacteristic

	serviceSuggestion := ""
	writeSuggestion := ""
	notifySuggestion := ""
	for _, service := range services {
		if service.UUID == "" {
			continue
		}
		if serviceSuggestion == "" {
			serviceSuggestion = service.UUID
		}
		if preferredService != "" && bleUUIDMatches(service.UUID, preferredService) {
			serviceSuggestion = service.UUID
		}
		for _, characteristic := range service.Characteristics {
			if characteristic.UUID == "" {
				continue
			}
			if writeSuggestion == "" && preferredWrite != "" && bleUUIDMatches(characteristic.UUID, preferredWrite) {
				writeSuggestion = characteristic.UUID
			}
			if notifySuggestion == "" && preferredNotify != "" && bleUUIDMatches(characteristic.UUID, preferredNotify) {
				notifySuggestion = characteristic.UUID
			}
			if writeSuggestion == "" && (characteristic.CanWrite || characteristic.CanWriteWithoutResponse) {
				writeSuggestion = characteristic.UUID
				if serviceSuggestion == "" || preferredService == "" {
					serviceSuggestion = service.UUID
				}
			}
			if notifySuggestion == "" && (characteristic.CanNotify || characteristic.CanIndicate) {
				notifySuggestion = characteristic.UUID
				if serviceSuggestion == "" || preferredService == "" {
					serviceSuggestion = service.UUID
				}
			}
		}
	}
	if notifySuggestion == "" {
		for _, service := range services {
			for _, characteristic := range service.Characteristics {
				if characteristic.UUID != "" && characteristic.CanRead {
					notifySuggestion = characteristic.UUID
					if serviceSuggestion == "" || preferredService == "" {
						serviceSuggestion = service.UUID
					}
					break
				}
			}
			if notifySuggestion != "" {
				break
			}
		}
	}
	return serviceSuggestion, writeSuggestion, notifySuggestion
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
