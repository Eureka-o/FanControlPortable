# FanControl v2.6.0 Preview 1

这是 FanControl 2.6.0 的首个预览版本，重点升级了历史趋势分析、SmartControl 长期学习和原生设备连接机制，并改善了 GPU 待机监测与主题显示体验。

This is the first preview of FanControl 2.6.0, featuring major improvements to history analysis, long-term SmartControl learning, native device connectivity, GPU idle monitoring, and theme compatibility.

## 注意 / Notice

- 本预览调整了 BLE/HID 设备仲裁、BS1 重连和 SmartControl 学习机制。不同设备与使用环境仍需进一步验证；遇到连接或控速异常时，请附带诊断包反馈。

  This preview updates BLE/HID arbitration, BS1 reconnection, and SmartControl learning. Hardware behavior may vary; please include a diagnostic package when reporting connection or fan-control issues.

- 从旧版本升级时，已有配置、设备档案、曲线方案、学习数据、历史记录、主题和 IP 设置会继续保留。

  Existing configurations, device profiles, fan curves, learning data, history, themes, and IP settings are preserved during upgrades.

## 更新亮点 / Highlights

- **可交互的历史趋势分析**：可以直接在图表上拖动选择时间范围，双击或点击按钮恢复完整历史；温度、风扇与功耗图表保持同步。

  **Interactive history analysis:** Drag across a chart to inspect a specific time range, then double-click or use the reset control to restore the full history.

- **更完整的显示管理**：支持选择和调整历史数据顺序，并新增总功耗数据。每张图只显示一项数据时，会使用轻量面积图，并标出最大值、最小值和平均值。

  **Expanded display management:** Select and reorder history series, including total power. Single-series charts use a subtle area fill with minimum, maximum, and average markers.

- **自适应曲线平滑**：根据采样数量自动调整平滑程度，短时间记录保留原始细节，长时间记录减少锯齿，同时保留真实极值。

  **Adaptive chart smoothing:** Smoothing now scales with sample count, preserving detail in short sessions while reducing visual noise in longer histories without losing real extrema.

- **功耗感知的长期稳态学习**：SmartControl 会结合多段稳定运行数据与 CPU/GPU 功耗环境更新学习偏移，减少短时波动造成的误学习；高温安全修正仍会立即生效。

  **Power-aware long-term learning:** SmartControl combines multiple steady-state periods with CPU/GPU power context, reducing overreaction to short fluctuations while keeping immediate thermal safety corrections.

- **更可靠的 BS1 蓝牙连接**：BLE 设备在原生设备仲裁中优先连接；优化扫描停止、失效连接清理和心跳生命周期，转速写入失败时会重新建立连接并重试。

  **More reliable BS1 connectivity:** BLE devices receive native connection priority, with improved scan cleanup, heartbeat lifecycle, stale-connection handling, and automatic retry after failed speed writes.

## 修复与改进 / Fixes and Improvements

- 修复 BS1 断联后重连时可能显示为 WiFi 设备的问题，设备名称、能力和实时状态现在始终来自当前实际连接。

  Fixed an issue where a reconnected BS1 could be displayed as a WiFi device. Device names, capabilities, and live status now always come from the active connection.

- WiFi 与虚拟串口设备仅在对应兼容模式开启时参与连接，同时继续保留设备库中的设备信息和配置。

  WiFi and virtual serial devices now participate in connection attempts only when their compatibility modes are enabled, while existing device-library entries and configurations remain available.

- 监听蓝牙和 HID 设备重新出现事件，缩短设备重新通电或系统唤醒后的连接恢复时间。

  Added Bluetooth and HID arrival monitoring to restore connections more quickly after a device is powered on again or the system resumes.

- 未连接设备时，曲线页仅显示历史趋势，不再初始化、归一化或保存设备曲线，避免离线查看历史时覆盖已有方案。

  When no device is connected, the curve page displays history only and no longer initializes, normalizes, or saves device curves, preventing offline history viewing from modifying existing profiles.

- 改进独立显卡待机识别，减少显存驻留导致的活跃状态误判，使 GPU 待机功耗历史更加稳定。

  Improved discrete GPU idle detection to reduce false activity caused by resident video memory, resulting in more stable GPU idle-power history.

- 历史范围选择、统计参考线、极值标记和面积图统一使用主题变量，在不同内置主题和自定义主题下保持一致的可读性。

  History selections, statistical reference lines, extrema markers, and area fills now use theme variables for consistent readability across built-in and custom themes.

- 断开连接后会立即清理旧设备的运行时状态，避免失效的转速、能力和控制信息继续出现在界面或参与自动控制。

  Runtime state from a disconnected device is now cleared immediately, preventing stale speed, capability, and control data from remaining visible or affecting automatic control.

- 改进 BLE 扫描的停止和超时处理，减少复杂蓝牙环境下的重复扫描、残留任务和异常资源占用。

  Improved BLE scan cancellation and timeout handling to reduce repeated scans, lingering tasks, and abnormal resource usage in complex Bluetooth environments.
