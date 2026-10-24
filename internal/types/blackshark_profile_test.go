package types

import (
	"testing"
	"time"
)

// BLE 可查看设备状态，但屏幕图片写入仅通过 WinUSB/libusb 有线通路开放。
func TestBlackSharkScreenImageTransferIsUSBOnly(t *testing.T) {
	ble := BlackSharkBRB02Profile()
	if ble.Transport != DeviceTransportBLE || ble.Capabilities.SupportsScreenImageTransfer {
		t.Fatal("BLE Black Shark profile must not advertise screen image transfer")
	}

	usb := BlackSharkBRB02USBProfile()
	if usb.Transport != DeviceTransportUSB || !usb.Capabilities.SupportsScreenImageTransfer {
		t.Fatal("WinUSB Black Shark profile must advertise USB image transfer")
	}
}

func TestBlackSharkBLEProfileDisablesSendIntervalGate(t *testing.T) {
	ble := BlackSharkBRB02Profile()
	if got := ble.Connection.MinSendIntervalMs; got >= 0 {
		t.Fatalf("BLE 档案的 MinSendIntervalMs = %d, want < 0（显式取消 100ms 门）—— "+
			"改成 0 会被 durationFromMillis 静默回落成 100ms，真机 A/B 已证明那是 6~7 倍的代价", got)
	}
	// USB 通路不读 BLE 节流字段，保持未设即可；这里只是防止有人"顺手"也给它写一个值。
	usb := BlackSharkBRB02USBProfile()
	if usb.Connection.MinSendIntervalMs != 0 {
		t.Fatalf("USB 档案不该声明 BLE 节流字段, got %d", usb.Connection.MinSendIntervalMs)
	}
}

// TestBlackSharkImageTransferBudgetPerTransport 图传预算必须按通路取，且必须覆盖量到的成本。
func TestBlackSharkImageTransferBudgetPerTransport(t *testing.T) {
	if got := BlackSharkImageTransferBudget(DeviceTransportUSB); got != BlackSharkImageTransferTimeoutUSB {
		t.Fatalf("USB 预算 = %v, want %v（上游原值，实测整轮约 18s，不动）", got, BlackSharkImageTransferTimeoutUSB)
	}

	frames := BlackSharkImageFrameCount()
	if frames != 2098 {
		t.Fatalf("总帧数 = %d, want 2098（2096 数据块 + C4 + C5）", frames)
	}

	ble := BlackSharkImageTransferBudget(DeviceTransportBLE)
	// 硬下界：真机量到（bleprobe 受控节拍）10.6 block/s ⇒ 94ms/块
	// ⇒ 整轮 ≈198s。预算按 100ms/帧 + slack 计，必须覆盖 94ms/帧 那条线。
	measured := time.Duration(frames) * 94 * time.Millisecond
	if ble < measured {
		t.Fatalf("BLE 预算 %v 覆盖不了实测整轮 %v", ble, measured)
	}
	if ble <= BlackSharkImageTransferTimeoutUSB {
		t.Fatalf("BLE 预算 %v 不该小于等于 USB 的 %v", ble, BlackSharkImageTransferTimeoutUSB)
	}

	// 未知通路回落 USB 那个保守值：落到长预算会让真正卡住的 USB 上传挂满 8 分钟才报错。
	for _, transport := range []string{"", "hid", "unknown"} {
		if got := BlackSharkImageTransferBudget(transport); got != BlackSharkImageTransferTimeoutUSB {
			t.Fatalf("通路 %q 的预算 = %v, want %v", transport, got, BlackSharkImageTransferTimeoutUSB)
		}
	}
}
