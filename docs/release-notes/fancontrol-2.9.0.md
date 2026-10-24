# FanControl 2.9.0

## 黑鲨风神 Pro 原生支持
### Black Shark Fengshen Pro Native Support

- **双通道连接 / Dual Transport Support**
  黑鲨风神 Pro 现在可通过 BLE 或 USB 有线模式原生连接，设备会按实际通路加载对应控制能力。
  Black Shark Fengshen Pro now supports native BLE and USB wired connections, with capabilities loaded from the active transport.

- **设备专属曲线 / Device-specific Curves**
  曲线页加入黑鲨专属 RPM 曲线、四档散热方案、温度点拖动和手动档位控制，最高转速按设备标定为 4000 RPM。
  The Curve page now provides Black Shark RPM curves, four cooling profiles, draggable temperature points, and manual gear control with a calibrated 4000 RPM ceiling.

- **变频控制 / Variable-speed Control**
  支持设备侧跟随和主机侧调速两种模式，可在黑鲨设置中选择，曲线与设备状态保持同步。
  Device-side following and host-side variable-speed control are available from Black Shark settings, keeping curves and runtime state synchronized.

## 灯效与屏幕
### Lighting & Screen

- **灯效控制 / Lighting Control**
  黑鲨专属面板支持灯效模式、颜色、亮度和速度等设备参数，并按设备协议分别处理。
  The Black Shark panel supports device lighting modes, colors, brightness, and speed using the device-specific protocol.

- **屏幕参数 / Screen Parameters**
  支持选择屏幕显示的 CPU、GPU 和时间参数，并可立即应用刷新设置。
  Select CPU, GPU, and time parameters for the device screen and apply refresh settings immediately.

- **USB 图传 / USB Image Transfer**
  USB 有线模式支持图片预览、裁剪、缩放、上传进度、中断、历史图重传和设备回读；BLE 模式保留控制能力，但不开放图传。
  USB wired mode supports image preview, crop, zoom, progress, cancel, history re-upload, and device readback. BLE keeps device control but does not expose image transfer.

## 连接与后台稳定性
### Connectivity & Background Stability

- **连接状态同步 / Connection State Synchronization**
  优化 BLE/USB 的连接、重连、挂起恢复和设备快照同步，减少已连接但无法控制的中间状态。
  Improved BLE/USB connection, reconnect, resume, and runtime snapshot synchronization to reduce unusable intermediate states.

- **托盘恢复 / Tray Recovery**
  改善 Windows Explorer 重启、电源状态切换和启动竞态下的托盘图标恢复。
  Improved tray icon recovery after Windows Explorer restarts, power-state changes, and startup races.

## 兼容性说明
### Compatibility Notes

- **黑鲨连接方式 / Black Shark Transports**
  黑鲨风神 Pro 正式支持 BLE 与 USB 有线模式；WiFi 调试设备不再作为内置设备注册。
  Black Shark Fengshen Pro officially supports BLE and USB wired mode; the WiFi debug device is no longer registered as a built-in device.

- **覆盖安装 / In-place Upgrade**
  可直接覆盖安装旧版本；设备配置、风扇曲线、学习数据、历史记录、主题、托盘设置和 IP 设置会继续保留。
  You can install over previous versions; device settings, fan curves, learning data, history, themes, tray settings, and IP settings are preserved.
