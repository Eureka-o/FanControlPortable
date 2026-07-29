# FanControl 2.6.1

## 功耗显示欺骗

### Power Display Spoofing

- 新增功耗显示欺骗功能，默认关闭，可分别调整 CPU 和 GPU 的显示倍率与功耗偏移。
  Added optional power display spoofing with independent CPU and GPU multipliers and offsets.

- 设置弹窗提供实时预览、实际计算公式，以及 CPU/GPU 独立重置功能。
  The settings dialog includes a live preview, substituted formula, and separate reset controls for CPU and GPU.

- 欺骗结果覆盖当前功耗、传感器列表、历史曲线和托盘右键菜单。
  Spoofed values are shown in current readings, sensor lists, history charts, and the system tray menu.

- 开启后，依赖功耗数据的预测与学习功能会静默暂停；真实采样、历史记录和诊断数据不会被修改。
  While enabled, power-dependent prediction and learning are paused silently. Raw telemetry, history, and diagnostics remain unchanged.

- 功能关闭时直接使用原始数据，不创建额外的历史副本或采样缓存。
  When disabled, original data is used directly without additional history copies or sampling caches.

## GPU 读取模式

### GPU Polling Modes

- GPU 读取模式新增“不读取”，可完全停止 GPU 温度和功耗采样，减少不必要的独显唤醒。
  Added a "Do not read" mode that disables GPU temperature and power polling to avoid unnecessary dGPU wake-ups.

- 如果当前控温来源为 GPU，切换至“不读取”后会自动回退到 CPU。
  If GPU is the active thermal source, selecting "Do not read" automatically falls back to CPU control.

## Hatsune Miku / Digital Stage

- 新增 Miku 高级主题，采用一体化背景、圆角侧栏、主题插画和猫啃小来圆润字体。
  Added the Miku advanced theme with a unified background, rounded sidebar, themed artwork, and the rounded Xiaolai typeface.
