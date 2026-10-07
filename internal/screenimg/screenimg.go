// 屏保图像：内置精选、裁剪与 RGB565 编码、`0xC5` 回读信息的解码。
//
// 上行分片与流控等待不在本包：图传已交给设备层的传输实现（`device.SendBlackSharkImage`，
// 含 7ms/报 + 跨 4 KiB 页补 40ms 的节拍）。这里只负责"把一张图变成设备要的那 121552 字节"，
// 以及读回来之后的解码与认图。
package screenimg

import (
	"bytes"
	"embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"math"
	"sort"
	"sync"
)

const (
	Width         = 428
	Height        = 142
	BytesPerPixel = 2
	// PayloadSize = 428 * 142 * 2；官方 .bin 素材恒为该长度。
	PayloadSize = Width * Height * BytesPerPixel // 121552

	// InfoPayloadSize 是 0xC4 / 0xC5 的载荷长度。
	InfoPayloadSize = 11
)

// Info 是 0xC4 / 0xC5 那 11 字节载荷的结构化视图。
type Info struct {
	// Timestamp 是 [0..3] 的 u32 小端值：Unix 时间戳（秒）。
	// 与官方落盘的临时文件名同源（官方用同一秒做文件名）。
	Timestamp uint32
	// Size 是 [4..7] 的 u32 小端值：图像字节数，恒为 121552（= 428*142*2）。
	Size uint32
	// CRC 是 [8..9] 的 u16 小端值：对图像字节的 CRC-16/XMODEM。
	CRC uint16
}

