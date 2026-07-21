# FanControl v2.6.0 正式版 / Stable Release

FanControl v2.6.0 带来了更完整的历史趋势分析、更稳定的原生设备连接，以及面向不同设备的功耗感知学习和噪声调节功能。

FanControl v2.6.0 introduces improved history analysis, more reliable native device connectivity, power-aware learning, and device-specific noise tuning.

## 更新亮点 / Highlights

### 1. 历史趋势分析 / History Analysis

- **交互式范围选择**：现在可以直接在历史图表中拖动选择时间范围，集中查看特定时间段的数据。

  **Interactive range selection:** You can now drag directly across history charts to inspect a specific time period.

- **图表同步查看**：温度、风扇转速和功耗图表使用相同的时间范围，并保持左右边界对齐。

  **Synchronized charts:** Temperature, fan speed, and power charts share the same time range and aligned boundaries.

- **单数据统计显示**：图表只显示一项数据时，会使用轻量面积图，并标出最小值、最大值和平均值。

  **Single-series statistics:** Charts with one visible series use a lightweight area view with minimum, maximum, and average markers.

- **自适应曲线平滑**：长时间记录会适当降低锯齿感，短时间记录保留更多原始细节，同时保留真实极值。

  **Adaptive curve smoothing:** Long histories reduce visual noise, while short histories preserve more detail and retain real extrema.

### 2. SmartControl 长期学习 / SmartControl Long-Term Learning

- **功耗感知学习**：SmartControl 会结合多个稳定运行阶段以及 CPU、GPU 功耗变化更新学习结果。

  **Power-aware learning:** SmartControl combines multiple steady-state periods with CPU and GPU power behavior when updating learned adjustments.

- **噪声收益参考**：可信的噪声诊断结果会帮助 SmartControl 判断降低转速是否能够带来明显的噪声收益。

  **Noise-benefit guidance:** Trusted noise diagnostic results help SmartControl determine whether reducing fan speed provides a meaningful noise benefit.

- **设备独立学习**：噪声诊断和学习结果按设备分别保存，不会在不同设备或不同转速单位之间混用。

  **Device-scoped learning:** Noise diagnostics and learning results are stored per device and are not shared across devices or speed units.

- **安全修正优先**：高温状态下的升速修正仍会立即生效，不受噪声学习影响。

  **Thermal safety priority:** Fan-speed increases required by high temperatures remain immediate and are not weakened by noise learning.

### 3. 噪声诊断 / Noise Diagnostics

- **自动转速扫描**：软件会按照设定范围自动调整风扇转速，并记录每个稳定转速点的噪声变化。

  **Automatic speed scanning:** The application automatically adjusts fan speed across the selected range and records noise changes at each stable point.

- **设备范围适配**：飞智设备从 1000 RPM 开始测试，百分比设备从 5% 开始测试，最高转速遵循设备能力。

  **Device-aware ranges:** FlyDigi devices start at 1000 RPM, percentage-based devices start at 5%, and the maximum follows device capabilities.

- **测试前确认**：开始前会显示测试点数量、预计时间、测试区间和必要的环境提醒。

  **Pre-test confirmation:** The application shows the point count, estimated duration, test range, and environment reminders before starting.

- **随时中断测试**：测试过程中可以随时停止，并丢弃本次已经采集的数据。

  **Cancellable testing:** The test can be stopped at any time, with an option to discard all data collected during the session.

- **独立功能定位**：噪声诊断用于评估噪声收益，不会代替轴噪扫描和用户的手动判断。

  **Separate diagnostic purpose:** Noise diagnostics evaluate noise benefits and do not replace axis-noise scanning or manual user ratings.

### 4. 轴噪扫描与避让 / Axis-Noise Scanning and Avoidance

- **手动噪声评级**：用户可以为每个转速点选择无轴噪、轻微轴噪或明显轴噪。

  **Manual noise ratings:** Each speed point can be rated as no axis noise, mild axis noise, or obvious axis noise.

- **可选扫描范围**：开始扫描前可以按照设备允许范围调整起始转速和结束转速。

  **Selectable scan range:** The starting and ending speeds can be adjusted within the device-supported range before scanning.

- **局部细化复扫**：首次发现轴噪后，软件会确认当前点，并对附近转速进行更精细的扫描。

  **Local refinement scans:** After axis noise is first detected, the application confirms the current point and performs a finer scan around nearby speeds.

