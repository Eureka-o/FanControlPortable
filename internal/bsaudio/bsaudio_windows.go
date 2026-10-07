//go:build windows

package bsaudio

import (
	"runtime"
	"time"
	"unsafe"

	"github.com/diegosz/go-wca/pkg/wca"
	"github.com/go-ole/go-ole"
)

// Start 打开默认渲染端点的 WASAPI loopback 采集，并启动后台协程持续算出主频。
func Start() (*Sampler, error) {
	s := &Sampler{stop: make(chan struct{}), done: make(chan struct{})}
	ready := make(chan error, 1)
	go s.capture(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return s, nil
}

// capture 是采集主循环（运行在一条被 LockOSThread 钉住的线程上）。
func (s *Sampler) capture(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(s.done)

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		ready <- err
		return
	}
	defer ole.CoUninitialize()

	var de *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &de); err != nil {
		ready <- err
		return
	}
	defer de.Release()

	var mmd *wca.IMMDevice
	if err := de.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &mmd); err != nil {
		ready <- err
		return
	}
	defer mmd.Release()

	var ac *wca.IAudioClient
	if err := mmd.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, nil, &ac); err != nil {
		ready <- err
		return
	}
	defer ac.Release()

	var mix *wca.WAVEFORMATEX
	if err := ac.GetMixFormat(&mix); err != nil {
		ready <- err
		return
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(mix)))

	sampleRate := float64(mix.NSamplesPerSec)
	if sampleRate <= 0 {
		sampleRate = 48000
	}

	wfx := &wca.WAVEFORMATEX{
		WFormatTag:      1, // WAVE_FORMAT_PCM
		NChannels:       2,
		NSamplesPerSec:  mix.NSamplesPerSec,
		WBitsPerSample:  16,
		NBlockAlign:     4, // 2ch × 16bit
		NAvgBytesPerSec: mix.NSamplesPerSec * 4,
		CbSize:          0,
	}
	streamFlags := uint32(wca.AUDCLNT_STREAMFLAGS_LOOPBACK |
		wca.AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM |
		wca.AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY)

	if err := ac.Initialize(wca.AUDCLNT_SHAREMODE_SHARED, streamFlags, wca.REFERENCE_TIME(400*10000), 0, wfx, nil); err != nil {
		ready <- err
		return
	}

	var acc *wca.IAudioCaptureClient
	if err := ac.GetService(wca.IID_IAudioCaptureClient, &acc); err != nil {
		ready <- err
		return
	}
	defer acc.Release()

	if err := ac.Start(); err != nil {
		ready <- err
		return
	}
	defer ac.Stop()

	// 到这里才算真的采起来了，可以放行调用方。
	ready <- nil

	// 复用的暂存区：采集循环跑得比"推送循环"快得多，避免每块都分配。
	mono := make([]float64, 0, WindowSize*4)
	re := make([]float64, WindowSize)
	im := make([]float64, WindowSize)

	for {
		select {
		case <-s.stop:
			return
		default:
		}

		var data *byte
		var frames, pflags uint32
		var devPos, qpcPos uint64
		if err := acc.GetBuffer(&data, &frames, &pflags, &devPos, &qpcPos); err != nil {
			time.Sleep(2 * time.Millisecond)
			continue
		}
		if frames == 0 {
			time.Sleep(2 * time.Millisecond)
			continue
		}

		// ReleaseBuffer 必须用引擎给的原始 frames —— 用截断后的值会让
		// 引擎以为还有数据没取走，进而丢帧/错位。
		use := int(frames)
		if use > MaxBlockFrames {
			use = MaxBlockFrames
		}

		if pflags&wca.AUDCLNT_BUFFERFLAGS_SILENT != 0 || use < MinBlockFrames {
			// 静音块（或太短的块）：不出新结果，但必须把数据取走。
			_ = acc.ReleaseBuffer(frames)
			time.Sleep(2 * time.Millisecond)
			continue
		}

		pcm := unsafe.Slice(data, use*4) // NBlockAlign = 4
		rms := RMS(pcm)
		if rms < SilenceRMS {
			mono = mono[:0]
			s.setFreq(0) // 0 = 静音 ⇒ 上层不发帧
		} else {
			mono = foldMono(mono, pcm)
			for len(mono) >= WindowSize {
				// 用最近的 WindowSize 个样本做一次 FFT（跨块累积 ⇒ bin 宽恒定）。
				win := mono[len(mono)-WindowSize:]
				s.setFreq(peakFrequencyInto(win, sampleRate, re, im))
				mono = mono[:0]
			}
		}

		_ = acc.ReleaseBuffer(frames)
		time.Sleep(2 * time.Millisecond)
	}
}
