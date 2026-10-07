// Package bsaudio 是黑鲨「音频同步」灯效（命令 `0x15`）的主机侧采集：
// 采系统正在播放的声音（WASAPI loopback），算出主频，交给上层分档（1|2|3）。
package bsaudio

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
)

// ErrUnsupported 表示当前平台不支持系统音频环回采集（非 Windows）。
var ErrUnsupported = errors.New("bsaudio: system audio loopback capture is only supported on Windows")

// WindowSize 是 FFT 窗口点数（固定 1024 ⇒ bin 宽 48000/1024 ≈ 46.875 Hz）。
const WindowSize = 1024

// SilenceRMS 是静音门限，与官方一致：常数 100.0，约 -50 dBFS。
const SilenceRMS = 100.0

// MaxBlockFrames 单块最多用这么多帧（官方：> 2048 截到 2048）。
const MaxBlockFrames = 2048

// MinBlockFrames 单块少于这么多帧就丢弃（官方：< 64 不发）。
const MinBlockFrames = 64

// Sampler 后台采集器。零值不可用，请用 Start 创建；Stop 后可丢弃。
type Sampler struct {
	freqBits atomic.Uint64 // math.Float64bits(主频 Hz)；0 = 静音/无数据
	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
}

// Frequency 返回最近一块的主频（Hz）；`0` = 静音或还没采到数据。
func (s *Sampler) Frequency() float64 {
	if s == nil {
		return 0
	}
	return math.Float64frombits(s.freqBits.Load())
}

// Stop 停止采集并等待采集协程退出（幂等；Start 失败时 s 为 nil，本方法安全）。
func (s *Sampler) Stop() {
	if s == nil || s.stop == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stop) })
	<-s.done
}

// setFreq 供平台实现写入采集结果。
func (s *Sampler) setFreq(freq float64) {
	s.freqBits.Store(math.Float64bits(freq))
}

// FFT 是就地 radix-2 快速傅里叶变换（长度必须是 2 的幂）。
// 自己写而不是引第三方：只需要这一个函数，且长度固定 1024。
func FFT(re, im []float64) {
	n := len(re)
	if n < 2 || n&(n-1) != 0 || len(im) != n {
		return
	}
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j |= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	// 预表只算一次，避免内层反复三角函数。
	cosT, sinT := make([]float64, n/2), make([]float64, n/2)
	for i := range cosT {
		ang := -2 * math.Pi * float64(i) / float64(n)
		cosT[i], sinT[i] = math.Cos(ang), math.Sin(ang)
	}
	for l := 2; l <= n; l <<= 1 {
		step := n / l
		for i := 0; i < n; i += l {
			for k := 0; k < l/2; k++ {
				cr, ci := cosT[k*step], sinT[k*step]
				ur, ui := re[i+k], im[i+k]
				xr, xi := re[i+k+l/2], im[i+k+l/2]
				vr, vi := xr*cr-xi*ci, xr*ci+xi*cr
				re[i+k], im[i+k] = ur+vr, ui+vi
				re[i+k+l/2], im[i+k+l/2] = ur-vr, ui-vi
			}
		}
	}
}

// peakFrequencyInto 计算窗口内幅度最大的那个 bin 对应的频率（Hz）；全静音/全零返回 0。
//
// 算法与官方一致：逐 bin |X| 取最大，再 `主频 = (采样率/N) × peakBin`。
func peakFrequencyInto(samples []float64, sampleRate float64, re, im []float64) float64 {
	n := WindowSize
	if len(re) < n || len(im) < n {
		return 0
	}
	re, im = re[:n], im[:n]
	for i := 0; i < n; i++ {
		if i < len(samples) {
			re[i] = samples[i]
		} else {
			re[i] = 0
		}
		im[i] = 0
	}
	FFT(re, im)
	best, bestBin := 0.0, 0
	for b := 1; b <= n/2; b++ {
		mag := re[b]*re[b] + im[b]*im[b]
		if mag > best {
			best, bestBin = mag, b
		}
	}
	if bestBin == 0 || best == 0 {
		return 0
	}
	return sampleRate / float64(n) * float64(bestBin)
}

// RMS 按官方口径算一块 PCM 的响度：`sqrt(mean(((L+R)/2)²))`（16bit 双声道交织）。
// 每块都算它，用来做静音门限（< 100 就不发帧）。
func RMS(pcm []byte) float64 {
	frames := len(pcm) / 4
	if frames == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < frames; i++ {
		l := float64(int16(uint16(pcm[i*4]) | uint16(pcm[i*4+1])<<8))
		r := float64(int16(uint16(pcm[i*4+2]) | uint16(pcm[i*4+3])<<8))
		m := (l + r) / 2
		sum += m * m
	}
	return math.Sqrt(sum / float64(frames))
}

// foldMono 把 16bit 双声道交织 PCM 折成单声道样本，追加进 dst，返回新的 dst。
// 折法是 `(L+R)/2`，与 RMS 的口径一致。
func foldMono(dst []float64, pcm []byte) []float64 {
	frames := len(pcm) / 4
	for i := 0; i < frames; i++ {
		l := float64(int16(uint16(pcm[i*4]) | uint16(pcm[i*4+1])<<8))
		r := float64(int16(uint16(pcm[i*4+2]) | uint16(pcm[i*4+3])<<8))
		dst = append(dst, (l+r)/2)
	}
	return dst
}