// Crc16Xmodem 计算 CRC-16/XMODEM（poly 0x1021，init 0x0000，不反转，不异或输出）。
// 十个真实样本算出的 CRC 与设备回读值全部吻合。
func Crc16Xmodem(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// ParseInfo 解析设备回读回来的信息载荷，用于上传后校验。
//
// 这里刻意不校验任何"魔数" —— 因为压根没有魔数（见 Info 的说明）。
func ParseInfo(payload []byte) (Info, error) {
	if len(payload) < InfoPayloadSize {
		return Info{}, fmt.Errorf("screen image info payload is %d bytes, want %d",
			len(payload), InfoPayloadSize)
	}
	return Info{
		Timestamp: binary.LittleEndian.Uint32(payload[0:4]),
		Size:      binary.LittleEndian.Uint32(payload[4:8]),
		CRC:       binary.LittleEndian.Uint16(payload[8:10]),
	}, nil
}

// Crop 描述"从原图里取哪一块送到屏上"。
type Crop struct {
	// Zoom 是相对"填满(cover)"的额外放大倍数：1 = 刚好填满。
	// 不允许小于 1：cover 已经是"两个方向都不留空"的最小倍数，
	// 再小就会在边上露出底色 —— 那不是裁剪，是缩小，设备屏上会出现黑边。
	Zoom float64
	// OffsetX / OffsetY ∈ [-1, 1]：在"多余的源图范围"里平移。
	// -1 = 贴左/上，0 = 居中，1 = 贴右/下；该方向没有多余范围时无效果。
	OffsetX float64
	OffsetY float64
}

// CropZoomMin / CropZoomMax 是 Zoom 的合法范围。上限取 4 倍：
// 再往上对 428x142 这种小屏已经没有可辨认的收益，只是把像素放大成色块。
const (
	CropZoomMin = 1.0
	CropZoomMax = 4.0
)

// DefaultCrop 是"填满并居中"，即不带额外交互的默认裁剪。
// 保持这个默认值不变，是为了不改变既有上传路径的结果。
func DefaultCrop() Crop { return Crop{Zoom: 1, OffsetX: 0, OffsetY: 0} }

// Normalize 把参数收进合法范围，并报告是否发生了改动。
// 界面传上来的值一律先过它：裁剪算错不会报错，只会"画出来不对"，属于最难查的那类。
func (c Crop) Normalize() (Crop, bool) {
	out := c
	changed := false
	if !(out.Zoom >= CropZoomMin) { // NaN 也走这里
		out.Zoom = CropZoomMin
		changed = true
	}
	if out.Zoom > CropZoomMax {
		out.Zoom = CropZoomMax
		changed = true
	}
	clampOff := func(v float64) float64 {
		// 先单独判 NaN：写成 `if !(v >= -1) { return 0 }` 会把所有 v<-1 也判成 NaN，
		// 于是 -9 被收敛到 0（居中）而不是 -1（贴边）—— 越界值会静默变成另一个合法值，
		// 属于最难发现的那类错误（不报错、只是效果不对）。
		if v != v {
			return 0
		}
		if v < -1 {
			return -1
		}
		if v > 1 {
			return 1
		}
		return v
	}
	if nx := clampOff(out.OffsetX); nx != out.OffsetX {
		out.OffsetX, changed = nx, true
	}
	if ny := clampOff(out.OffsetY); ny != out.OffsetY {
		out.OffsetY, changed = ny, true
	}
	return out, changed
}

// Encode 把任意图片转成设备要的字节，并给出 CRC。
//
// 缩放规则：放宽到「≥428x142 / 自动缩放」，因此任何尺寸都接受。
func Encode(img image.Image) (payload []byte, crc uint16, err error) {
	return EncodeWith(img, DefaultCrop())
}

// EncodeWith 与 Encode 相同，但允许指定裁剪框（手动裁剪编辑器用）。
func EncodeWith(img image.Image, crop Crop) (payload []byte, crc uint16, err error) {
	rgb, err := CoverToRGBWith(img, Width, Height, crop)
	if err != nil {
		return nil, 0, err
	}
	payload = RGBToRGB565BE(rgb)
	if len(payload) != PayloadSize {
		return nil, 0, fmt.Errorf("encoded payload is %d bytes, want %d", len(payload), PayloadSize)
	}
	return payload, Crc16Xmodem(payload), nil
}

// CoverToRGBWith 等比缩放 + 按 crop 裁剪到 w x h，返回 RGB888 缓冲（行主序）。
//
// 这是唯一的裁剪实现：手动裁剪预览、实际上传、以及既有的自动居中路径都走它。
func CoverToRGBWith(img image.Image, w, h int, crop Crop) ([]byte, error) {
	if img == nil {
		return nil, fmt.Errorf("screen image is nil")
	}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return nil, fmt.Errorf("screen image has empty bounds %v", b)
	}
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("target size %dx%d is invalid", w, h)
	}
	crop, _ = crop.Normalize()

	// cover：取较大的缩放比，保证两个方向都不留空；再乘以用户要的额外放大
	scale := float64(w) / float64(sw)
	if s := float64(h) / float64(sh); s > scale {
		scale = s
	}
	scale *= crop.Zoom

	// 源图上对应的窗口
	cropW := float64(w) / scale
	cropH := float64(h) / scale
	if cropW > float64(sw) {
		cropW = float64(sw)
	}
	if cropH > float64(sh) {
		cropH = float64(sh)
	}
	slackX := float64(sw) - cropW
	slackY := float64(sh) - cropH
	// 居中 + 按 offset 平移（offset=−1 贴左/上，+1 贴右/下）
	srcX := float64(b.Min.X) + slackX/2*(1+crop.OffsetX)
	srcY := float64(b.Min.Y) + slackY/2*(1+crop.OffsetY)
	srcX = math.Min(math.Max(srcX, float64(b.Min.X)), float64(b.Min.X)+slackX)
	srcY = math.Min(math.Max(srcY, float64(b.Min.Y)), float64(b.Min.Y)+slackY)

	out := make([]byte, w*h*3)
	for y := 0; y < h; y++ {
		sy := srcY + (float64(y)+0.5)*cropH/float64(h)
		for x := 0; x < w; x++ {
			sx := srcX + (float64(x)+0.5)*cropW/float64(w)
			r, g, bl := sampleBilinear(img, sx, sy, b)
			i := (y*w + x) * 3
			out[i], out[i+1], out[i+2] = r, g, bl
		}
	}
	return out, nil
}

