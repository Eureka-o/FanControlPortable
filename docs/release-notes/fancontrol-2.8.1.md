# FanControl 2.8.1

## 仅监控模式
### Monitor-only Mode

- **仅监控模式 / Monitor-only Mode**
  无需连接风扇设备即可读取 CPU/GPU 温度与功耗，适合设备暂时不在身边时使用。
  Monitor CPU/GPU temperatures and power without connecting a fan device.

- **首页监控布局 / Monitor-only Dashboard**
  首页分别展示 CPU 与 GPU 信息、温度、功耗及近期功耗趋势，不再显示风扇控制相关内容。
  The home page now separates CPU and GPU information, temperature, power, and recent power trends without fan-control sections.

- **后台资源占用 / Background Resource Usage**
  仅监控模式下减少设备搜索、连接和控制相关的后台活动，运行更加轻量。
  Monitor-only mode reduces background device discovery, connection, and control activity for a lighter runtime footprint.

## 历史曲线显示
### History Chart Display

- **完整历史曲线 / Full History Chart**
  仅监控模式首页使用与原曲线页一致的完整历史曲线，支持温度、功耗和时间范围查看。
  Monitor-only mode now uses the same full history chart as the original Curve page, including temperature, power, and time-range views.

- **曲线显示设置 / Chart Display Settings**
  保留历史曲线的显示设置、数据系列选择、时间缩放和单数据统计功能。
  History charts retain display settings, series selection, time-range zooming, and single-series statistics.

- **简化监控曲线 / Simplified Monitor-only Charts**
  仅监控模式下不显示风扇速度轴和设备连接事件，温度与功耗曲线保持独立且对齐。
  Fan speed axes and device connection events are omitted in Monitor-only mode while temperature and power charts remain aligned.

## 托盘显示
### Tray Monitoring

- **温度与功耗信息 / Temperature and Power Metrics**
  仅监控模式下托盘悬浮提示显示 CPU/GPU 温度与功耗，不再显示设备未连接提示。
  The monitor-only tray tooltip now shows CPU/GPU temperatures and power instead of a disconnected-device message.

## 兼容性说明
### Compatibility Notes

- **覆盖安装 / In-place Upgrade**
  可直接覆盖安装旧版本；设备配置、风扇曲线、学习数据、历史记录、主题、托盘设置和 IP 设置会继续保留。
  You can install over previous versions; device settings, fan curves, learning data, history, themes, tray settings, and IP settings are preserved.
