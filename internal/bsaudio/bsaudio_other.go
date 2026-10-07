//go:build !windows

package bsaudio

// Start 在非 Windows 平台直接报告不支持 —— 「音频同步」灯效需要系统音频环回采集，
// 只有 Windows（WASAPI）提供。上层拿到 ErrUnsupported 应当如实禁用该功能，
// 不要静默降级成"假装在推"。
func Start() (*Sampler, error) {
	return nil, ErrUnsupported
}