// PNGFromRGB 把 RGB888 缓冲编成 PNG。
func PNGFromRGB(rgb []byte, w, h int) ([]byte, error) {
	if len(rgb) != w*h*3 {
		return nil, fmt.Errorf("rgb buffer is %d bytes, want %d", len(rgb), w*h*3)
	}
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			o := im.PixOffset(x, y)
			im.Pix[o] = rgb[i]
			im.Pix[o+1] = rgb[i+1]
			im.Pix[o+2] = rgb[i+2]
			im.Pix[o+3] = 0xFF
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, im); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sampleBilinear 在源图上做双线性采样（Alpha 按背景黑做合成，因为设备不存 alpha）。
func sampleBilinear(img image.Image, sx, sy float64, b image.Rectangle) (uint8, uint8, uint8) {
	x0 := int(sx - 0.5)
	y0 := int(sy - 0.5)
	fx := sx - 0.5 - float64(x0)
	fy := sy - 0.5 - float64(y0)

	var acc [3]float64
	var wsum float64
	for dy := 0; dy <= 1; dy++ {
		for dx := 0; dx <= 1; dx++ {
			px, py := x0+dx, y0+dy
			if px < b.Min.X {
				px = b.Min.X
			}
			if py < b.Min.Y {
				py = b.Min.Y
			}
			if px >= b.Max.X {
				px = b.Max.X - 1
			}
			if py >= b.Max.Y {
				py = b.Max.Y - 1
			}
			wx := 1 - fx
			if dx == 1 {
				wx = fx
			}
			wy := 1 - fy
			if dy == 1 {
				wy = fy
			}
			w := wx * wy
			r16, g16, b16, a16 := img.At(px, py).RGBA() // 16-bit, alpha-premultiplied
			// 反预乘再按 alpha 合成到黑底，避免透明像素被算成白色
			if a16 != 0 {
				r16 = r16 * 0xFFFF / a16
				g16 = g16 * 0xFFFF / a16
				b16 = b16 * 0xFFFF / a16
			}
			av := float64(a16) / 65535.0
			acc[0] += float64(r16>>8) * av * w
			acc[1] += float64(g16>>8) * av * w
			acc[2] += float64(b16>>8) * av * w
			wsum += w
		}
	}
	if wsum == 0 {
		return 0, 0, 0
	}
	return clamp8(acc[0] / wsum), clamp8(acc[1] / wsum), clamp8(acc[2] / wsum)
}

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// RGBToRGB565BE 把 RGB888 转成设备用的 RGB565 大端字节流。
func RGBToRGB565BE(rgb []byte) []byte {
	n := len(rgb) / 3
	out := make([]byte, n*2)
	for i := 0; i < n; i++ {
		r, g, b := rgb[i*3], rgb[i*3+1], rgb[i*3+2]
		v := uint16(r>>3)<<11 | uint16(g>>2)<<5 | uint16(b>>3)
		out[i*2] = byte(v >> 8)
		out[i*2+1] = byte(v)
	}
	return out
}

// RGB565BEToRGB 是上面的逆运算，用于回读校验与调试。
func RGB565BEToRGB(payload []byte) []byte {
	n := len(payload) / 2
	out := make([]byte, n*3)
	for i := 0; i < n; i++ {
		v := uint16(payload[i*2])<<8 | uint16(payload[i*2+1])
		r := byte((v >> 11) & 0x1F)
		g := byte((v >> 5) & 0x3F)
		b := byte(v & 0x1F)
		out[i*3] = r<<3 | r>>2
		out[i*3+1] = g<<2 | g>>4
		out[i*3+2] = b<<3 | b>>2
	}
	return out
}

// 内置官方精选素材。

//go:embed presets/*.png
var presetFS embed.FS

// Preset 是一张内置的官方精选。
type Preset struct {
	// Index 从 1 开始，是素材在资源数据段里的出现顺序，
	// 不是官方资源名里的 StaticImg 序号 —— 见 Presets() 的说明。
	Index int
	// Name 是展示名，按界面位置编号：界面上第 1 项叫「预设一」…第 10 项叫「预设十」。
	Name string
	// File 是内嵌资源文件名。
	File string
}

// presetNames 是界面上那十项的展示名，第 1 项 =「预设一」。
var presetNames = []string{
	"预设一", "预设二", "预设三", "预设四", "预设五",
	"预设六", "预设七", "预设八", "预设九", "预设十",
}

// OfficialUILabelOrder 记录官方 UI 那十项与内置素材序号的对应关系。
// 界面上第 i 项 = 素材序号 OfficialUILabelOrder[i-1]（素材序号 = 资源数据段里的出现顺序）。
var OfficialUILabelOrder = []int{6, 3, 1, 9, 7, 5, 2, 10, 8, 4}

