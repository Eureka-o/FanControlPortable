package deviceprofileexec

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
	"tinygo.org/x/bluetooth"
)

const (
	defaultBLEScanTimeoutMs = 10000
	maxBLEScanTimeoutMs     = 15000
)

type BLEScanner interface {
	ScanBLEDevices(ctx context.Context, params types.BLEScanParams) ([]types.BLEDeviceInfo, error)
}

type BLEScannerFunc func(ctx context.Context, params types.BLEScanParams) ([]types.BLEDeviceInfo, error)

func (f BLEScannerFunc) ScanBLEDevices(ctx context.Context, params types.BLEScanParams) ([]types.BLEDeviceInfo, error) {
	return f(ctx, params)
}

type DefaultBLEScanner struct {
	coordinator *bleScanCoordinator
	scan        BLEScannerFunc
}

type bleScanCall struct {
	done    chan struct{}
	devices []types.BLEDeviceInfo
	err     error
}

type bleScanCoordinator struct {
	mutex    sync.Mutex
	inFlight map[string]*bleScanCall
}

var (
	defaultBLEAdapterLease    = make(chan struct{}, 1)
	defaultBLEScanCoordinator = newBLEScanCoordinator()
)

func newBLEScanCoordinator() *bleScanCoordinator {
	return &bleScanCoordinator{inFlight: make(map[string]*bleScanCall)}
}

