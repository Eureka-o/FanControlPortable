# FanControl 2.8.0

## 传感器与设备兼容性
### Sensor & Device Compatibility

- **AMD 8945HX 兼容性 / AMD 8945HX Compatibility**
  改进部分 AMD 8945HX 机型的温度与功耗读取，并增加兼容性回退路径。
  Improved temperature and power readings on selected AMD 8945HX systems with additional fallback support.

- **原生设备连接 / Native Device Connectivity**
  优化原生 BLE/HID 设备的发现、连接与异常恢复。
  Improved native BLE/HID discovery, connection stability, and recovery.

## PawnIO 驱动维护
### PawnIO Driver Maintenance

- **驱动重新安装 / Driver Reinstallation**
  设置中支持重新安装 PawnIO，适用于传感器读取异常等情况。
  Added an in-app PawnIO reinstall workflow for sensor readout issues.

## 自启动与托盘稳定性
### Startup & Tray Stability

- **自启动可靠性 / Startup Reliability**
  自启动任务支持电池供电启动，不再受 72 小时运行限制，并支持异常退出后自动重启。
  Startup now works on battery power, has no 72-hour limit, and can restart after an unexpected exit.

- **托盘恢复 / Tray Recovery**
  Explorer 重启、睡眠唤醒或托盘异常后，可重复恢复托盘图标。
  Tray icons can now recover repeatedly after Explorer restarts, resume, or tray failures.

## 主题视觉升级
### Theme Visual Upgrade

- **FanControl Classic / FanControl Classic**
  原 THRM 主题完成高级主题升级，并正式更名为 FanControl Classic，采用一体化幕布、毛玻璃 Dock 与卡片以及轻量科技纹理。
  The former THRM theme has been upgraded to an advanced theme and officially renamed FanControl Classic, with a unified canvas, glass Dock and cards, and lightweight technology-inspired textures.

- **高级主题适配 / Advanced Theme Compatibility**
  优化高级主题在展开 Dock、玻璃组件和幕布背景下的显示，并在升级安装时同步新的内置主题资源。
  Improved advanced-theme presentation with the expanded Dock, glass surfaces, and curtain backgrounds, while refreshing bundled theme assets during installation upgrades.

## 兼容性说明
### Compatibility Notes

- **覆盖安装 / In-place Upgrade**
  可直接覆盖安装旧版本；设备配置、风扇曲线、学习数据、历史记录、主题、托盘设置和 IP 设置会继续保留。
  You can install over previous versions; device settings, fan curves, learning data, history, themes, tray settings, and IP settings are preserved.
