# FanControl 2.8.2

## 设备连接优先级
### Device Connection Priority

- **优先连接 WiFi 设备 / WiFi-first Connection**
  WiFi 兼容模式新增“优先连接 WiFi 设备”开关，默认关闭。开启后，主页会先扫描 WiFi，未找到 WiFi 设备后再扫描 BLE/HID；找到 WiFi 设备后会立即进入连接。
  Added a “WiFi-first Connection” switch under WiFi compatibility mode, disabled by default. When enabled, the home page scans WiFi before BLE/HID and connects immediately when a WiFi device is found.

- **多设备连接保护 / Multiple-device Protection**
  发现多个 WiFi 设备时保留手动选择流程，避免自动连接到错误设备。
  When multiple WiFi devices are found, FanControl keeps manual selection to prevent connecting to the wrong device.

## 托盘稳定性
### Tray Stability

- **资源管理器重启恢复 / Explorer Restart Recovery**
  修复重启 Windows Explorer 后托盘图标偶尔消失的问题，通知区域恢复后会自动重新显示 FanControl 托盘图标。
  Fixed tray icons occasionally disappearing after restarting Windows Explorer. The FanControl tray icon now returns after the notification area is rebuilt.

- **启动托盘注册 / Startup Tray Registration**
  改善程序启动和系统通知区域重建期间的托盘注册稳定性。
  Improved tray registration stability while the app starts or the Windows notification area is being rebuilt.

## 兼容性说明
### Compatibility Notes

- **覆盖安装 / In-place Upgrade**
  可直接覆盖安装旧版本；设备配置、风扇曲线、学习数据、历史记录、主题、托盘设置和 IP 设置会继续保留。
  You can install over previous versions. Device settings, fan curves, learning data, history, themes, tray settings, and IP settings are preserved.