func (c *bleScanCoordinator) scanDevices(
	ctx context.Context,
	params types.BLEScanParams,
	scan BLEScannerFunc,
) ([]types.BLEDeviceInfo, error) {
	keyBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	key := string(keyBytes)

	c.mutex.Lock()
	if call, ok := c.inFlight[key]; ok {
		c.mutex.Unlock()
		select {
		case <-call.done:
			return cloneBLEDeviceInfos(call.devices), call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &bleScanCall{done: make(chan struct{})}
	c.inFlight[key] = call
	c.mutex.Unlock()

	devices, scanErr := scan(ctx, params)
	c.mutex.Lock()
	call.devices = cloneBLEDeviceInfos(devices)
	call.err = scanErr
	delete(c.inFlight, key)
	close(call.done)
	c.mutex.Unlock()
	return cloneBLEDeviceInfos(devices), scanErr
}

func cloneBLEDeviceInfos(devices []types.BLEDeviceInfo) []types.BLEDeviceInfo {
	if devices == nil {
		return nil
	}
	cloned := make([]types.BLEDeviceInfo, len(devices))
	for i, device := range devices {
		cloned[i] = device
		cloned[i].ServiceUUIDs = append([]string(nil), device.ServiceUUIDs...)
		cloned[i].ManufacturerData = append([]types.BLEManufacturerData(nil), device.ManufacturerData...)
		cloned[i].WriteCharacteristicUUIDs = append([]string(nil), device.WriteCharacteristicUUIDs...)
		cloned[i].NotifyCharacteristicUUIDs = append([]string(nil), device.NotifyCharacteristicUUIDs...)
		cloned[i].MatchReasons = append([]string(nil), device.MatchReasons...)
	}
	return cloned
}

func acquireDefaultBLEAdapter(ctx context.Context) (func(), error) {
	ctx = ctxWithDefault(ctx)
	select {
	case defaultBLEAdapterLease <- struct{}{}:
		return func() { <-defaultBLEAdapterLease }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func ScanBLEDevices(params types.BLEScanParams) ([]types.BLEDeviceInfo, error) {
	return ScanBLEDevicesWithScanner(context.Background(), DefaultBLEScanner{}, params)
}

func ScanBLEDevicesWithScanner(ctx context.Context, scanner BLEScanner, params types.BLEScanParams) ([]types.BLEDeviceInfo, error) {
	if scanner == nil {
		return nil, errors.New("ble scanner is not configured")
	}
	params = normalizeBLEScanParams(params)
	scanCtx, cancel := context.WithTimeout(ctxWithDefault(ctx), time.Duration(params.TimeoutMs)*time.Millisecond)
	defer cancel()

	devices, err := scanner.ScanBLEDevices(scanCtx, params)
	if err != nil {
		return nil, err
	}
	return NormalizeAndMatchBLEDevices(devices, params), nil
}

func (s DefaultBLEScanner) ScanBLEDevices(ctx context.Context, params types.BLEScanParams) ([]types.BLEDeviceInfo, error) {
	ctx = ctxWithDefault(ctx)
	params = normalizeBLEScanParams(params)
	coordinator := s.coordinator
	if coordinator == nil {
		coordinator = defaultBLEScanCoordinator
	}
	scan := s.scan
	if scan == nil {
		scan = scanDefaultBLEDevices
	}
	return coordinator.scanDevices(ctx, params, scan)
}

func scanDefaultBLEDevices(ctx context.Context, params types.BLEScanParams) ([]types.BLEDeviceInfo, error) {
	release, err := acquireDefaultBLEAdapter(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return nil, err
	}

	return scanBLEAdvertisements(
		ctx,
		params,
		func(report func(types.BLEDeviceInfo)) error {
			return adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
				report(bleDeviceInfoFromScanResult(result))
			})
		},
		adapter.StopScan,
	)
}

func scanBLEAdvertisements(
	ctx context.Context,
	params types.BLEScanParams,
	scan func(report func(types.BLEDeviceInfo)) error,
	stop func() error,
) ([]types.BLEDeviceInfo, error) {
	var mutex sync.Mutex
	seen := make(map[string]types.BLEDeviceInfo)
	stopRequested := make(chan struct{})
	scanDone := make(chan struct{})
	var stopOnce sync.Once
	requestStop := func() {
		stopOnce.Do(func() {
			close(stopRequested)
			_ = stop()
		})
	}
	go func() {
		select {
		case <-ctx.Done():
			requestStop()
		case <-scanDone:
		}
	}()
	err := scan(func(info types.BLEDeviceInfo) {
		if strings.TrimSpace(info.Address) == "" {
			return
		}
		key := strings.ToUpper(info.Address)
		mutex.Lock()
		seen[key] = mergeBLEDeviceInfo(seen[key], info)
		matched := params.OnlyMatched && scoreBLEDevice(seen[key], params).Matched
		mutex.Unlock()
		if matched {
			requestStop()
		}
	})
	close(scanDone)
	if err != nil {
		select {
		case <-stopRequested:
			// Windows may return ERROR_INVALID_FUNCTION after an intentional StopScan.
		default:
			return nil, err
		}
	}

	mutex.Lock()
	defer mutex.Unlock()
	out := make([]types.BLEDeviceInfo, 0, len(seen))
	for _, info := range seen {
		out = append(out, info)
	}
	return out, nil
}

func NormalizeAndMatchBLEDevices(devices []types.BLEDeviceInfo, params types.BLEScanParams) []types.BLEDeviceInfo {
	params = normalizeBLEScanParams(params)
	out := make([]types.BLEDeviceInfo, 0, len(devices))
	seen := make(map[string]int, len(devices))
	for _, device := range devices {
		device = normalizeBLEDeviceInfo(device)
		if strings.TrimSpace(device.Address) == "" {
			continue
		}
		device = scoreBLEDevice(device, params)
		if params.OnlyMatched && !device.Matched {
			continue
		}
		key := strings.ToUpper(device.Address)
		if idx, ok := seen[key]; ok {
			out[idx] = mergeBLEDeviceInfo(out[idx], device)
			out[idx] = scoreBLEDevice(out[idx], params)
			continue
		}
		seen[key] = len(out)
		out = append(out, device)
	}
	sortBLEDeviceInfos(out)
	return out
}

func normalizeBLEScanParams(params types.BLEScanParams) types.BLEScanParams {
	params.TimeoutMs = clampBLEScanTimeout(params.TimeoutMs)
	params.NameFilter = strings.TrimSpace(params.NameFilter)
	params.ServiceUUID = normalizeBLEUUID(params.ServiceUUID)
	params.WriteCharacteristicUUID = normalizeBLEUUID(params.WriteCharacteristicUUID)
	params.NotifyCharacteristicUUID = normalizeBLEUUID(params.NotifyCharacteristicUUID)
	profiles := make([]types.DeviceProfile, 0, len(params.Profiles))
	for _, profile := range params.Profiles {
		profile = types.NormalizeDeviceProfile(profile, "")
		if profile.Transport == types.DeviceTransportBLE {
			profiles = append(profiles, profile)
		}
	}
	params.Profiles = profiles
	return params
}

func clampBLEScanTimeout(timeoutMs int) int {
	if timeoutMs <= 0 {
		return defaultBLEScanTimeoutMs
	}
	if timeoutMs > maxBLEScanTimeoutMs {
		return maxBLEScanTimeoutMs
	}
	return timeoutMs
}

func bleDeviceInfoFromScanResult(result bluetooth.ScanResult) types.BLEDeviceInfo {
	info := types.BLEDeviceInfo{
		Address: result.Address.String(),
		RSSI:    int(result.RSSI),
	}
	if result.AdvertisementPayload == nil {
		return info
	}
	info.Name = strings.TrimSpace(result.LocalName())
	for _, uuid := range result.ServiceUUIDs() {
		info.ServiceUUIDs = append(info.ServiceUUIDs, normalizeBLEUUID(uuid.String()))
	}
	for _, data := range result.ManufacturerData() {
		info.ManufacturerData = append(info.ManufacturerData, types.BLEManufacturerData{
			CompanyID: data.CompanyID,
			DataHex:   strings.ToUpper(hex.EncodeToString(data.Data)),
		})
	}
	return info
}

func normalizeBLEDeviceInfo(info types.BLEDeviceInfo) types.BLEDeviceInfo {
	info.Address = strings.TrimSpace(info.Address)
	info.Name = strings.TrimSpace(info.Name)
	info.ServiceUUIDs = normalizeBLEUUIDs(info.ServiceUUIDs)
	info.WriteCharacteristicUUIDs = normalizeBLEUUIDs(info.WriteCharacteristicUUIDs)
	info.NotifyCharacteristicUUIDs = normalizeBLEUUIDs(info.NotifyCharacteristicUUIDs)
	for i := range info.ManufacturerData {
		info.ManufacturerData[i].DataHex = strings.ToUpper(strings.TrimSpace(info.ManufacturerData[i].DataHex))
	}
	if info.SuggestedNameFilter == "" && info.Name != "" {
		info.SuggestedNameFilter = info.Name
	}
	if info.SuggestedServiceUUID == "" && len(info.ServiceUUIDs) > 0 {
		info.SuggestedServiceUUID = info.ServiceUUIDs[0]
	}
	if info.SuggestedWriteCharacteristic == "" && len(info.WriteCharacteristicUUIDs) > 0 {
		info.SuggestedWriteCharacteristic = info.WriteCharacteristicUUIDs[0]
	}
	if info.SuggestedNotifyCharacteristic == "" && len(info.NotifyCharacteristicUUIDs) > 0 {
		info.SuggestedNotifyCharacteristic = info.NotifyCharacteristicUUIDs[0]
	}
	return info
}

func mergeBLEDeviceInfo(left, right types.BLEDeviceInfo) types.BLEDeviceInfo {
	if strings.TrimSpace(left.Address) == "" {
		return normalizeBLEDeviceInfo(right)
	}
	left = normalizeBLEDeviceInfo(left)
	right = normalizeBLEDeviceInfo(right)
	if right.Name != "" {
		left.Name = right.Name
	}
	if right.RSSI != 0 && (left.RSSI == 0 || right.RSSI > left.RSSI) {
		left.RSSI = right.RSSI
	}
	left.ServiceUUIDs = mergeStringSet(left.ServiceUUIDs, right.ServiceUUIDs)
	left.WriteCharacteristicUUIDs = mergeStringSet(left.WriteCharacteristicUUIDs, right.WriteCharacteristicUUIDs)
	left.NotifyCharacteristicUUIDs = mergeStringSet(left.NotifyCharacteristicUUIDs, right.NotifyCharacteristicUUIDs)
	left.ManufacturerData = mergeBLEManufacturerData(left.ManufacturerData, right.ManufacturerData)
	return normalizeBLEDeviceInfo(left)
}

func scoreBLEDevice(device types.BLEDeviceInfo, params types.BLEScanParams) types.BLEDeviceInfo {
	device.MatchScore = 0
	device.MatchReasons = nil
	device.Matched = false
	device.MatchedProfileID = ""
	device.MatchedProfileDisplayName = ""

	bestScore, bestReasons := scoreBLEDeviceForFilters(device, params.NameFilter, params.ServiceUUID, params.WriteCharacteristicUUID, params.NotifyCharacteristicUUID)
	bestID, bestName := "", ""
	for _, profile := range params.Profiles {
		score, reasons, profileID, profileName := scoreBLEDeviceForProfile(device, profile)
		if score > bestScore {
			bestScore, bestReasons, bestID, bestName = score, reasons, profileID, profileName
		}
	}

	device.MatchScore = bestScore
	device.MatchReasons = bestReasons
	device.MatchedProfileID = bestID
	device.MatchedProfileDisplayName = bestName
	device.Matched = bestScore > 0
	return device
}

func scoreBLEDeviceForProfile(device types.BLEDeviceInfo, profile types.DeviceProfile) (int, []string, string, string) {
	conn := profile.Connection
	score, reasons := scoreBLEDeviceForFilters(
		device,
		conn.BLENameFilter,
		conn.BLEServiceUUID,
		conn.BLEWriteCharacteristic,
		conn.BLENotifyCharacteristic,
	)
	if score == 0 {
		return 0, nil, "", ""
	}
	return score, reasons, profile.ID, profile.DisplayName
}

func scoreBLEDeviceForFilters(device types.BLEDeviceInfo, nameFilter, serviceUUID, writeUUID, notifyUUID string) (int, []string) {
	score := 0
	reasons := make([]string, 0, 4)
	nameFilter = strings.TrimSpace(nameFilter)
	serviceUUID = normalizeBLEUUID(serviceUUID)
	writeUUID = normalizeBLEUUID(writeUUID)
	notifyUUID = normalizeBLEUUID(notifyUUID)

	if nameFilter != "" && containsFold(device.Name, nameFilter) {
		score += 40
		reasons = append(reasons, "name")
	}
	if serviceUUID != "" && uuidInSet(device.ServiceUUIDs, serviceUUID) {
		score += 50
		reasons = append(reasons, "service")
	}
	if writeUUID != "" && uuidInSet(device.WriteCharacteristicUUIDs, writeUUID) {
		score += 30
		reasons = append(reasons, "write")
	}
	if notifyUUID != "" && uuidInSet(device.NotifyCharacteristicUUIDs, notifyUUID) {
		score += 20
		reasons = append(reasons, "notify")
	}
	return score, reasons
}

func sortBLEDeviceInfos(devices []types.BLEDeviceInfo) {
	sort.SliceStable(devices, func(i, j int) bool {
		if devices[i].Matched != devices[j].Matched {
			return devices[i].Matched
		}
		if devices[i].MatchScore != devices[j].MatchScore {
			return devices[i].MatchScore > devices[j].MatchScore
		}
		if devices[i].RSSI != devices[j].RSSI {
			return devices[i].RSSI > devices[j].RSSI
		}
		leftName := strings.ToUpper(devices[i].Name)
		rightName := strings.ToUpper(devices[j].Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return strings.ToUpper(devices[i].Address) < strings.ToUpper(devices[j].Address)
	})
}

func normalizeBLEUUIDs(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = normalizeBLEUUID(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeBLEUUID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func uuidInSet(values []string, target string) bool {
	target = normalizeBLEUUID(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if bleUUIDMatches(value, target) {
			return true
		}
	}
	return false
}

func bleUUIDMatches(value, target string) bool {
	value = strings.ReplaceAll(normalizeBLEUUID(value), "-", "")
	target = strings.ReplaceAll(normalizeBLEUUID(target), "-", "")
	if value == "" || target == "" {
		return false
	}
	if value == target {
		return true
	}
	const bluetoothBaseSuffix = "00001000800000805f9b34fb"
	if len(value) == 32 && len(target) == 4 && strings.HasPrefix(value, "0000"+target) && strings.HasSuffix(value, bluetoothBaseSuffix) {
		return true
	}
	if len(target) == 32 && len(value) == 4 && strings.HasPrefix(target, "0000"+value) && strings.HasSuffix(target, bluetoothBaseSuffix) {
		return true
	}
	return false
}

func containsFold(haystack, needle string) bool {
	haystack = strings.ToLower(strings.TrimSpace(haystack))
	needle = strings.ToLower(strings.TrimSpace(needle))
	return haystack != "" && needle != "" && strings.Contains(haystack, needle)
}

func mergeStringSet(left, right []string) []string {
	out := make([]string, 0, len(left)+len(right))
	seen := make(map[string]bool, len(left)+len(right))
	for _, value := range append(left, right...) {
		value = normalizeBLEUUID(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func mergeBLEManufacturerData(left, right []types.BLEManufacturerData) []types.BLEManufacturerData {
	out := make([]types.BLEManufacturerData, 0, len(left)+len(right))
	seen := make(map[string]bool, len(left)+len(right))
	for _, item := range append(left, right...) {
		item.DataHex = strings.ToUpper(strings.TrimSpace(item.DataHex))
		key := strconv.Itoa(int(item.CompanyID)) + ":" + item.DataHex
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}