// presetNameForIndex 返回某素材在界面上的展示名。
//
// 名字跟着界面位置走，不跟素材序号走：界面按官方顺序排列后，若还拿素材序号命名，
// 网格上会显示成「预设六 / 预设三 / 预设一…」这种乱序（与官方 UI 也对不上）。
func presetNameForIndex(index int) string {
	for position, placed := range OfficialUILabelOrder {
		if placed == index {
			return presetNames[position%len(presetNames)]
		}
	}
	return fmt.Sprintf("预设%d", index)
}

// PresetsInOfficialOrder 按官方 UI 的顺序返回十张内置素材。
func PresetsInOfficialOrder() ([]Preset, error) {
	all, err := Presets()
	if err != nil {
		return nil, err
	}
	byIndex := make(map[int]Preset, len(all))
	for _, p := range all {
		byIndex[p.Index] = p
	}
	out := make([]Preset, 0, len(OfficialUILabelOrder))
	for i, idx := range OfficialUILabelOrder {
		p, ok := byIndex[idx]
		if !ok {
			return nil, fmt.Errorf("preset %d (official position %d) not found", idx, i+1)
		}
		out = append(out, p)
	}
	return out, nil
}

// Presets 返回十张内置的官方精选，按素材在资源数据段里的出现顺序升序。
// 每张的 Name 是它在界面上的名字（见 presetNameForIndex）。
func Presets() ([]Preset, error) {
	entries, err := fs.Glob(presetFS, "presets/*.png")
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	out := make([]Preset, 0, len(entries))
	for i, e := range entries {
		index := i + 1
		out = append(out, Preset{
			Index: index,
			Name:  presetNameForIndex(index),
			File:  e,
		})
	}
	return out, nil
}

// PresetPNG 返回内嵌素材的原始 PNG 字节。
func PresetPNG(file string) ([]byte, error) {
	return presetFS.ReadFile(file)
}

// PresetImage 解码第 index 张（1..10）内置精选，并顺手给出它对应的设备字节与 CRC。
func PresetImage(index int) (image.Image, []byte, uint16, error) {
	all, err := Presets()
	if err != nil {
		return nil, nil, 0, err
	}
	for _, p := range all {
		if p.Index != index {
			continue
		}
		f, err := presetFS.Open(p.File)
		if err != nil {
			return nil, nil, 0, err
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("decode %s: %w", p.File, err)
		}
		payload, crc, err := Encode(img)
		if err != nil {
			return nil, nil, 0, err
		}
		return img, payload, crc, nil
	}
	return nil, nil, 0, fmt.Errorf("preset %d not found", index)
}

// presetCRCOnce / presetCRCByPtr 惰性构建「内置精选 CRC → 官方位次」的映射。
var (
	presetCRCOnce  sync.Once
	presetCRCByPtr map[uint16]int // CRC -> 官方位次（1..10）
	presetCRCErr   error
)

func buildPresetCRCCache() {
	presetCRCByPtr = make(map[uint16]int, len(OfficialUILabelOrder))
	ordered, err := PresetsInOfficialOrder()
	if err != nil {
		presetCRCErr = err
		return
	}
	for i, p := range ordered {
		_, _, crc, err := PresetImage(p.Index)
		if err != nil {
			presetCRCErr = err
			return
		}
		presetCRCByPtr[crc] = i + 1 // 官方位次从 1 开始
	}
}

// PresetByCRC 按设备回读的 CRC 反查这是内置的第几项（官方位次 1..10）。
//
// `0xC5` 只回 11 字节信息（时间戳/大小/CRC），协议没有读图命令，
// 因此只能靠 CRC 与十张内置预设比对来"认图"。
func PresetByCRC(crc uint16) (officialPosition int, preset Preset, ok bool) {
	presetCRCOnce.Do(buildPresetCRCCache)
	if presetCRCErr != nil || len(presetCRCByPtr) == 0 {
		return 0, Preset{}, false
	}
	pos, hit := presetCRCByPtr[crc]
	if !hit {
		return 0, Preset{}, false
	}
	ordered, err := PresetsInOfficialOrder()
	if err != nil || pos < 1 || pos > len(ordered) {
		return 0, Preset{}, false
	}
	return pos, ordered[pos-1], true
}
