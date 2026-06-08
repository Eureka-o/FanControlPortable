//go:build !legacydevice

package device

import (
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	flyDigiLightSpeedFast   byte = 0x05
	flyDigiLightSpeedMedium byte = 0x0A
	flyDigiLightSpeedSlow   byte = 0x0F
)

func (m *Manager) setFlyDigiHIDLightStrip(cfg types.LightStripConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.flyDigiHID == nil {
		return fmt.Errorf("飞智 HID 设备未连接")
	}

	brightness := clampFlyDigiLightBrightness(cfg.Brightness)
	speed := parseFlyDigiLightSpeed(cfg.Speed)

	switch cfg.Mode {
	case "off":
		return m.setFlyDigiHIDRGBOffLocked()
	case "smart_temp":
		return m.setFlyDigiHIDLightSmartTempLocked()
	case "static_single":
		color := firstOrDefaultFlyDigiColor(cfg.Colors)
		return m.setFlyDigiHIDLightStaticSingleLocked(color, brightness)
	case "static_multi":
		colors := toThreeFlyDigiColors(cfg.Colors)
		return m.setFlyDigiHIDLightStaticMultiLocked(colors, brightness)
	case "rotation":
		colors := ensureMinFlyDigiColors(cfg.Colors, 1)
		return m.setFlyDigiHIDLightRotationLocked(colors, speed, brightness)
	case "flowing":
		return m.setFlyDigiHIDLightFlowingLocked(speed, brightness)
	case "breathing":
		colors := ensureMinFlyDigiColors(cfg.Colors, 1)
		return m.setFlyDigiHIDLightBreathingLocked(colors, speed, brightness)
	default:
		return fmt.Errorf("未知灯带模式: %s", cfg.Mode)
	}
}

func (m *Manager) setFlyDigiHIDRGBOff() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isConnected || m.flyDigiHID == nil {
		return false
	}
	return m.setFlyDigiHIDRGBOffLocked() == nil
}

func (m *Manager) setFlyDigiHIDRGBOffLocked() error {
	return m.sendFlyDigiLightCommandLocked(0x46, 0x03, 0x00)
}

func clampFlyDigiLightBrightness(value int) byte {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return byte(value)
}

func parseFlyDigiLightSpeed(speed string) byte {
	switch speed {
	case "fast":
		return flyDigiLightSpeedFast
	case "slow":
		return flyDigiLightSpeedSlow
	default:
		return flyDigiLightSpeedMedium
	}
}

func firstOrDefaultFlyDigiColor(colors []types.RGBColor) types.RGBColor {
	if len(colors) == 0 {
		return types.RGBColor{R: 255, G: 255, B: 255}
	}
	return colors[0]
}

func toThreeFlyDigiColors(colors []types.RGBColor) [3]types.RGBColor {
	base := [3]types.RGBColor{
		{R: 255, G: 0, B: 0},
		{R: 0, G: 255, B: 0},
		{R: 0, G: 128, B: 255},
	}
	for i := 0; i < len(base) && i < len(colors); i++ {
		base[i] = colors[i]
	}
	return base
}

func ensureMinFlyDigiColors(colors []types.RGBColor, min int) []types.RGBColor {
	if len(colors) >= min {
		return colors
	}
	defaults := []types.RGBColor{{R: 255, G: 0, B: 0}, {R: 0, G: 255, B: 0}, {R: 0, G: 128, B: 255}}
	result := make([]types.RGBColor, 0, min)
	result = append(result, colors...)
	for len(result) < min {
		result = append(result, defaults[len(result)%len(defaults)])
	}
	return result
}

func (m *Manager) sendFlyDigiLightCommandLocked(fields ...byte) error {
	if len(fields) < 2 {
		return fmt.Errorf("invalid light command")
	}
	frame := deviceproto.BuildFrame(fields[0], fields[2:]...)
	buf := m.lightCmdBuf[:]
	for i := range buf {
		buf[i] = 0
	}
	copy(buf, deviceproto.BuildReport(frame, hidLightReportLen))
	m.recordDebugFrame("tx", types.DeviceTransportHID, buf)
	return m.flyDigiHID.WriteReport(buf, 1200*time.Millisecond)
}

func makeFlyDigiLightF0(mode, speed, brightness byte, baseColor types.RGBColor) [10]byte {
	return [10]byte{0x00, 0x02, 0x00, mode, speed, brightness, baseColor.R, baseColor.G, baseColor.B, 0x00}
}

func (m *Manager) applyFlyDigiLightFramesLocked(f0 [10]byte, frames [30][10]byte) error {
	if err := m.sendFlyDigiLightCommandLocked(0x46, 0x03, 0x00); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)

	handshakes := [][]byte{
		{0x46, 0x03, 0x01}, {0x46, 0x03, 0x01}, {0x45, 0x02},
		{0x45, 0x03, 0x01}, {0x41, 0x02}, {0x41, 0x03, 0x01},
	}
	for _, cmd := range handshakes {
		if err := m.sendFlyDigiLightCommandLocked(cmd...); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}

	f0Payload := append([]byte{0x47, 0x0D, 0x00}, f0[:]...)
	if err := m.sendFlyDigiLightCommandLocked(f0Payload...); err != nil {
		return err
	}
	for i := range 30 {
		framePayload := append([]byte{0x47, 0x0D, byte(i + 1)}, frames[i][:]...)
		if err := m.sendFlyDigiLightCommandLocked(framePayload...); err != nil {
			return err
		}
		time.Sleep(1 * time.Millisecond)
	}
	return m.sendFlyDigiLightCommandLocked(0x43, 0x03, 0x01)
}

