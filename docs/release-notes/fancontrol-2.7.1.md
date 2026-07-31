# FanControl 2.7.1

## 历史趋势与事件记录

### History Trends and Timeline Events

- 历史图展示设置新增事件显示总开关，并可分别控制设备断开、重新连接、系统恢复和曲线方案切换事件。
  Added a master event visibility switch with independent controls for device disconnects, reconnections, system resume, and curve profile changes.

- 优化历史曲线绘制，保留轻量视觉平滑，同时确保曲线经过原始极值和跳变数据。
  Refined history rendering with light visual smoothing while preserving original extrema and abrupt changes.

## 曲线方案导入与导出

### Curve Profile Import and Export

- 导出曲线方案时可自主勾选一个或多个方案，默认仅选择当前正在使用的方案。
  Curve profile export now supports selecting one or multiple profiles, defaulting to the active profile.

- 复制方案码和导出文件只包含已选择的方案，不再自动包含全部方案。
  Clipboard and file exports now contain only the selected profiles.

## 性能与诊断

### Performance and Diagnostics

- 减少温度历史和智能控温学习数据的磁盘写入频率，并在软件正常退出时安全保存待写入数据。
  Reduced disk writes from temperature history and Smart Control learning data while safely flushing pending data on exit.

- 优化 WiFi 设备健康检查，复用仍然有效的最新遥测数据，减少重复读取。
  Optimized WiFi health checks by reusing fresh telemetry and avoiding redundant device reads.

- 增强诊断包，加入连接阶段记录、运行时设备信息和智能控温决策摘要。
  Enhanced diagnostic packages with connection-stage records, runtime device information, and Smart Control decision summaries.

## 界面与主题兼容性

### Interface and Theme Compatibility

- 移除关闭毛玻璃效果后工作区残留的区域分割线。
  Removed the remaining workspace divider when window blur is disabled.

- 优化赛博朋克 2077 主题在紧凑 Dock 下的悬停效果和装饰布局。
  Improved compact Dock hover feedback and decoration placement in the Cyberpunk 2077 theme.

## 兼容性说明

### Compatibility Notes

- 可直接覆盖安装旧版本；设备配置、风扇曲线、学习数据、历史记录、主题、托盘设置和 IP 设置将继续保留。
  You can install over previous versions. Device settings, fan curves, learning data, history, themes, tray settings, and IP settings are preserved.