- **渐进式避让**：自动曲线控制会尽可能减少轴噪区间的停留时间，同时避免转速出现突兀跳变。

  **Gradual avoidance:** Automatic curve control reduces time spent in axis-noise zones while avoiding abrupt fan-speed changes.

- **设备独立保存**：轴噪结果跟随设备保存，不会跟随曲线方案切换或复制到其他设备。

  **Device-scoped profiles:** Axis-noise results are stored with the device and are not switched with curve profiles or copied to other devices.

### 5. 原生设备连接 / Native Device Connectivity

- **原生设备优先**：BLE 和 HID 设备拥有更明确的连接优先级，不会被 WiFi 兼容设备错误覆盖。

  **Native-device priority:** BLE and HID devices receive clearer connection priority and are not incorrectly replaced by WiFi compatibility devices.

- **BS1 重连改进**：改进 BLE 扫描停止、失效连接清理、心跳生命周期和断线重连流程。

  **Improved BS1 reconnection:** BLE scan cancellation, stale-connection cleanup, heartbeat lifecycle, and reconnection handling have been improved.

- **控制失败恢复**：转速写入失败时会重新确认连接状态，并在连接恢复后重新尝试。

  **Control failure recovery:** Failed speed writes now trigger connection validation and can be retried after the connection recovers.

- **设备状态同步**：设备名称、能力、连接方式和实时转速始终来自当前实际连接的设备。

  **Synchronized device status:** Device names, capabilities, transport, and live speed now always come from the active connection.

## 修复与改进 / Fixes and Improvements

### 设备与控制 / Devices and Control

- 修复 BS1 重连后可能错误显示为 WiFi 设备的问题。

  Fixed an issue where a reconnected BS1 could be displayed as a WiFi device.

- 修复未连接设备时打开曲线页可能初始化、归一化或保存设备曲线的问题。

  Fixed curve initialization, normalization, and saving when opening the curve page without a connected device.

- 未连接设备时，曲线页现在只显示历史趋势，不再修改设备相关配置。

  When no device is connected, the curve page now displays history only and does not modify device-specific settings.

- WiFi 和虚拟串口设备只有在对应兼容模式启用后才会参与连接。

  WiFi and virtual serial devices now participate in connection attempts only when their compatibility modes are enabled.

### 图表与监测 / Charts and Monitoring

- 改进 GPU 待机功耗识别，减少失效活动状态造成的异常功耗尖峰。

  Improved GPU idle-power detection to reduce abnormal spikes caused by stale activity states.

- 改进历史图表的范围选择、极值标记、统计参考线和面积填充效果。

  Improved history range selection, extrema markers, statistical reference lines, and area fills.

- 图表提示现在会同时显示当前时间点的全部可见数据。

  Chart tooltips now display all visible data for the selected time point.

### 界面与主题 / Interface and Themes

- 改进窄窗口下的弹窗尺寸、内容滚动和操作区域布局。

  Improved dialog sizing, content scrolling, and action layouts on narrow windows.

- 修复弹窗打开时背景页面可能出现横向闪动阴影的问题。

  Fixed a horizontal flickering shadow that could appear behind open dialogs.

- 图表颜色、统计标记和选择区域统一使用主题变量。

  Chart colors, statistical markers, and selection areas now consistently use theme variables.

- 改进自定义主题字体回退，并补充缺失的中文字符。

  Improved custom-theme font fallback and restored missing Chinese glyphs.

## 兼容性说明 / Compatibility Notes

- 现有配置、设备档案、曲线方案、学习数据、历史记录、主题、托盘设置和 IP 设置会在升级时保留。

  Existing configurations, device profiles, fan curves, learning data, history, themes, tray settings, and IP settings are preserved during upgrades.

- WiFi 和虚拟串口设备仍属于手动启用的兼容模式。

  WiFi and virtual serial devices remain manually enabled compatibility modes.

- BLE 和 HID 设备继续使用原生自动发现和连接机制。

  BLE and HID devices continue to use native automatic discovery and connection handling.

- 噪声诊断会主动调整风扇转速，开始前请确认测试范围、麦克风和周围环境。

  Noise diagnostics actively change fan speed, so verify the test range, microphone, and environment before starting.