func (m *Manager) setFlyDigiHIDLightStaticSingleLocked(color types.RGBColor, brightness byte) error {
	f0 := makeFlyDigiLightF0(0x00, flyDigiLightSpeedMedium, brightness, color)
	var frames [30][10]byte
	factor := float64(brightness) / 100.0
	r := byte(float64(color.R) * factor)
	g := byte(float64(color.G) * factor)
	b := byte(float64(color.B) * factor)
	for _, idx := range []int{2, 5, 8, 11, 14} {
		frames[idx][6], frames[idx][7], frames[idx][8] = r, g, b
	}
	return m.applyFlyDigiLightFramesLocked(f0, frames)
}

func (m *Manager) setFlyDigiHIDLightStaticMultiLocked(colors [3]types.RGBColor, brightness byte) error {
	f0 := makeFlyDigiLightF0(0x00, flyDigiLightSpeedMedium, brightness, colors[0])
	var frames [30][10]byte
	factor := float64(brightness) / 100.0
	for z, idx := range []int{2, 5, 8, 11, 14} {
		col := colors[(z+1)%3]
		frames[idx][6] = byte(float64(col.R) * factor)
		frames[idx][7] = byte(float64(col.G) * factor)
		frames[idx][8] = byte(float64(col.B) * factor)
	}
	return m.applyFlyDigiLightFramesLocked(f0, frames)
}

func (m *Manager) setFlyDigiHIDLightRotationLocked(colors []types.RGBColor, speed, brightness byte) error {
	if len(colors) < 1 {
		return fmt.Errorf("旋转需要至少 1 个颜色")
	}
	if len(colors) > 6 {
		colors = colors[:6]
	}
	f0 := makeFlyDigiLightF0(0x05, speed, brightness, types.RGBColor{R: 0, G: 0, B: 0})
	var frames [30][10]byte
	stream := make([]byte, 304)
	numColors := len(colors)
	factor := float64(brightness) / 100.0
	for chunkIdx := range 6 {
		chunkStart := chunkIdx * 30
		for p := range 10 {
			var r, g, b byte
			if p < 6 {
				colorIdx := (p + chunkIdx) % 6
				if colorIdx < numColors {
					target := colors[colorIdx]
					r = byte(float64(target.R) * factor)
					g = byte(float64(target.G) * factor)
					b = byte(float64(target.B) * factor)
				}
			}
			stream[chunkStart+p*3] = r
			stream[chunkStart+p*3+1] = g
			stream[chunkStart+p*3+2] = b
		}
	}
	for k := range 304 {
		if k < 4 {
			f0[6+k] = stream[k]
		} else {
			idx := k - 4
			frames[idx/10][idx%10] = stream[k]
		}
	}
	return m.applyFlyDigiLightFramesLocked(f0, frames)
}

func (m *Manager) setFlyDigiHIDLightFlowingLocked(speed, brightness byte) error {
	flowingBase := [9][10]byte{
		{0x7f, 0x7f, 0x00, 0xff, 0x00, 0x7f, 0x7f, 0x00, 0xff, 0x00},
		{0x00, 0x7f, 0x00, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x7f, 0x7f, 0x00},
		{0x00, 0x00, 0x00, 0xff, 0x00, 0x7f, 0x7f, 0x00, 0xff, 0x00},
		{0x7f, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x00, 0x7f},
		{0x7f, 0x00, 0xff, 0x00, 0x00, 0x7f, 0x00, 0x7f, 0x00, 0x00},
		{0xff, 0x00, 0x7f, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x00},
	}
	f0 := makeFlyDigiLightF0(0x05, speed, brightness, types.RGBColor{R: 0, G: 255, B: 0})
	factor := float64(brightness) / 100.0
	var frames [30][10]byte
	for i := range 30 {
		src := flowingBase[i%9]
		for j := range 9 {
			frames[i][j] = byte(float64(src[j]) * factor)
		}
		frames[i][9] = src[9]
	}
	return m.applyFlyDigiLightFramesLocked(f0, frames)
}

func (m *Manager) setFlyDigiHIDLightBreathingLocked(colors []types.RGBColor, speed, brightness byte) error {
	if len(colors) == 0 {
		return fmt.Errorf("颜色列表不能为空")
	}
	if len(colors) > 5 {
		colors = colors[:5]
	}
	mode := byte(len(colors)*2 - 1)
	f0 := makeFlyDigiLightF0(mode, speed, brightness, types.RGBColor{R: 0, G: 0, B: 0})
	var frames [30][10]byte
	factor := float64(brightness) / 100.0
	var pattern [30]byte
	for i, col := range colors {
		offset := i * 6
		if offset+2 >= len(pattern) {
			break
		}
		pattern[offset] = byte(float64(col.R) * factor)
		pattern[offset+1] = byte(float64(col.G) * factor)
		pattern[offset+2] = byte(float64(col.B) * factor)
	}
	for k := range 304 {
		val := pattern[k%30]
		if k < 4 {
			f0[6+k] = val
		} else {
			idx := k - 4
			frames[idx/10][idx%10] = val
		}
	}
	return m.applyFlyDigiLightFramesLocked(f0, frames)
}

func (m *Manager) setFlyDigiHIDLightSmartTempLocked() error {
	handshakes := [][]byte{
		{0x46, 0x03, 0x01}, {0x46, 0x03, 0x01}, {0x45, 0x02}, {0x45, 0x03, 0x01},
	}
	for _, cmd := range handshakes {
		if err := m.sendFlyDigiLightCommandLocked(cmd...); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := m.sendFlyDigiLightCommandLocked(0x44, 0x03, 0x01); err != nil {
		return err
	}
	time.Sleep(5 * time.Millisecond)
	return m.sendFlyDigiLightCommandLocked(0x43, 0x03, 0x01)
}
